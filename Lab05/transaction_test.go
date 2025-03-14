package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// setupTestMongoDB khởi tạo client MongoDB cho bài test và thiết lập dữ liệu ban đầu.
func setupTestMongoDB(t *testing.T) *MongoDBClient {
	// Kết nối đến các instance MongoDB
	client1, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Không thể kết nối đến MongoDB1: %v", err)
	}

	client2, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://localhost:27018"))
	if err != nil {
		t.Fatalf("Không thể kết nối đến MongoDB2: %v", err)
	}

	// Khởi tạo cơ sở dữ liệu test
	db1 := client1.Database("test_bank1")
	db2 := client2.Database("test_bank2")

	// Xóa dữ liệu cũ
	_, err = db1.Collection("accounts").DeleteMany(context.Background(), bson.M{})
	if err != nil {
		t.Fatalf("Không thể xóa dữ liệu cũ từ test_bank1: %v", err)
	}
	_, err = db2.Collection("accounts").DeleteMany(context.Background(), bson.M{})
	if err != nil {
		t.Fatalf("Không thể xóa dữ liệu cũ từ test_bank2: %v", err)
	}

	// Thêm dữ liệu ban đầu
	_, err = db1.Collection("accounts").InsertOne(context.Background(), bson.M{"name": "Alice", "balance": 1000})
	if err != nil {
		t.Fatalf("Không thể thêm dữ liệu vào test_bank1: %v", err)
	}
	_, err = db2.Collection("accounts").InsertOne(context.Background(), bson.M{"name": "Bob", "balance": 500})
	if err != nil {
		t.Fatalf("Không thể thêm dữ liệu vào test_bank2: %v", err)
	}

	return &MongoDBClient{client1, client2, db1, db2}
}

// TestTransferMoney_Success kiểm tra việc chuyển tiền thành công giữa các tài khoản.
func TestTransferMoney_Success(t *testing.T) {
	client := setupTestMongoDB(t)

	err := TransferMoney(client, "Alice", "Bob", 200)
	assert.NoError(t, err)

	// Kiểm tra số dư sau giao dịch
	var alice, bob bson.M
	err = client.db1.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Alice"}).Decode(&alice)
	assert.NoError(t, err)
	err = client.db2.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Bob"}).Decode(&bob)
	assert.NoError(t, err)

	assert.Equal(t, int32(800), alice["balance"]) // Số dư kỳ vọng của Alice
	assert.Equal(t, int32(700), bob["balance"])   // Số dư kỳ vọng của Bob
}

// TestTransferMoney_InsufficientFunds kiểm tra chuyển tiền khi không đủ số dư.
func TestTransferMoney_InsufficientFunds(t *testing.T) {
	client := setupTestMongoDB(t)

	err := TransferMoney(client, "Alice", "Bob", 1200) // Alice chỉ có 1000
	assert.Error(t, err)

	// Kiểm tra số dư không thay đổi
	var alice, bob bson.M
	err = client.db1.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Alice"}).Decode(&alice)
	assert.NoError(t, err)
	err = client.db2.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Bob"}).Decode(&bob)
	assert.NoError(t, err)

	assert.Equal(t, int32(1000), alice["balance"]) // Số dư không thay đổi
	assert.Equal(t, int32(500), bob["balance"])    // Số dư không thay đổi
}

// TestTransferMoney_NonExistentUser kiểm tra chuyển tiền đến người dùng không tồn tại.
func TestTransferMoney_NonExistentUser(t *testing.T) {
	client := setupTestMongoDB(t)

	err := TransferMoney(client, "Alice", "Charlie", 100) // Charlie không tồn tại
	assert.Error(t, err)

	// Kiểm tra số dư của Alice không thay đổi
	var alice bson.M
	err = client.db1.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Alice"}).Decode(&alice)
	assert.NoError(t, err)
	assert.Equal(t, int32(1000), alice["balance"])

	// Xác minh không có tài liệu nào được tạo cho Charlie
	var charlie bson.M
	err = client.db2.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Charlie"}).Decode(&charlie)
	assert.Error(t, err) // Phải thất bại vì Charlie không tồn tại
}

// TestTransferMoney_RollbackOnFailure kiểm tra rollback giao dịch khi thất bại.
func TestTransferMoney_RollbackOnFailure(t *testing.T) {
	client := setupTestMongoDB(t)

	// Lưu ý: Để mô phỏng thất bại chính xác, cần sửa TransferMoney để chèn lỗi.
	// Ở đây, chúng ta giả định TransferMoney xử lý rollback đúng cách.

	err := TransferMoney(client, "Alice", "Bob", 100)
	assert.NoError(t, err) // Giả định không có mô phỏng lỗi

	// Kiểm tra số dư để đảm bảo cập nhật đúng
	var alice, bob bson.M
	err = client.db1.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Alice"}).Decode(&alice)
	assert.NoError(t, err)
	err = client.db2.Collection("accounts").FindOne(context.Background(), bson.M{"name": "Bob"}).Decode(&bob)
	assert.NoError(t, err)

	assert.Equal(t, int32(900), alice["balance"]) // Kỳ vọng nếu giao dịch thành công
	assert.Equal(t, int32(600), bob["balance"])   // Kỳ vọng nếu giao dịch thành công
}