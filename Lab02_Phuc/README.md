# Lab02: Lab 02 - Distributed Key-Value Store with Replication

## Giới Thiệu

Dự án này triển khai một hệ thống lưu trữ key-value phân tán, có khả năng:
- Sao lưu dữ liệu trên nhiều server để đảm bảo tính sẵn sàng cao.
- Sử dụng thuật toán Bully để bầu chọn leader trong trường hợp lỗi xảy ra.
- Hỗ trợ các thao tác cơ bản: PUT, GET, và đồng bộ dữ liệu giữa các server.

## Cấu Trúc Dự Án

Dự án bao gồm ba thành phần chính:

1. **Client.go**:
    - Gửi yêu cầu PUT và GET tới server chính (primary server).
    - Tự động phát hiện server chính nếu server hiện tại không phản hồi.
    - Xác thực dữ liệu đã được sao lưu trên tất cả các server.

2. **Server.go**:
    - Xử lý các yêu cầu PUT và GET từ client.
    - Đồng bộ hóa dữ liệu tới các server sao lưu.
    - Tự động bầu chọn leader mới nếu server chính gặp lỗi.

3. **types.go**:
    - Định nghĩa các kiểu dữ liệu và mã lỗi dùng chung cho client và server.

## Cách Chạy

### 1. Khởi Chạy Server
Server có thể được chạy bằng cách sử dụng 3 cổng port mặc định là `1234`, `1235` và `1236`. Ví dụ:
```bash
go run Server.go -id=1 -port=1234
go run Server.go -id=2 -port=1235
go run Server.go -id=3 -port=1236
```

### 2. Khởi Chạy Client
```bash
go run Client.go
```

### 3. Tình Huống Mô Phỏng
- TC01: PUT/GET khi tất cả các server đang hoạt động.

- TC02: Mô phỏng lỗi server chính, chờ bầu chọn leader mới, và thực hiện PUT/GET

## Hệ thống hỗ trợ

### Thuật toán Bully

Hệ thống sử dụng thuật toán Bully để bầu chọn leader:

- Server có ID cao nhất được chọn làm leader.
- Khi phát hiện leader bị lỗi, server sẽ tự động gửi yêu cầu bầu chọn.
- Nếu không có phản hồi từ server có ID cao hơn, server hiện tại sẽ trở thành leader.
- Leader mới thông báo đến tất cả các server còn lại.

### Cơ chế heartbeat
Cơ chế heartbeat giúp theo dõi trạng thái của leader và phát hiện khi leader gặp lỗi:

#### 1. Cách Hoạt Động
- Leader (Primary Server) gửi tín hiệu heartbeat đến các server còn lại theo chu kỳ (mỗi 2 giây).
- Các server follower cập nhật `lastHeartBeat` khi nhận được tín hiệu từ leader.
- Nếu một follower không nhận được heartbeat từ leader trong 5 giây, nó sẽ coi leader đã chết và khởi động quá trình bầu chọn leader mới.
#### 2. Cách Thực Hiện
- Leader gửi heartbeat: Gửi RPC `Heartbeat()` đến tất cả các follower. Nếu follower phản hồi, leader tiếp tục hoạt động bình thường. Nếu follower không phản hồi, leader ghi log lỗi nhưng vẫn tiếp tục gửi heartbeat.

- Follower nhận heartbeat: Khi nhận được heartbeat, follower cập nhật giá trị lastHeartBeat của nó. Follower ghi nhận leaderID của leader hiện tại.

- Phát hiện lỗi leader: Follower kiểm tra giá trị lastHeartBeat của mình mỗi 3 giây. Nếu không nhận được heartbeat trong 5 giây, follower sẽ kích hoạt quá trình bầu chọn leader mới (`thuật toán Bully`).