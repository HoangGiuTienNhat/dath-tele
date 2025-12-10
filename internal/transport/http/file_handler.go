package http

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"file-sharing/internal/model"

	"github.com/gin-gonic/gin"
)

// Khai báo các Interface cần dùng (để tránh dependency vòng)
// Lấy từ storage/file.go
type FileRepository interface {
	CreateFile(ctx context.Context, file model.File) (*model.File, error)
}

// Lấy từ storage/minio_repo.go
type MinioService interface {
	CreatePresignedPutURL(ctx context.Context, objectKey string, contentType string) (string, error)
}

type FileHandler struct {
	fileRepo FileRepository
	minioSvc MinioService
}

func NewFileHandler(fileRepo FileRepository, minioSvc MinioService) *FileHandler {
	return &FileHandler{
		fileRepo: fileRepo,
		minioSvc: minioSvc,
	}
}

type FileInitRequest struct {
	Filename  string    `json:"filename" binding:"required"`
	Size      int64     `json:"size" binding:"required"`
	MimeType  string    `json:"mime_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FileInitResponse struct {
	FileID    int64     `json:"file_id"`
	ObjectKey string    `json:"object_key"`
	UploadURL string    `json:"upload_url,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// generateRandomToken generates a random 16-byte hex string
func generateRandomToken() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()) // fallback
	}
	return hex.EncodeToString(b)
}

// POST /v1/files
// HandleFileInit là handler cho POST /v1/files: Khởi tạo upload
func (h *FileHandler) HandleFileInit(c *gin.Context) {
	// 1. Xác thực và lấy User từ Context
	user, exists := GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 2. Bind JSON request
	var req FileInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload", "details": err.Error()})
		return
	}

	// 3. Chuẩn bị metadata file
	objectKey := generateRandomToken() // Tạo một khóa đối tượng ngẫu nhiên

	newFile := model.File{
		OwnerUserID: user.ID,
		Filename:    req.Filename,
		ObjectKey:   objectKey,
		Size:        req.Size,
		Mime:        req.MimeType,
		Status:      model.FileStatusPending, // Đặt trạng thái ban đầu là 'pending'
		// Không cần truyền CreatedAt/UpdatedAt vì DB sẽ tự động tạo/update
	}

	// 4. Lưu metadata vào DB (repository)
	createdFile, err := h.fileRepo.CreateFile(c.Request.Context(), newFile)
	if err != nil {
		log.Printf("DB Error: Failed to create file record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize file record"})
		return
	}

	// 5. Tạo Presigned URL (dùng MinIO Service)
	// URL sẽ chỉ cho phép client thực hiện 1 PUT duy nhất
	if h.minioSvc == nil {
		log.Printf("MinIO Error: MinIO service not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "File upload service not available - MinIO not configured",
		})
		return
	}

	uploadURL, err := h.minioSvc.CreatePresignedPutURL(c.Request.Context(), objectKey, req.MimeType)
	if err != nil {
		log.Printf("MinIO Error: Failed to generate upload URL: %v", err)
		// Nếu MinIO lỗi, cân nhắc xóa bản ghi DB để tránh file rác (tùy chọn)
		// Hiện tại ta chỉ trả về lỗi
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate upload URL"})
		return
	}

	// 6. Trả về response thành công
	resp := FileInitResponse{
		FileID:    createdFile.ID,
		ObjectKey: createdFile.ObjectKey,
		UploadURL: uploadURL, // URL để client upload trực tiếp
		Status:    createdFile.Status,
		CreatedAt: createdFile.CreatedAt,
		UpdatedAt: createdFile.UpdatedAt,
	}

	c.JSON(http.StatusOK, resp)
}

