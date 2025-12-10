package main

import (
	"file-sharing/internal/config"
	"file-sharing/internal/share"
	"file-sharing/internal/storage"
	"file-sharing/internal/transport/http"
	"log"


	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// ---- 1. Khởi tạo Database Connection ----
	config, err := config.LoadConfig("./env")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := sqlx.Connect("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	defer db.Close()
	log.Println("Connected to database")

	// ---- 2. Khởi tạo Repository ----
	userRepo := storage.NewUserRepository(db)

	// Tạo MinIO client repo
	minioEndpoint := config.MinioEndpoint
	minioBucket := config.MinioBucket
	var minioRepo *storage.MinioRepo
	if minioEndpoint != "" && minioBucket != "" {
		minioAccess := config.MinioAccessKey
		minioSecret := config.MinioSecretKey
		useSSL := strings.ToLower(config.MinioUseSSL) == "true"
		mr, err := storage.NewMinioRepo(minioEndpoint, minioAccess, minioSecret, minioBucket, useSSL)
		if err != nil {
			log.Printf("Failed to init minio repo: %v", err)
		} else {
			log.Println("MinIO service initialized successfully")
			minioRepo = mr
		}
	} else {
		log.Println("Warning: MinIO not configured - file upload will not be available")
	}
	// Khởi tạo File Repository (Đã sửa lỗi hàm khởi tạo) - nguyen_trung_kien_addded
	fileRepo := storage.NewPostgresFileRepository(db)
	shareRepo := storage.NewShareRepository(db, minioRepo)

	// ---- 3. Khởi tạo Gin Router ----
	router := gin.Default()

	// ---- Khởi tạo Service ----
	shareService := share.NewShareService(shareRepo)

	// --- Khởi tạo Auth Service ---
	authService := share.NewAuthService(shareRepo)

	// ---- 4. Khởi tạo Handlers & Middlewares ----
	userHandler := http.NewUserHandler()
	authMiddleware := http.AuthMiddleware(userRepo)
	fileHandler := http.NewFileHandler(fileRepo, minioRepo)
	listFilesHandler := http.ListUserFilesHandler(db.DB)
	shareHandler := http.NewShareHandler(shareService)
	// Authoize Password Handler
	authorizePasswordHandler := http.NewAuthorizePasswordHandler(authService)

	// Report handlers (register these so client endpoints exist)
	reportCompleteHandler := http.ReportUploadCompleteHandler(db.DB)
	getReportHandler := http.GetUploadReportHandler(db.DB)
	listReportsHandler := http.ListUploadReportsHandler(db.DB)

	// ---- 5. Đăng ký API Routes ----
	api := router.Group("/api")
	{
		// Health check endpoint
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
		authed := api.Group("/")
		authed.Use(authMiddleware)
		{
			authed.GET("/me", userHandler.GetCurrentUser)
			authed.POST("/v1/files", fileHandler.HandleFileInit)
			authed.GET("/v1/files", listFilesHandler)
			// report endpoints
			authed.POST("/v1/files/:file_id/report-complete", reportCompleteHandler)
			authed.GET("/v1/files/:file_id/report", getReportHandler)
			authed.GET("/v1/upload-reports", listReportsHandler)

			// flow 3.5: /share
			authed.POST("v1/shares", shareHandler.HandleCreateShare)
			authed.POST("/v1/shares/:id/revoke", shareHandler.HandleRevoke)
			// authorize password endpoint
			authed.POST("/v1/shares/:id/authorize", authorizePasswordHandler.HandleAuthorizePassword)

			authed.GET("/v1/shares", shareHandler.HandleListShares)
			authed.GET("/v1/shares/:id/download", shareHandler.HandleDownload)

			// get metadata
			authed.GET("/v1/shares/:id", shareHandler.HandleGetShareMetadata)
		}
	}

	// ---- Swagger UI ----
	// Serve OpenAPI YAML at a fixed absolute URL so Swagger UI can fetch it reliably
	router.StaticFile("/openapi.yaml", "./api/openapi.yaml")
	// Important: use absolute URL for swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("http://localhost:8080/openapi.yaml"),
	))

	// ---- 6. Khởi động HTTP Server ----
	log.Printf("Starting server on %s", config.HTTPServerAddress)
	if err := router.Run(config.HTTPServerAddress); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
