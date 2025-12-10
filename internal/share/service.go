package share

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"file-sharing/internal/model"
	"file-sharing/internal/storage"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	RevokeShare(ctx context.Context, shareID int64, userID int64) error

	ListShares(ctx context.Context, userID int64, limit, offset int) ([]model.Share, error)

	// ListSharesWithDetails lấy danh sách shares kèm thông tin file và tổng số
	ListSharesWithDetails(ctx context.Context, userID int64, limit, offset int) (*model.ShareListResponseDTO, error)

	// PrepareDownload validates share, increments counter, and returns presigned URL with metadata
	// Returns: presignedURL, filename, requirePassword, error
	PrepareDownload(ctx context.Context, shareID int64, requesterUserID int64) (url string, filename string, requirePassword bool, err error)

	GetMetadata(ctx context.Context, id int64) (*model.ShareMetadataResponseDTO, error)
	CreateShare(ctx context.Context, userID int64, req *model.CreateShareRequest) (*model.Share, error)
}

type shareService struct {
	repo storage.ShareRepository
}

func NewShareService(repo storage.ShareRepository) Service {
	return &shareService{
		repo: repo,
	}
}

// Hàm sinh  chuỗi hash ngẫu nhiên
func generateRandomHash(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *shareService) RevokeShare(ctx context.Context, shareID int64, userID int64) error {
	return s.repo.RevokeShare(ctx, shareID, userID)
}

// ListShares lists all shares for a user
func (s *shareService) ListShares(ctx context.Context, userID int64, limit, offset int) ([]model.Share, error) {
	reports, err := s.repo.ListSharesByOwnerUserID(ctx, userID, limit, offset)
	if err != nil {
		log.Printf("Failed to list shares in service for user %d: %v", userID, err)
		return nil, err
	}

	if reports == nil {
		reports = []model.Share{} // Trả về mảng rỗng thay vì null
	}

	return reports, nil
}

// ListSharesWithDetails lấy danh sách shares kèm thông tin file và tổng số để hỗ trợ pagination
func (s *shareService) ListSharesWithDetails(ctx context.Context, userID int64, limit, offset int) (*model.ShareListResponseDTO, error) {
	// Lấy danh sách shares kèm thông tin file
	shares, err := s.repo.ListSharesWithFileByOwnerUserID(ctx, userID, limit, offset)
	if err != nil {
		log.Printf("Failed to list shares with details for user %d: %v", userID, err)
		return nil, err
	}

	// Đếm tổng số shares
	total, err := s.repo.CountSharesByOwnerUserID(ctx, userID)
	if err != nil {
		log.Printf("Failed to count shares for user %d: %v", userID, err)
		return nil, err
	}

	if shares == nil {
		shares = []model.ShareListItemDTO{} // Trả về mảng rỗng thay vì null
	}

	return &model.ShareListResponseDTO{
		Shares: shares,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// PrepareDownload validates share and returns objectKey, filename, requirePassword
// Checks: exists, revoked/expired, download limit
func (s *shareService) PrepareDownload(ctx context.Context, shareID int64, requesterUserID int64) (string, string, bool, error) {
	// load share meta
	shareRec, err := s.repo.GetShareByID(ctx, shareID)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // not found
			return "", "", false, ErrShareNotFoundOrAccessDenied
		}
		return "", "", false, err
	}

	// check expired/revoked
	if shareRec != nil {
		if shareRec.Revoked { // revoked
			return "", "", false, ErrShareRevokedOrExpired
		}
		if shareRec.ExpiresAt != nil && shareRec.ExpiresAt.Before(time.Now()) { // expired
			return "", "", false, ErrShareRevokedOrExpired
		}
	}

	// lấy object key + filename
	objectKey, filename, err := s.repo.GetActiveFilePathByShareID(ctx, shareID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", false, ErrShareNotFoundOrAccessDenied
		}
		return "", "", false, err
	}

	// check + increment download counter (Hiện là no-op)
	err = s.repo.CheckAndIncrementDownload(ctx, shareID)
	if err != nil {
		return "", "", false, ErrMaxDownloadsExceeded
	}

	// generate presigned URL (120 seconds expiry)
	const expirySec = 120
	url, err := s.repo.PresignObject(ctx, objectKey, expirySec)
	if err != nil {
		return "", "", false, err
	}

	// return URL, filename, requirePassword
	requirePassword := false
	if shareRec != nil {
		requirePassword = shareRec.RequirePassword
	}

	return url, filename, requirePassword, nil
}

var (
	// trả về khi share không tồn tại hoặc không được phép truy cập
	ErrShareNotFoundOrAccessDenied = storage.ErrShareNotFoundOrAccessDenied
	// trả về khi share đã bị revoke hoặc đã hết hạn
	ErrShareRevokedOrExpired = errors.New("share revoked or expired")
	// trả khi đã đạt max_downloads
	ErrMaxDownloadsExceeded = errors.New("max downloads exceeded")
)

func (s *shareService) GetMetadata(ctx context.Context, id int64) (*model.ShareMetadataResponseDTO, error) {
	md, err := s.repo.GetShareMetadata(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.ShareMetadataResponseDTO{
		ID:        md.Share.ID,
		Hash:      md.Share.Hash,
		Revoked:   md.Share.Revoked,
		ExpiresAt: md.Share.ExpiresAt,
		CreatedAt: md.Share.CreatedAt,
		File: model.FileMetadataDTO{
			ID:        md.File.ID,
			Filename:  md.File.Filename,
			ObjectKey: md.File.ObjectKey,
			Size:      md.File.Size,
			Mime:      md.File.Mime,
			Status:    md.File.Status,
		},
		Owner: model.UserMinimalDTO{
			ID:         md.User.ID,
			Username:   md.User.Username,
			TelegramID: md.User.TelegramID,
		},
	}, nil
}

func (s *shareService) CreateShare(ctx context.Context, userID int64, req *model.CreateShareRequest) (*model.Share, error) {
	// Tạo chuỗi hash unique cho link (ví dụ 8 bytes -> 16 ký tự hex)
	linkHash, err := generateRandomHash(8)
	if err != nil {
		return nil, err
	}

	// Xử lý mật khẩu (nếu có)
	var passwordHash string
	requirePassword := false

	if req.Password != "" {
		requirePassword = true
		// Hash mật khẩu bằng bcrypt
		bytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), 14)
		if err != nil {
			return nil, err
		}
		passwordHash = string(bytes)
	}

	// Tạo model share
	newShare := &model.Share{
		FileID:          req.FileID,
		OwnerUserID:     userID,
		Hash:            linkHash,
		RequirePassword: requirePassword,
		HashPassword:    passwordHash,
		ExpiresAt:       req.ExpiresAt,
		Revoked:         false,
	}

	// Gọi Repo để lưu
	return s.repo.CreateShare(ctx, newShare)
}