type FileResponse struct {
	ID        int64     `json:"id"`
	ObjectKey string    `json:"object_key"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Mime      string    `json:"mime"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GET /v1/files - Liệt kê file của người dùng hiện tại
func ListUserFilesHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		telegramID := c.GetHeader("X-Telegram-User-Id")
		username := c.GetHeader("X-Telegram-Username")

		if telegramID == "" || username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Telegram headers"})
			return
		}

		query := `
			SELECT
				f.id,
				f.object_key,
				f.filename,
				f.size,
				f.mime,
				f.status,
				f.created_at,
				f.updated_at
			FROM files f
			JOIN users u ON u.id = f.owner_user_id
			WHERE u.telegram_user_id = $1
			ORDER BY f.created_at DESC
		`

		rows, err := db.Query(query, telegramID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer rows.Close()

		files := []FileResponse{}
		for rows.Next() {
			var file FileResponse
			err := rows.Scan(
				&file.ID,
				&file.ObjectKey,
				&file.Filename,
				&file.Size,
				&file.Mime,
				&file.Status,
				&file.CreatedAt,
				&file.UpdatedAt,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan file"})
				return
			}
			files = append(files, file)
		}

		if err = rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, files)
	}
}

// ReportUploadCompleteRequest represents the request body for reporting upload completion
type ReportUploadCompleteRequest struct {
	Status           string  `json:"status" binding:"required,oneof=completed failed"`
	ReportType       string  `json:"report_type" binding:"required,oneof=success error warning"`
	Message          string  `json:"message"`
	ErrorCode        string  `json:"error_code"`
	ErrorMessage     string  `json:"error_message"`
	FileChecksum     string  `json:"file_checksum"`
	FileSizeActual   int64   `json:"file_size_actual"`
	UploadDurationMs int     `json:"upload_duration_ms"`
	BandwidthKbps    float64 `json:"bandwidth_kbps"`
}

// ReportUploadCompleteResponse represents the response for upload completion report
type ReportUploadCompleteResponse struct {
	ReportID   int64     `json:"report_id"`
	FileID     int64     `json:"file_id"`
	Status     string    `json:"status"`
	ReportType string    `json:"report_type"`
	Message    string    `json:"message,omitempty"`
	ReportedAt time.Time `json:"reported_at"`
}

// POST /v1/files/{file_id}/report-complete
// ReportUploadCompleteHandler handles the upload completion report
func ReportUploadCompleteHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		telegramID := c.GetHeader("X-Telegram-User-Id")
		username := c.GetHeader("X-Telegram-Username")

		if telegramID == "" || username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Telegram headers"})
			return
		}

		fileIDStr := c.Param("file_id")
		fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
			return
		}

		var req ReportUploadCompleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Step 1: Find or create user
		var userID int64
		err = db.QueryRow(`SELECT id FROM users WHERE telegram_user_id = $1`, telegramID).Scan(&userID)
		if err == sql.ErrNoRows {
			err = db.QueryRow(`
				INSERT INTO users (telegram_user_id, username)
				VALUES ($1, $2)
				RETURNING id
			`, telegramID, username).Scan(&userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
				return
			}
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Step 2: Verify file belongs to user
		var existingFileID int64
		err = db.QueryRow(`
			SELECT id FROM files 
			WHERE id = $1 AND owner_user_id = $2
		`, fileID, userID).Scan(&existingFileID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found or access denied"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Step 3: Create upload report
		var reportID int64
		now := time.Now()
		err = db.QueryRow(`
			INSERT INTO upload_reports (
				file_id,
				owner_user_id,
				status,
				report_type,
				message,
				error_code,
				error_message,
				file_checksum,
				file_size_actual,
				upload_duration_ms,
				bandwidth_kbps,
				reported_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id
		`, fileID, userID, req.Status, req.ReportType, req.Message, req.ErrorCode,
			req.ErrorMessage, req.FileChecksum, req.FileSizeActual, req.UploadDurationMs,
			req.BandwidthKbps, now, now).Scan(&reportID)
		if err != nil {
			log.Printf("Failed to create upload report: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create report"})
			return
		}

		// Step 4: Update file status based on report status
		newFileStatus := "completed"
		if req.Status == "failed" {
			newFileStatus = "failed"
		}

		_, err = db.Exec(`
			UPDATE files
			SET status = $1, updated_at = NOW()
			WHERE id = $2
		`, newFileStatus, fileID)
		if err != nil {
			log.Printf("Failed to update file status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update file status"})
			return
		}

		resp := ReportUploadCompleteResponse{
			ReportID:   reportID,
			FileID:     fileID,
			Status:     req.Status,
			ReportType: req.ReportType,
			Message:    req.Message,
			ReportedAt: now,
		}
		c.JSON(http.StatusCreated, resp)
	}
}

