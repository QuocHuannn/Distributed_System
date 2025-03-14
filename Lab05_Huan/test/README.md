# Hướng dẫn chạy test cho hệ thống giao dịch phân tán

Thư mục này chứa các file test để kiểm tra chức năng của hệ thống giao dịch phân tán.

## Cấu trúc thư mục

- `integration_test.go`: Kiểm tra tích hợp toàn bộ hệ thống
- `unit_test.go`: Kiểm tra các thành phần riêng lẻ của hệ thống

## Chạy unit test

Unit test kiểm tra các thành phần riêng lẻ của hệ thống mà không cần khởi động toàn bộ hệ thống.

```bash
cd Lab05
go test -v ./test -run "TestConsistentHash|TestDataStore|TestTransactionManager|TestTransferMoney|TestBookTrip"
```

## Chạy integration test

Integration test kiểm tra toàn bộ hệ thống, bao gồm việc khởi động Coordinator và các Server.

```bash
cd Lab05
go test -v ./test -run TestSystem
```

## Các test case

### Unit Test

1. **TestConsistentHash**: Kiểm tra thuật toán consistent hashing
   - AddNode: Thêm node vào hash ring
   - RemoveNode: Xóa node khỏi hash ring
   - GetNodeForKey: Lấy node chịu trách nhiệm cho một key

2. **TestDataStore**: Kiểm tra các thao tác cơ bản trên data store
   - SetAndGet: Đặt và lấy giá trị
   - Delete: Xóa key
   - NonExistent: Kiểm tra key không tồn tại

3. **TestTransactionManager**: Kiểm tra transaction manager
   - BeginTransaction: Bắt đầu giao dịch
   - GetTransaction: Lấy giao dịch theo ID
   - NonExistentTransaction: Kiểm tra giao dịch không tồn tại

4. **TestTransferMoney**: Kiểm tra chức năng chuyển tiền
   - SuccessfulTransfer: Chuyển tiền thành công
   - InsufficientFunds: Chuyển tiền thất bại do không đủ tiền

5. **TestBookTrip**: Kiểm tra chức năng đặt chỗ
   - SuccessfulBooking: Đặt chỗ thành công

### Integration Test

1. **TestSystem**: Kiểm tra toàn bộ hệ thống
   - InitializeAccounts: Khởi tạo tài khoản
   - TransferMoney: Chuyển tiền
   - BookTrip: Đặt chỗ
   - TransferMoneyInsufficientFunds: Chuyển tiền với số dư không đủ
   - ConcurrentTransactions: Nhiều giao dịch đồng thời

## Lưu ý

- Integration test sẽ khởi động các process thực tế, nên cần đảm bảo các port không bị sử dụng bởi các ứng dụng khác.
- Sau khi chạy integration test, các process có thể vẫn chạy trong background. Sử dụng lệnh `ps` và `kill` để dừng chúng nếu cần.
- Các test case được thiết kế để chạy độc lập, nhưng một số test case có thể phụ thuộc vào kết quả của các test case khác trong cùng một nhóm test. 