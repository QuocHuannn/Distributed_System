package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBClient struct {
	client1 *mongo.Client
	client2 *mongo.Client
	db1     *mongo.Database
	db2     *mongo.Database
}

func NewMongoDBClient(uri1, uri2 string) *MongoDBClient {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client1, err := mongo.Connect(ctx, options.Client().ApplyURI(uri1))
	if err != nil {
		log.Fatalf("Error connecting to MongoDB1: %v", err)
	}

	client2, err := mongo.Connect(ctx, options.Client().ApplyURI(uri2))
	if err != nil {
		log.Fatalf("Error connecting to MongoDB2: %v", err)
	}

	return &MongoDBClient{
		client1: client1,
		client2: client2,
		db1:     client1.Database("bank1"),
		db2:     client2.Database("bank2"),
	}
}