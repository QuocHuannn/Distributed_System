package main

import (
	"log"
)

func main() {
	// Connect to two MongoDB instances
	client := NewMongoDBClient("mongodb://localhost:27017", "mongodb://localhost:27018")

	// Chuyển tiền từ "Alice" (MongoDB1) sang "Bob" (MongoDB2)
	err := TransferMoney(client, "Alice", "Bob", 100)
	if err != nil {
		log.Fatalf("Transaction failed: %v", err)
	}
}