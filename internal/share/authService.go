package share

import (
	"context"
	"file-sharing/internal/storage"
	"file-sharing/internal/files"
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"
	"time"
	
)

type AuthServices interface{
// Hàm AuthorizeSharePassword trả về token truy cập tạm thời nếu mật khẩu đúng
	AuthorizeSharePasswordAndIssueToken(ctx context.Context, shareID int64, password string) (token string, err error)
	// VerifyShareToken xác minh token JWT và trả về shareID nếu hợp lệ
	VerifyShareToken(token string) (shareID int64, err error)
}

type AuthService struct {
	repo storage.ShareRepository
}
func NewAuthService(repo storage.ShareRepository) AuthServices {
	return &AuthService{
		repo: repo,
	}
}

// Triển khai các hàm liên quan đến authorize password
func CheckPasswordHash(password, hash string) bool {
	// Giả sử sử dụng bcrypt để so sánh
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if  err == nil{
		println("Correct")
		return true
	} else if err == bcrypt.ErrMismatchedHashAndPassword {
		println("Wrong Password!")
		return false
	} else {
		println("Another errors!")
		return false
	}
}

func GenerateAccessToken(shareID int64) (string, error) {
	// Giả sử sử dụng JWT để tạo token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"share_id": shareID,
		"exp":      time.Now().Add(15 * time.Minute).Unix(), // Token có hạn trong 15 phút
	})
	secretKey := []byte("your-secret-key") // Thay bằng khóa bí mật thực tế
	return token.SignedString(secretKey)
}

func VerifyAccessToken(tokenString string) (int64, error) {
	secretKey := []byte("your-secret-key") // Phải giống secret key dùng để tạo token
	token, err := jwt.ParseWithClaims(tokenString, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		return secretKey, nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(*jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, jwt.ErrSignatureInvalid
	}

	shareID, ok := (*claims)["share_id"].(float64)
	if !ok {
		return 0, jwt.ErrInvalidKeyType
	}

	return int64(shareID), nil
}

func (a *AuthService) AuthorizeSharePasswordAndIssueToken(ctx context.Context, shareID int64, password string) (token string, err error) {
	// Lấy hash mật khẩu từ repository
	passwordHash, err := a.repo.GetPasswordHash(ctx, shareID)
	if err != nil {
		return "", err
	}
	
	// So sánh mật khẩu đã mã hóa với mật khẩu cung cấp
	if !CheckPasswordHash(password, passwordHash) {
		return "", files.InvalidPasswordError()
	}
	// Nếu đúng, trả về một token để truy cập tạm thời
	token, err = GenerateAccessToken(shareID)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (a *AuthService) VerifyShareToken(tokenString string) (shareID int64, err error) {
	return VerifyAccessToken(tokenString)
}