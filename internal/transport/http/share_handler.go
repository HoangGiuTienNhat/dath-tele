package http

import (
	"errors"
	"file-sharing/internal/model"
	"file-sharing/internal/share"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ShareHandler struct {
	service share.Service
}

func NewShareHandler(service share.Service) *ShareHandler {
	return &ShareHandler{
		service: service,
	}
}

// POST /v1/shares/:id/revoke
func (h *ShareHandler) HandleRevoke(c *gin.Context) {
	// Lấy User hiện tại từ Context (do AuthMiddleware nạp vào)
	user, exists := GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Lấy Share ID từ URL
	shareIDStr := c.Param("id")
	shareID, err := strconv.ParseInt(shareIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid share ID"})
		return
	}

	// Gọi Service
	err = h.service.RevokeShare(c.Request.Context(), shareID, user.ID)
	if err != nil {
		// Nếu lỗi là do không tìm thấy hoặc không đúng chủ sở hữu
		// if err.Error() == "share not found or access denied" {
		if errors.Is(err, share.ErrShareNotFoundOrAccessDenied) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	// Trả về thành công
	c.JSON(http.StatusOK, gin.H{
		"message":  "Share revoked successfully",
		"share_id": shareID,
		"status":   "revoked",
	})
}

// GET /v1/shares
func (h *ShareHandler) HandleListShares(c *gin.Context) {
	// 1. Lấy User từ context (do AuthMiddleware nạp vào)
	user, exists := GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 2. Lấy tham số pagination (giống như ListUploadReportsHandler)
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

	// 3. Gọi Service với hàm mới để lấy thông tin đầy đủ
	result, err := h.service.ListSharesWithDetails(c.Request.Context(), user.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 4. Trả về kết quả với pagination info
	c.JSON(http.StatusOK, result)
}

// GET /v1/shares/:id/download
func (h *ShareHandler) HandleDownload(c *gin.Context) {
	// auth + parse id
	user, ok := GetUserFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	idStr := c.Param("id")
	shareID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share id"})
		return
	}

	// tạo presigned url qua service (service sẽ Verify & CheckAndIncrement)
	const expirySec = 120
	url, err := h.service.CreatePresignedURL(c.Request.Context(), shareID, user.ID, expirySec)
	if err != nil {
		if errors.Is(err, share.ErrShareNotFoundOrAccessDenied) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, share.ErrMaxDownloadsExceeded) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "download limit reached"})
			return
		}
		if errors.Is(err, share.ErrShareRevokedOrExpired) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create presigned url"})
		return
	}

	// Return link
	c.JSON(http.StatusOK, gin.H{
		"url":        url,
		"expires_in": expirySec,
	})
}

// GET /v1/shares/:id
func (h *ShareHandler) HandleGetShareMetadata(c *gin.Context) {
	// 1. Lấy User từ context (AuthMiddleware đã nạp vào)
	_, exists := GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 2. Lấy share ID từ URL
	idStr := c.Param("id")
	shareID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid share ID"})
		return
	}

	// 3. Gọi Service để lấy metadata
	data, err := h.service.GetMetadata(c.Request.Context(), shareID)
	if err != nil {
		// Nếu lỗi là share không tồn tại
		c.JSON(http.StatusNotFound, gin.H{"error": "Share not found"})
		return
	}

	// 4. Trả về JSON
	c.JSON(http.StatusOK, data)
}

// Khởi tạo 1 chia sẻ mới
// POST v1/shares
func (h *ShareHandler) HandleCreateShare(c *gin.Context) {
	user, exists := GetUserFromContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req model.CreateShareRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid body: " + err.Error()})
		return
	}

	// validation
	if req.FileID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id is required"})
		return
	}

	createdShare, err := h.service.CreateShare(c.Request.Context(), user.ID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "[debug] Failed to create share"})
	}

	c.JSON(http.StatusCreated, createdShare)
}
