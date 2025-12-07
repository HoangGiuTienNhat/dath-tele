package model

import "time"

type Share struct {
	ID              int64      `json:"id" db:"id"`
	FileID          int64      `json:"file_id" db:"file_id"`
	OwnerUserID     int64      `json:"owner_user_id" db:"owner_user_id"`
	Hash            string     `json:"hash" db:"hash"`
	RequirePassword bool       `json:"require_password" db:"require_password"`
	HashPassword    string     `json:"-" db:"hash_password"`
	Revoked         bool       `json:"revoked" db:"revoked"`
	ExpiresAt       *time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}

type ShareMetadata struct {
	Share *Share
	File  *FileWithOwner
	User  *User
}

type FileMetadataDTO struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	ObjectKey string `json:"object_key"`
	Size      int64  `json:"size"`
	Mime      string `json:"mime"`
	Status    string `json:"status"`
}

type UserMinimalDTO struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	TelegramID int64  `json:"telegram_user_id"`
}

type ShareMetadataResponseDTO struct {
	ID        int64           `json:"id"`
	Hash      string          `json:"hash"`
	Revoked   bool            `json:"revoked"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	File      FileMetadataDTO `json:"file"`
	Owner     UserMinimalDTO  `json:"owner"`
}

type CreateShareRequest struct {
	FileID    int64      `json:"file_id" binding:"required"`
	Password  string     `json:"password"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// ShareListItemDTO đại diện cho một item trong danh sách shares của người dùng
type ShareListItemDTO struct {
	ID              int64      `json:"id"`
	FileID          int64      `json:"file_id"`
	Hash            string     `json:"hash"`
	RequirePassword bool       `json:"require_password"`
	Revoked         bool       `json:"revoked"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Filename        string     `json:"filename"`
	FileSize        int64      `json:"file_size"`
	FileMime        string     `json:"file_mime"`
	FileStatus      string     `json:"file_status"`
}

// ShareListResponseDTO là response cho API liệt kê shares
type ShareListResponseDTO struct {
	Shares []ShareListItemDTO `json:"shares"`
	Total  int64              `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}
