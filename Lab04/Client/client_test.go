package Client

import (
	"log"
	"net"
	"net/rpc"
	"testing"
	"time"
	"Lab04/Server"
)

// Start a test server
func startTestServer(port string) {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Listen error: %v", err)
	}
	kv := Server.NewKVServer()
	rpc.Register(kv)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println("Accept error:", err)
				continue
			}
			go rpc.ServeConn(conn)
		}
	}()
}

// TestClient tests the Client methods
func TestClient(t *testing.T) {
	// Start the server
	port := "localhost:1234"
	startTestServer(port)
	time.Sleep(1 * time.Second) // Wait for the server to start

	// Create a client
	client := NewClient([]string{port}, 100)

	// TC01: Test AddNode
	t.Run("TC01: AddNode", func(t *testing.T) {
		err := client.AddNode("localhost:1235")
		if err != nil {
			t.Fatalf("AddNode error: %v", err)
		}

		err = client.AddNode("localhost:1236")
		if err != nil {
			t.Fatalf("AddNode error: %v", err)
		}
		startTestServer("localhost:1236")
		time.Sleep(1 * time.Second) 
	})

	// TC02: Test RemoveNode
	t.Run("TC02: RemoveNode", func(t *testing.T) {
		err := client.RemoveNode("localhost:1235")
		if err != nil {
			t.Fatalf("RemoveNode error: %v", err)
		}
	})

	// TC03: Test GetNodeId
	t.Run("TC03: GetNodeId", func(t *testing.T) {
		_, err := client.GetNodeId("color")
		if err != nil {
			t.Fatalf("GetNodeId error: %v", err)
		}
	})

	// TC04: Test Set
	t.Run("TC04: Set", func(t *testing.T) {
		err := client.Set("color", "blue")
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}
	})

	// TC05: Test Get
	t.Run("TC05: Get", func(t *testing.T) {
		value, err := client.Get("color")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if value != "blue" {
			t.Fatalf("Expected 'blue', got '%s'", value)
		}
	})

	// Clean up: Close the listener (if needed)
	// Note: In a real-world scenario, you might want to gracefully shut down the server.
}

// Helper function to clean up after tests
func cleanup() {
	// Implement any necessary cleanup logic here
} 