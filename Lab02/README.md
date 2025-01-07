# Lab 02 - Distributed Key-Value Store with Replication

Hệ thống **Key-Value Store** với cơ chế **Primary-Backup Replication** và **Bully Algorithm** cho **Leader Election**.

## Tính năng

- **Primary-Backup Replication**
- **Automatic Leader Election** sử dụng **Bully Algorithm**
- **Sequential Consistency** với **timestamp**
- Tự động phục hồi khi **Primary** fails
- **Client** tự động phát hiện **Primary server**

## Cách chạy

### 1. Khởi động các Server

Mở 3 terminal riêng biệt và chạy các lệnh sau:

- **Terminal 1 (Primary Server)**: 
  ```bash
  go run Server/Server.go -id 1 -port 1234
  ```
- **Terminal 2 (Backup Server 1)**: 
  ```bash
  go run Server/Server.go -id 2 -port 1235
  ```
- **Terminal 3 (Backup Server 2)**: 
  ```bash
  go run Server/Server.go -id 3 -port 1236
  ```

### 2. Khởi động Client

Mở terminal mới và chạy: 
```bash
go run Client/Client.go
```

## Test Cases

### TC01: PUT/GET to/from the primary, GET from backup

1. Chạy cả 3 server như hướng dẫn trên.
2. Chạy client để thực hiện **PUT**.
3. Kiểm tra giá trị trên cả **primary** và **backup servers**.

**Kết quả mong đợi**:
- **PUT** thành công trên **primary**.
- Dữ liệu được **replicate** sang các **backup**.
- **GET** từ bất kỳ server nào cũng trả về giá trị mới nhất.

### TC02: Primary Failure

1. Chạy cả 3 server.
2. Tắt **primary server** (Ctrl+C trên terminal của server ID 1).
3. Chờ khoảng 2-3 giây cho quá trình bầu cử.
4. Thực hiện **PUT/GET** mới.

**Kết quả mong đợi**:
- Hệ thống tự động bầu **primary** mới.
- **Client** tự động kết nối đến **primary** mới.
- Các thao tác **PUT/GET** vẫn hoạt động bình thường.

### TC03: Sequential Consistency

1. Chạy các server.
2. Chạy nhiều **client** đồng thời.
3. Thực hiện nhiều **PUT** cùng lúc.

**Kết quả mong đợi**:
- Các thao tác được đảm bảo **sequential consistency** nhờ **timestamp**.
- Không xảy ra **race condition**.
- Tất cả các **backup** đều nhận được dữ liệu theo đúng thứ tự.

## Xử lý lỗi

1. **Nếu không kết nối được server**:
   - Kiểm tra port có đang được sử dụng không.
   - Đảm bảo **firewall** không chặn kết nối.

2. **Nếu primary election không hoạt động**:
   - Kiểm tra **log** của các server.
   - Đảm bảo tất cả server có thể kết nối với nhau.

3. **Nếu replication không hoạt động**:
   - Kiểm tra kết nối mạng giữa các server.
   - Xem **log** để tìm lỗi cụ thể.

## Monitoring

- **Server logs** sẽ hiển thị trạng thái của server (**Primary/Backup**).
- **Election logs** cho biết quá trình bầu cử.
- **Client logs** hiển thị kết quả của các thao tác **PUT/GET**.

## Giới hạn

- Chưa có **persistent storage**.
- Chưa có cơ chế **recovery** khi toàn bộ hệ thống crash.
- Chưa có cơ chế xử lý **partition tolerance** đầy đủ.
