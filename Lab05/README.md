# Distributed Transaction System (Lab05)

Hệ thống giao dịch phân tán sử dụng Two-Phase Commit (2PC) để đảm bảo tính nhất quán trong các giao dịch trên nhiều máy chủ.

## Kiến trúc hệ thống

Hệ thống bao gồm các thành phần sau:

1. **Coordinator**: Điều phối giao dịch phân tán, thực hiện Two-Phase Commit
2. **Servers**: Lưu trữ dữ liệu và tham gia vào giao dịch phân tán
3. **Client**: Giao tiếp với hệ thống, thực hiện các giao dịch

### Biên dịch

```bash
cd Lab05
go build -o coordinator Coordinator/coordinator.go
go build -o server Server/server.go
go build -o client Client/client.go
```

## Chạy hệ thống

### 1. Khởi động Coordinator

```bash
./coordinator --address localhost:8000
```

### 2. Khởi động các Servers

```bash
./server --address localhost:8001 --coordinator localhost:8000
./server --address localhost:8002 --coordinator localhost:8000
./server --address localhost:8003 --coordinator localhost:8000
```

### 3. Chạy Client

```bash
./client --coordinator localhost:8000 --interactive
```

## Các giao dịch được hỗ trợ

### 1. TransferMoney

Chuyển tiền giữa hai tài khoản nằm trên các máy chủ khác nhau.

```
transfer <from_account> <to_account> <amount>
```

Ví dụ:
```
transfer account1 account2 100.50
```

### 2. BookTrip

Đặt chỗ cho nhiều dịch vụ (xe, khách sạn, chuyến bay) nằm trên các máy chủ khác nhau.

```
book <car_type> <car_days> <hotel_type> <hotel_nights> <flight_class> <flight_seats>
```

Ví dụ:
```
book SUV 3 Deluxe 2 Economy 2
```

### 3. Các thao tác cơ bản

```
get <key>
set <key> <value>
delete <key>
```

## Cách hoạt động

### Two-Phase Commit (2PC)

1. **Phase 1 (Prepare)**: Coordinator gửi yêu cầu PREPARE đến tất cả các server tham gia. Mỗi server kiểm tra xem có thể thực hiện giao dịch không và trả lời.

2. **Phase 2 (Commit/Abort)**:
   - Nếu tất cả các server trả lời OK, Coordinator gửi yêu cầu COMMIT đến tất cả các server.
   - Nếu bất kỳ server nào trả lời ERROR, Coordinator gửi yêu cầu ABORT đến tất cả các server.

### Consistent Hashing

Hệ thống sử dụng Consistent Hashing để xác định server nào chịu trách nhiệm cho một key cụ thể, giúp phân phối dữ liệu đồng đều và giảm thiểu việc phân phối lại khi thêm/xóa server.

## Xử lý lỗi

- Nếu một server không phản hồi trong quá trình giao dịch, giao dịch sẽ bị hủy bỏ.
- Hệ thống ghi log các giao dịch để có thể khôi phục trong trường hợp lỗi.

## Giới hạn

- Hệ thống hiện tại không hỗ trợ khôi phục tự động sau khi lỗi.
- Không có cơ chế phát hiện deadlock.
- Chưa có cơ chế bảo mật.

## Tài liệu tham khảo

- [FoundationDB Building Clusters](https://apple.github.io/foundationdb/building-cluster.html)
- [FoundationDB Class Scheduling in Go](https://apple.github.io/foundationdb/class-scheduling-go.html)
- [MongoDB Sharded Cluster Deployment](https://www.mongodb.com/docs/manual/tutorial/deploy-shard-cluster/)
- [MongoDB Transactions](https://www.mongodb.com/docs/manual/core/transactions/) 