// GET /v1/files/{file_id}/report
// GetUploadReportHandler retrieves the latest upload report for a file
func GetUploadReportHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		telegramID := c.GetHeader("X-Telegram-User-Id")
		username := c.GetHeader("X-Telegram-Username")

		if telegramID == "" || username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Telegram headers"})
			return
		}

		fileIDStr := c.Param("file_id")
		fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file_id"})
			return
		}

		// Step 1: Find user
		var userID int64
		err = db.QueryRow(`SELECT id FROM users WHERE telegram_user_id = $1`, telegramID).Scan(&userID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Step 2: Verify file belongs to user and get report
		var report model.UploadReport
		err = db.QueryRow(`
			SELECT
				ur.id,
				ur.file_id,
				ur.owner_user_id,
				ur.status,
				ur.report_type,
				ur.message,
				ur.error_code,
				ur.error_message,
				ur.file_checksum,
				ur.file_size_actual,
				ur.upload_duration_ms,
				ur.bandwidth_kbps,
				ur.reported_at,
				ur.updated_at
			FROM upload_reports ur
			JOIN files f ON f.id = ur.file_id
			WHERE ur.file_id = $1 AND f.owner_user_id = $2
			ORDER BY ur.reported_at DESC
			LIMIT 1
		`, fileID, userID).Scan(
			&report.ID,
			&report.FileID,
			&report.OwnerUserID,
			&report.Status,
			&report.ReportType,
			&report.Message,
			&report.ErrorCode,
			&report.ErrorMessage,
			&report.FileChecksum,
			&report.FileSizeActual,
			&report.UploadDurationMs,
			&report.BandwidthKbps,
			&report.ReportedAt,
			&report.UpdatedAt,
		)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "upload report not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, report)
	}
}

// GET /v1/upload-reports
// ListUploadReportsHandler lists all upload reports for the current user
func ListUploadReportsHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		telegramID := c.GetHeader("X-Telegram-User-Id")
		username := c.GetHeader("X-Telegram-Username")

		if telegramID == "" || username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Telegram headers"})
			return
		}

		// Parse pagination parameters
		limit := 20
		offset := 0
		if l := c.Query("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		if o := c.Query("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		// Step 1: Find user
		var userID int64
		err := db.QueryRow(`SELECT id FROM users WHERE telegram_user_id = $1`, telegramID).Scan(&userID)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		// Step 2: Get upload reports
		query := `
			SELECT
				id,
				file_id,
				owner_user_id,
				status,
				report_type,
				message,
				error_code,
				error_message,
				file_checksum,
				file_size_actual,
				upload_duration_ms,
				bandwidth_kbps,
				reported_at,
				updated_at
			FROM upload_reports
			WHERE owner_user_id = $1
			ORDER BY reported_at DESC
			LIMIT $2 OFFSET $3
		`

		rows, err := db.Query(query, userID, limit, offset)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
		defer rows.Close()

		reports := []model.UploadReport{}
		for rows.Next() {
			var report model.UploadReport
			err := rows.Scan(
				&report.ID,
				&report.FileID,
				&report.OwnerUserID,
				&report.Status,
				&report.ReportType,
				&report.Message,
				&report.ErrorCode,
				&report.ErrorMessage,
				&report.FileChecksum,
				&report.FileSizeActual,
				&report.UploadDurationMs,
				&report.BandwidthKbps,
				&report.ReportedAt,
				&report.UpdatedAt,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan report"})
				return
			}
			reports = append(reports, report)
		}

		if err = rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}

		c.JSON(http.StatusOK, reports)
	}
}
