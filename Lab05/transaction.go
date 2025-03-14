package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// TransactionLog đại diện cho một bản ghi log giao dịch để hỗ trợ rollback thủ công
type TransactionLog struct {
	ID         string    `bson:"_id"`
	FromUser   string    `bson:"from_user"`
	ToUser     string    `bson:"to_user"`
	Amount     int32     `bson:"amount"`
	Phase      string    `bson:"phase"` // "prepared" hoặc "committed"
	Timestamp  time.Time `bson:"timestamp"`
	Instance   string    `bson:"instance"` // "MongoDB1" hoặc "MongoDB2"
}

// TransferMoney thực hiện giao dịch chuyển tiền với 2PC và xử lý trạng thái không nhất quán
func TransferMoney(client *MongoDBClient, fromUser string, toUser string, amount int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Tạo ID giao dịch duy nhất
	transactionID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Bước 1: Kiểm tra điều kiện trước khi giao dịch
	// Kiểm tra người gửi
	var fromAccount bson.M
	err := client.db1.Collection("accounts").FindOne(ctx, bson.M{"name": fromUser}).Decode(&fromAccount)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("người gửi %s không tồn tại", fromUser)
		}
		return fmt.Errorf("lỗi kiểm tra người gửi: %v", err)
	}
	// kiểm tra số dư
	balance, ok := fromAccount["balance"].(int32)
	if !ok {
		return fmt.Errorf("kiểu dữ liệu số dư của %s không hợp lệ", fromUser)
	}
	if balance < int32(amount) {
		return fmt.Errorf("số dư của %s không đủ (%d < %d)", fromUser, balance, amount)
	}
	// kiểm tra người nhận
	var toAccount bson.M
	err = client.db2.Collection("accounts").FindOne(ctx, bson.M{"name": toUser}).Decode(&toAccount)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("người nhận %s không tồn tại", toUser)
		}
		return fmt.Errorf("lỗi kiểm tra người nhận: %v", err)
	}

	// Bước 2: Khởi tạo session
	session1, err := client.client1.StartSession()
	if err != nil {
		return fmt.Errorf("không thể tạo session trên MongoDB1: %v", err)
	}
	defer session1.EndSession(ctx)

	session2, err := client.client2.StartSession()
	if err != nil {
		return fmt.Errorf("không thể tạo session trên MongoDB2: %v", err)
	}
	defer session2.EndSession(ctx)

	// Giai đoạn Prepare
	err = session1.StartTransaction()
	if err != nil {
		return fmt.Errorf("không thể bắt đầu giao dịch trên MongoDB1: %v", err)
	}

	err = session2.StartTransaction()
	if err != nil {
		session1.AbortTransaction(ctx)
		return fmt.Errorf("không thể bắt đầu giao dịch trên MongoDB2: %v", err)
	}

	// Prepare trên MongoDB1
	err = mongo.WithSession(ctx, session1, func(sc mongo.SessionContext) error {
		// Ghi log trạng thái Prepare
		_, err := client.db1.Collection("transaction_logs").InsertOne(sc, TransactionLog{
			ID:        transactionID,
			FromUser:  fromUser,
			ToUser:    toUser,
			Amount:    int32(amount),
			Phase:     "prepared",
			Timestamp: time.Now(),
			Instance:  "MongoDB1",
		})
		if err != nil {
			return fmt.Errorf("ghi log Prepare thất bại: %v", err)
		}

		result, err := client.db1.Collection("accounts").UpdateOne(
			sc,
			bson.M{"name": fromUser, "balance": bson.M{"$gte": int32(amount)}},
			bson.M{"$inc": bson.M{"balance": -int32(amount)}},
		)
		if err != nil {
			return err
		}
		if result.MatchedCount == 0 {
			return fmt.Errorf("không tìm thấy tài khoản %s hoặc số dư không đủ", fromUser)
		}
		return nil
	})
	if err != nil {
		session1.AbortTransaction(ctx)
		session2.AbortTransaction(ctx)
		return fmt.Errorf("giai đoạn Prepare thất bại trên MongoDB1: %v", err)
	}

	// Prepare trên MongoDB2
	err = mongo.WithSession(ctx, session2, func(sc mongo.SessionContext) error {
		// Ghi log trạng thái Prepare
		_, err := client.db2.Collection("transaction_logs").InsertOne(sc, TransactionLog{
			ID:        transactionID,
			FromUser:  fromUser,
			ToUser:    toUser,
			Amount:    int32(amount),
			Phase:     "prepared",
			Timestamp: time.Now(),
			Instance:  "MongoDB2",
		})
		if err != nil {
			return fmt.Errorf("ghi log Prepare thất bại: %v", err)
		}

		result, err := client.db2.Collection("accounts").UpdateOne(
			sc,
			bson.M{"name": toUser},
			bson.M{"$inc": bson.M{"balance": int32(amount)}},
		)
		if err != nil {
			return err
		}
		if result.MatchedCount == 0 {
			return fmt.Errorf("không tìm thấy tài khoản %s", toUser)
		}
		return nil
	})
	if err != nil {
		session1.AbortTransaction(ctx)
		session2.AbortTransaction(ctx)
		return fmt.Errorf("giai đoạn Prepare thất bại trên MongoDB2: %v", err)
	}

	// Giai đoạn Commit
	err = session1.CommitTransaction(ctx)
	if err != nil {
		session2.AbortTransaction(ctx)
		return fmt.Errorf("giai đoạn Commit thất bại trên MongoDB1, rollback MongoDB2: %v", err)
	}

	// Cập nhật log sau khi commit MongoDB1
	_, err = client.db1.Collection("transaction_logs").UpdateOne(
		ctx,
		bson.M{"_id": transactionID, "instance": "MongoDB1"},
		bson.M{"$set": bson.M{"phase": "committed"}},
	)
	if err != nil {
		log.Printf("Cảnh báo: Không thể cập nhật log commit trên MongoDB1: %v", err)
	}

	err = session2.CommitTransaction(ctx)
	if err != nil {
		// MongoDB1 đã commit, rollback không khả thi, thực hiện compensating transaction
		log.Printf("Lỗi: Commit thất bại trên MongoDB2 sau khi MongoDB1 đã commit: %v", err)
		err = compensateMongoDB1(ctx, client.db1, transactionID, fromUser, amount)
		if err != nil {
			return fmt.Errorf("trạng thái không nhất quán và không thể hoàn tác trên MongoDB1: %v", err)
		}
		return fmt.Errorf("giao dịch thất bại trên MongoDB2, đã hoàn tác trên MongoDB1")
	}

	// Cập nhật log sau khi commit MongoDB2
	_, err = client.db2.Collection("transaction_logs").UpdateOne(
		ctx,
		bson.M{"_id": transactionID, "instance": "MongoDB2"},
		bson.M{"$set": bson.M{"phase": "committed"}},
	)
	if err != nil {
		log.Printf("Cảnh báo: Không thể cập nhật log commit trên MongoDB2: %v", err)
	}

	log.Println("Giao dịch hoàn tất thành công!")
	return nil
}

// compensateMongoDB1 thực hiện giao dịch bù để hoàn tác thay đổi trên MongoDB1
func compensateMongoDB1(ctx context.Context, db *mongo.Database, transactionID, fromUser string, amount int) error {
	result, err := db.Collection("accounts").UpdateOne(
		ctx,
		bson.M{"name": fromUser},
		bson.M{"$inc": bson.M{"balance": int32(amount)}},
	)
	if err != nil {
		return fmt.Errorf("không thể hoàn tác trên MongoDB1: %v", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("không tìm thấy tài khoản %s để hoàn tác", fromUser)
	}

	// Ghi log trạng thái hoàn tác
	_, err = db.Collection("transaction_logs").UpdateOne(
		ctx,
		bson.M{"_id": transactionID, "instance": "MongoDB1"},
		bson.M{"$set": bson.M{"phase": "compensated"}},
	)
	if err != nil {
		log.Printf("Cảnh báo: Không thể cập nhật log hoàn tác trên MongoDB1: %v", err)
	}

	log.Printf("Đã hoàn tác giao dịch %s trên MongoDB1", transactionID)
	return nil
}