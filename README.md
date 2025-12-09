# Telegram file sharing

### Danh sách thành viên
| STT | MSSV    | Tên                    |
|-----|---------|-------------------------|
| 1   | 2212370 | Nguyễn Trọng Nhân      |
| 2   | 2213696 | Nguyễn Chí Trung       |
| 3   | 2352651 | Nguyễn Ngọc Tuấn Kiệt  |
| 4   | 2312535 | Nguyễn Quỳnh Như       |
| 5   | 2313854 | Trần Hoàng Uyên        |
| 6   | 2252396 | Nguyễn Trung Kiên      |
| 7   | 2312397 | Lê Bá Nguyễn           |
| 8   | 2313452| Lê Trọng Tín           |
| 9   | 2313739 | Mai Anh Tuấn           |
|10   | 2312460 | Hoàng Giữ Tiến Nhất    |

## Hướng dẫn chạy dự án (Local Development)

1.  **Cài đặt môi trường:**

    - Cài đặt [Docker](https://www.docker.com/products/docker-desktop/) và Docker Compose.
    - Cài đặt [Go](https://go.dev/doc/install) (phiên bản 1.2x trở lên).

2.  **Cấu hình môi trường:**

    - Copy file `env/example.env` thành `env/dev.env`.
    - Cập nhật các biến môi trường trong `env/dev.env` cho phù hợp với máy local của bạn.

3.  **Chạy ứng dụng Go:**

    - Tải các thư viện cần thiết: `go mod tidy`
    - Chạy dịch vụ API: `go run ./cmd/api/main.go`

4.  **Truy cập Swagger UI:**

    - Truy cập [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) để xem tài liệu API và thử nghiệm các endpoint.
    - Xem các Mock Conversation Flow trong thư mục `docs/` để hiểu cách tương tác với bot.

## Cấu trúc thư mục

Dưới đây là mô tả chức năng của các thư mục chính trong dự án:

```
.
├── README.md                      # Giới thiệu, thành viên, hướng dẫn nhanh
├── docker-compose.yml             # Cấu hình Docker Compose cho môi trường dev và prod
├── .github/workflows/             # Cấu hình CI/CD
│  ├── ci.yml                      # Build, test, scan source code
│  └─ deploy.yml                  # Deploy lên server
├── deploy/                        # Cấu hình cho môi trường production
│  ├── nginx/                      # Cấu hình Nginx reverse proxy
│  └─ k8s/                        # Cấu hình Kubernetes (tương lai)
├── env/                           # Chứa các file biến môi trường
│  ├── example.env                 # File mẫu
│  ├─ dev.enc                     # File mã hoá cho môi trường dev (dùng Sops)
│  └─ prod.enc                    # File mã hoá cho môi trường prod (dùng Sops)
├── cmd/                           # Điểm khởi chạy (main) của các ứng dụng
│  ├── api/                        # Main package cho dịch vụ API
│  └─ worker/                     # Main package cho các job chạy nền
├── internal/                      # Logic nghiệp vụ và mã nguồn chính của ứng dụng
│  ├── config/                     # Đọc và quản lý cấu hình
│  ├── auth/                       # Xử lý xác thực (Telegram user, TOTP)
│  ├── files/                      # Xử lý file (upload, S3, scan virus)
│  ├── share/                      # Rule engine cho việc chia sẻ file
│  ├── users/                      # Quản lý thông tin người dùng
│  ├── audit/                      # Ghi log truy cập
│  ├── storage/                    # Tương tác với database (Postgres, Redis)
│  ├── transport/                  # Xử lý giao tiếp với bên ngoài
│  │  ├── http/                    # REST handlers
│  │  └─ telegram/                # Telegram SDK wrappers/DTO
│  └─ pkg/                        # Các thư viện, tiện ích dùng chung
├── api/                           # Định nghĩa API
│  └─ openapi.yaml                # API Spec (Swagger/OpenAPI)
├── migrations/                    # Quản lý phiên bản database
│  └─ *.sql                       # Các file SQL migration
├── docs/                          # Tài liệu chi tiết
│                                  # Deploy guide, User guide, API docs…
└─ reports/                       # Báo cáo môn học
```

## Quy trình phát triển một tính năng mới

1.  **Tạo nhánh mới:** Luôn bắt đầu một tính năng mới trên một nhánh riêng. Tên nhánh nên có dạng `feature/<ten-tinh-nang>`.

    ```bash
    git checkout -b feature/my-new-feature
    ```

2.  **Cập nhật Database (Nếu cần):**

    - Tạo một file migration mới trong thư mục `migrations/` với tên theo thứ tự (ví dụ: `0003_add_new_table.sql`).
    - Viết các câu lệnh SQL để thay đổi cấu trúc database.

3.  **Cập nhật API Spec (Nếu cần):**

    - Chỉnh sửa file `api/openapi.yaml` để định nghĩa các endpoint, request, response mới.
    - Sử dụng các công cụ code-gen để tự động sinh ra các model và interface từ file spec (nếu có).

4.  **Viết code logic:**

    - **Storage:** Thêm các phương thức mới trong thư mục `internal/storage/` để tương tác với database.
    - **Business Logic:** Viết logic nghiệp vụ chính trong các thư mục tương ứng trong `internal/` (ví dụ: `internal/users/`, `internal/files/`).
    - **Transport/Handler:** Tạo các HTTP handler mới trong `internal/transport/http/` để xử lý request từ client.
    - Kết nối các thành phần trên trong `cmd/api/main.go`.

5.  **Viết Tests:** Viết unit test và integration test cho các logic và handler mới.

6.  **Tạo Pull Request:**

    - Commit code và đẩy nhánh lên repository.
    - Tạo một Pull Request (PR) vào nhánh `develop` hoặc `main`.
    - Mô tả chi tiết các thay đổi trong PR và yêu cầu review từ các thành viên khác.

7.  **Merge và Dọn dẹp:** Sau khi PR được duyệt và merge, xoá nhánh feature đã làm việc.
    ```bash
    git branch -d feature/my-new-feature
    ```
