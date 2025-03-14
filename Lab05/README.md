# Lab05: Cài đặt Distributed Transaction sử dụng 2-Phase-Commit
## Dependencies

Dự án sử dụng các gói sau (xem chi tiết trong `go.mod`):
- `go.mongodb.org/mongo-driver v1.17.3`: Driver MongoDB cho Go.
- `github.com/stretchr/testify v1.10.0`: Thư viện để viết test case.

## Cách hệ thống hoạt động

### 1. Cấu hình MongoDB
- Hai instance MongoDB được cấu hình trong cùng một Replica Set `rs0`:
  - **MongoDB1**: `localhost:27017`, lưu trữ database `bank1`.
  - **MongoDB2**: `localhost:27018`, lưu trữ database `bank2`.
- Mặc dù được cấu hình trong Replica Set, mã nguồn hiện tại xử lý chúng như hai instance độc lập.

### 2. Logic giao dịch (`transaction.go`)
- **Kiểm tra ban đầu**: Xác minh người gửi và người nhận tồn tại, số dư đủ.
- **Giai đoạn Prepare**:
  - Ghi log trạng thái `"prepared"`.
  - Cập nhật số dư trên MongoDB1 (trừ tiền) và MongoDB2 (cộng tiền).
- **Giai đoạn Commit**:
  - Commit trên MongoDB1, sau đó MongoDB2.
  - Nếu MongoDB2 thất bại, thực hiện giao dịch bù để hoàn tác trên MongoDB1.
- **Log giao dịch**: Lưu trạng thái (`prepared`, `committed`, `compensated`) trong collection `transaction_logs`.

### 3. Test case (`transaction_test.go`)
- **Thành công**: Chuyển 200 từ Alice (1000 → 800) sang Bob (500 → 700).
- **Không đủ số dư**: Thử chuyển 1200 từ Alice (1000), giao dịch thất bại.
- **Người dùng không tồn tại**: Chuyển tiền đến "Charlie", giao dịch thất bại.
- **Rollback**: Kiểm tra rollback khi giao dịch thất bại (chưa mô phỏng lỗi cụ thể).

## Hướng dẫn chạy hệ thống

### Yêu cầu
- **MongoDB**: Đã cài đặt (phiên bản 4.0 trở lên).
- **Go**: Đã cài đặt (phiên bản 1.23.4 hoặc tương thích).
- **Hệ điều hành**: Windows, macOS, hoặc Linux.

### Bước 1: Cài đặt MongoDB và Go
1. **MongoDB**: Tải từ [trang chính thức](https://www.mongodb.com/try/download/community) và cài đặt.
2. **Go**: Tải từ [trang chính thức](https://golang.org/dl/), cài đặt và thêm `go` vào `PATH`.

### Bước 2: Tạo thư mục dữ liệu
- Tạo hai thư mục trong thư mục dự án:
  ```sh
  mkdir -p data/db1 data/db2

### Bước 3: Chạy hai instance MongoDB
- Mở terminal đầu tiên, chạy MongoDB1:
```sh 
mongod --dbpath data/db1 --replSet rs0 --port 27017 --bind_ip localhost 
```
- Mở terminal thứ hai, chạy MongoDB2:
```sh
mongod --dbpath data/db2 --replSet rs0 --port 27018 --bind_ip localhost
```
### Bước 4: Khởi tạo Replica Set
- Mở terminal thứ ba và kết nối vào MongoDB shell của instance đầu tiên:
```sh 
mongo --port 27017
```
- Trong shell, chạy lệnh để khởi tạo Replica Set:
```javascript
rs.initiate({
  _id: "rs0",
  members: [
    { _id: 0, host: "localhost:27017" },
    { _id: 1, host: "localhost:27018" }
  ]
})
```
- Kiểm tra trạng thái
```javascript
rs.status()
```
- Tạo test data
```javascript
use bank1
db.accounts.insertOne({ name: "Alice", balance: 1000 })

use bank2
db.accounts.insertOne({ name: "Bob", balance: 500 })
```
- Đảm bảo thấy Primary (27017) và Secondary (27018) hoạt động.
### Bước 5: Cài đặt dependencies
```sh
go mod tidy
```
### Bước 6: Chạy chương trình
```sh
go run main.go
```
Kết quả hiển thị `PASS` hoặc `FAIL`

## Lưu ý

- **Kiểm tra kết nối:**  Đảm bảo hai instance MongoDB đang chạy trước khi chạy ứng dụng Go.

- **Xử lý lỗi:**  Nếu test thất bại do kiểu dữ liệu (ví dụ: `int` vs `int32`), hãy điều chỉnh `assert.Equal` trong file `transaction_test.go` để ép kiểu dữ liệu phù hợp.

- **Tối ưu hóa:**  Để đảm bảo tính nguyên tử hoàn toàn, bạn có thể sửa mã sử dụng URI Replica Set thay vì triển khai 2PC thủ công:

  ```mongodb
  mongodb://localhost:27017,localhost:27018/?replicaSet=rs0

## Điểm mạnh và hạn chế

### Điểm mạnh

- **Tính nhất quán** được đảm bảo qua 2PC và log giao dịch.
- Hỗ trợ **rollback thủ công** khi có lỗi.
- **Dễ kiểm tra** thông qua các test case.

### Hạn chế

- Không **nguyên tử hoàn toàn** khi dùng hai instance MongoDB độc lập.
- Việc ghi log giao dịch làm **tăng độ trễ**.
- Phụ thuộc vào **giao dịch bù** nếu quá trình Commit thất bại.

## Đề xuất cải tiến

- Sử dụng một client MongoDB duy nhất với URI Replica Set để tận dụng giao dịch nguyên tử sẵn có của MongoDB:
  ```go
  client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017,localhost:27018/?replicaSet=rs0"))
  ```
- Loại bỏ 2PC thủ công và thay bằng **giao dịch đa tài liệu** của MongoDB.


