package main

import (
	"fmt"
	"log"
	"time"
	"Lab04/Client"
	"Lab04/Server"
)

func main() {
	// Start servers
	go Server.StartServer("localhost:1234")
	go Server.StartServer("localhost:1235")

	// Wait for servers to start
	fmt.Println("Waiting for servers to start...")
	time.Sleep(2 * time.Second)

	// Create client
	client := Client.NewClient([]string{"localhost:1234", "localhost:1235"}, 100)

	// TC01: Test AddNode
	log.Println("=== TC01: AddNode ===")
	err := client.AddNode("localhost:1236")
	if err != nil {
		log.Fatalf("AddNode error: %v", err)
	}
	log.Println("Added node localhost:1236 successfully")

	// TC02: Test RemoveNode
	log.Println("=== TC02: RemoveNode ===")
	err = client.RemoveNode("localhost:1236")
	if err != nil {
		log.Fatalf("RemoveNode error: %v", err)
	}
	log.Println("Removed node localhost:1236 successfully")

	// TC03: Test GetNodeId
	log.Println("=== TC03: GetNodeId ===")
	serverAddr, err := client.GetNodeId("color")
	if err != nil {
		log.Fatalf("GetNodeId error: %v", err)
	}
	log.Printf("Node responsible for 'color': %s", serverAddr)

	// TC04: Test Set
	log.Println("=== TC04: Set ===")
	err = client.Set("color", "blue")
	if err != nil {
		log.Fatalf("Set error: %v", err)
	}
	log.Println("Set 'color' to 'blue' successfully")

	// TC05: Test Get
	log.Println("=== TC05: Get ===")
	value, err := client.Get("color")
	if err != nil {
		log.Fatalf("Get error: %v", err)
	}
	log.Printf("Value for 'color': %s", value)
}