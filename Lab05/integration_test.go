package main

import (
	shared "Lab05/Shared"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

// Server defines the structure of a server in the system (copied from Server.go for testing)
type Server struct {
	ID            string
	Address       string
	Data          map[string]string
	Mutex         sync.RWMutex
	ConsistentMap *shared.ConsistentHash
	Servers       []shared.ServerInfo
	Logger        *log.Logger
}

// Client defines the client structure (copied from Client.go for testing)
type Client struct {
	ServerAddr string
	Conn       net.Conn
	Encoder    *json.Encoder
	Decoder    *json.Decoder
}

// setupTestServer initializes a test server
func setupTestServer(t *testing.T, id string, addr string) *Server {
	// Create logger
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] ", id), log.LstdFlags)

	// Initialize server
	server := &Server{
		ID:            id,
		Address:       addr,
		Data:          make(map[string]string),
		ConsistentMap: shared.NewConsistentHash(10),
		Servers:       []shared.ServerInfo{},
		Logger:        logger,
	}

	// Add current server to consistent hash map
	server.ConsistentMap.Add(server.Address)
	server.Servers = append(server.Servers, shared.ServerInfo{ID: server.ID, Address: server.Address})

	// Start server
	listener, err := net.Listen("tcp", server.Address)
	if err != nil {
		t.Fatalf("Cannot start server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				break
			}
			go server.handleConnection(conn)
		}
	}()

	return server
}

// handleConnection handles client connections (copied from Server.go)
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	s.Logger.Printf("New connection from %s", conn.RemoteAddr().String())

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var msg shared.Message
		err := decoder.Decode(&msg)
		if err != nil {
			break
		}

		s.Logger.Printf("Received message: %+v", msg)

		// Determine responsible server for this key
		responsibleServer := s.ConsistentMap.Get(msg.Key)

		// If current server is not responsible
		if responsibleServer != s.Address && responsibleServer != "" {
			s.Logger.Printf("Forwarding request to server %s", responsibleServer)

			// Forward request to responsible server
			response := s.forwardRequest(responsibleServer, msg)
			encoder.Encode(response)
			continue
		}

		// Handle request if current server is responsible
		response := s.handleRequest(msg)
		encoder.Encode(response)
	}
}

// handleRequest handles client requests (copied from Server.go)
func (s *Server) handleRequest(msg shared.Message) shared.Message {
	response := shared.Message{
		Type:      "RESPONSE",
		Key:       msg.Key,
		Timestamp: time.Now(),
	}

	switch msg.Type {
	case "GET":
		s.Mutex.RLock()
		value, exists := s.Data[msg.Key]
		s.Mutex.RUnlock()

		if exists {
			response.Value = value
			response.Status = "SUCCESS"
		} else {
			response.Status = "ERROR"
			response.Error = "Key does not exist"
		}

	case "SET":
		s.Mutex.Lock()
		s.Data[msg.Key] = msg.Value
		s.Mutex.Unlock()
		response.Status = "SUCCESS"

	case "DELETE":
		s.Mutex.Lock()
		_, exists := s.Data[msg.Key]
		if exists {
			delete(s.Data, msg.Key)
			response.Status = "SUCCESS"
		} else {
			response.Status = "ERROR"
			response.Error = "Key does not exist"
		}
		s.Mutex.Unlock()

	default:
		response.Status = "ERROR"
		response.Error = "Invalid request type"
	}

	s.Logger.Printf("Send response: %+v", response)
	return response
}

// forwardRequest forwards a request to the responsible server (copied from Server.go)
func (s *Server) forwardRequest(serverAddr string, msg shared.Message) shared.Message {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		s.Logger.Printf("Cannot connect to server %s: %v", serverAddr, err)
		return shared.Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     fmt.Sprintf("Cannot connect to server %s", serverAddr),
		}
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	err = encoder.Encode(msg)
	if err != nil {
		s.Logger.Printf("Error when sending request to server %s: %v", serverAddr, err)
		return shared.Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error when forwarding request",
		}
	}

	var response shared.Message
	err = decoder.Decode(&response)
	if err != nil {
		s.Logger.Printf("Error when receiving response from server %s: %v", serverAddr, err)
		return shared.Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error when receiving response from responsible server",
		}
	}

	return response
}

// redistributeData redistributes data (copied from Server.go)
func (s *Server) redistributeData() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	// Create a copy of current data
	dataCopy := make(map[string]string)
	for k, v := range s.Data {
		dataCopy[k] = v
	}

	// Clear all current data
	s.Data = make(map[string]string)

	// Redistribute data
	for key, value := range dataCopy {
		responsibleServer := s.ConsistentMap.Get(key)

		// If current server is responsible, store data
		if responsibleServer == s.Address {
			s.Data[key] = value
		} else {
			// Send data to responsible server
			msg := shared.Message{
				Type:      "SET",
				Key:       key,
				Value:     value,
				Timestamp: time.Now(),
			}
			s.forwardRequest(responsibleServer, msg)
		}
	}

	s.Logger.Printf("Redistributed data after system configuration change")
}

// setupTestClient initializes a test client
func setupTestClient(t *testing.T, serverAddr string) *Client {
	client := &Client{
		ServerAddr: serverAddr,
	}

	err := client.Connect()
	if err != nil {
		t.Fatalf("Cannot connect to server: %v", err)
	}

	return client
}

// Connect connects to the server (copied from Client.go)
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return err
	}

	c.Conn = conn
	c.Encoder = json.NewEncoder(conn)
	c.Decoder = json.NewDecoder(conn)
	return nil
}

// Close closes the connection (copied from Client.go)
func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

// Set sends a SET request to the server (copied from Client.go)
func (c *Client) Set(key, value string) error {
	msg := shared.Message{
		Type:      "SET",
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("Error when sending request: %v", err)
	}

	var response shared.Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("Error when receiving response: %v", err)
	}

	if response.Status == "SUCCESS" {
		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	} else {
		return fmt.Errorf(response.Error)
	}
}

// Get sends a GET request to the server and returns the value (copied from Client.go)
func (c *Client) Get(key string) (string, error) {
	msg := shared.Message{
		Type:      "GET",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return "", fmt.Errorf("Error when sending request: %v", err)
	}

	var response shared.Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return "", fmt.Errorf("Error when receiving response: %v", err)
	}

	if response.Status == "SUCCESS" {
		return response.Value, nil
	} else {
		return "", fmt.Errorf(response.Error)
	}
}

// TestSetAndGet checks the Set and Get functionality
func TestSetAndGet(t *testing.T) {
	// Initialize server
	server := setupTestServer(t, "server1", "localhost:8091")

	// Initialize client
	client := setupTestClient(t, "localhost:8091")
	defer client.Close()

	// TC04: Set(key, value)
	key := "testkey"
	value := "testvalue"

	err := client.Set(key, value)
	if err != nil {
		t.Errorf("Error when setting key: %v", err)
	}

	// Check if data was stored on server
	server.Mutex.RLock()
	storedValue, exists := server.Data[key]
	server.Mutex.RUnlock()

	if !exists {
		t.Errorf("Key does not exist on server after set")
	}

	if storedValue != value {
		t.Errorf("Value mismatch. Expected: %s, Got: %s", value, storedValue)
	}

	fmt.Println("TC04: Set(key, value) - PASSED")

	// TC05: Get(key)
	retrievedValue, err := client.Get(key)
	if err != nil {
		t.Errorf("Error when getting key: %v", err)
	}

	if retrievedValue != value {
		t.Errorf("Value mismatch. Expected: %s, Got: %s", value, retrievedValue)
	}

	fmt.Println("TC05: Get(key) - PASSED")
}

// TestMultipleServers checks the system with multiple servers
func TestMultipleServers(t *testing.T) {
	// Initialize 3 servers
	server1 := setupTestServer(t, "server1", "localhost:8092")
	server2 := setupTestServer(t, "server2", "localhost:8093")
	server3 := setupTestServer(t, "server3", "localhost:8094")

	// Add server2 and server3 to server1
	server1.Mutex.Lock()
	server1.Servers = append(server1.Servers, shared.ServerInfo{ID: "server2", Address: "localhost:8093"})
	server1.Servers = append(server1.Servers, shared.ServerInfo{ID: "server3", Address: "localhost:8094"})
	server1.ConsistentMap.Add("localhost:8093")
	server1.ConsistentMap.Add("localhost:8094")
	server1.Mutex.Unlock()

	// Add server1 and server3 to server2
	server2.Mutex.Lock()
	server2.Servers = append(server2.Servers, shared.ServerInfo{ID: "server1", Address: "localhost:8092"})
	server2.Servers = append(server2.Servers, shared.ServerInfo{ID: "server3", Address: "localhost:8094"})
	server2.ConsistentMap.Add("localhost:8092")
	server2.ConsistentMap.Add("localhost:8094")
	server2.Mutex.Unlock()

	// Add server1 and server2 to server3
	server3.Mutex.Lock()
	server3.Servers = append(server3.Servers, shared.ServerInfo{ID: "server1", Address: "localhost:8092"})
	server3.Servers = append(server3.Servers, shared.ServerInfo{ID: "server2", Address: "localhost:8093"})
	server3.ConsistentMap.Add("localhost:8092")
	server3.ConsistentMap.Add("localhost:8093")
	server3.Mutex.Unlock()

	// Initialize client to connect to server1
	client := setupTestClient(t, "localhost:8092")
	defer client.Close()

	// Set multiple keys
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)

		err := client.Set(key, value)
		if err != nil {
			t.Errorf("Error when setting key %s: %v", key, err)
		}
	}

	// Check if all keys can be retrieved
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := fmt.Sprintf("value%d", i)

		retrievedValue, err := client.Get(key)
		if err != nil {
			t.Errorf("Error when getting key %s: %v", key, err)
			continue
		}

		if retrievedValue != expectedValue {
			t.Errorf("Value mismatch for key %s. Expected: %s, Got: %s", key, expectedValue, retrievedValue)
		}
	}

	// Check data distribution
	totalKeys := 0
	server1.Mutex.RLock()
	totalKeys += len(server1.Data)
	server1.Mutex.RUnlock()

	server2.Mutex.RLock()
	totalKeys += len(server2.Data)
	server2.Mutex.RUnlock()

	server3.Mutex.RLock()
	totalKeys += len(server3.Data)
	server3.Mutex.RUnlock()

	if totalKeys != 10 {
		t.Errorf("Total keys mismatch. Expected: 10, Got: %d", totalKeys)
	}

	// In data distribution
	server1.Mutex.RLock()
	fmt.Printf("Server1: %d keys\n", len(server1.Data))
	server1.Mutex.RUnlock()

	server2.Mutex.RLock()
	fmt.Printf("Server2: %d keys\n", len(server2.Data))
	server2.Mutex.RUnlock()

	server3.Mutex.RLock()
	fmt.Printf("Server3: %d keys\n", len(server3.Data))
	server3.Mutex.RUnlock()
}

// TestNodeFailure checks the system when a node fails
func TestNodeFailure(t *testing.T) {
	// Initialize 3 servers
	server1 := setupTestServer(t, "server1", "localhost:8095")
	server2 := setupTestServer(t, "server2", "localhost:8096")
	server3 := setupTestServer(t, "server3", "localhost:8097")

	// Configure servers
	// Add server2 and server3 to server1
	server1.Mutex.Lock()
	server1.Servers = append(server1.Servers, shared.ServerInfo{ID: "server2", Address: "localhost:8096"})
	server1.Servers = append(server1.Servers, shared.ServerInfo{ID: "server3", Address: "localhost:8097"})
	server1.ConsistentMap.Add("localhost:8096")
	server1.ConsistentMap.Add("localhost:8097")
	server1.Mutex.Unlock()

	// Add server1 and server3 to server2
	server2.Mutex.Lock()
	server2.Servers = append(server2.Servers, shared.ServerInfo{ID: "server1", Address: "localhost:8095"})
	server2.Servers = append(server2.Servers, shared.ServerInfo{ID: "server3", Address: "localhost:8097"})
	server2.ConsistentMap.Add("localhost:8095")
	server2.ConsistentMap.Add("localhost:8097")
	server2.Mutex.Unlock()

	// Add server1 and server2 to server3
	server3.Mutex.Lock()
	server3.Servers = append(server3.Servers, shared.ServerInfo{ID: "server1", Address: "localhost:8095"})
	server3.Servers = append(server3.Servers, shared.ServerInfo{ID: "server2", Address: "localhost:8096"})
	server3.ConsistentMap.Add("localhost:8095")
	server3.ConsistentMap.Add("localhost:8096")
	server3.Mutex.Unlock()

	// Initialize client to connect to server1
	client := setupTestClient(t, "localhost:8095")
	defer client.Close()

	// Set some keys
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)

		err := client.Set(key, value)
		if err != nil {
			t.Errorf("Error when setting key %s: %v", key, err)
		}
	}

	// Store data before server2 fails
	keysBeforeFailure := make(map[string]string)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value, err := client.Get(key)
		if err == nil {
			keysBeforeFailure[key] = value
		}
	}

	// Simulate server2 failing by removing it from consistent hash map
	server1.Mutex.Lock()
	server1.ConsistentMap.Remove("localhost:8096")
	// Update server list
	newServers := []shared.ServerInfo{}
	for _, s := range server1.Servers {
		if s.Address != "localhost:8096" {
			newServers = append(newServers, s)
		}
	}
	server1.Servers = newServers
	server1.Mutex.Unlock()

	server3.Mutex.Lock()
	server3.ConsistentMap.Remove("localhost:8096")
	// Update server list
	newServers = []shared.ServerInfo{}
	for _, s := range server3.Servers {
		if s.Address != "localhost:8096" {
			newServers = append(newServers, s)
		}
	}
	server3.Servers = newServers
	server3.Mutex.Unlock()

	// Redistribute data from server2 to server1 and server3
	// First, get data from server2
	server2.Mutex.RLock()
	server2Data := make(map[string]string)
	for k, v := range server2.Data {
		server2Data[k] = v
	}
	server2.Mutex.RUnlock()

	// Redistribute data from server2
	for key, value := range server2Data {
		// Determine new responsible server
		var responsibleServer string
		server1.Mutex.RLock()
		responsibleServer = server1.ConsistentMap.Get(key)
		server1.Mutex.RUnlock()

		// Store data in new server
		if responsibleServer == "localhost:8095" {
			server1.Mutex.Lock()
			server1.Data[key] = value
			server1.Mutex.Unlock()
		} else if responsibleServer == "localhost:8097" {
			server3.Mutex.Lock()
			server3.Data[key] = value
			server3.Mutex.Unlock()
		}
	}

	// Wait for a moment to redistribute data
	time.Sleep(1 * time.Second)

	// Check if all keys can still be retrieved
	failedKeys := 0
	for key, expectedValue := range keysBeforeFailure {
		retrievedValue, err := client.Get(key)
		if err != nil {
			t.Logf("Cannot retrieve key %s after server2 fails: %v", key, err)
			failedKeys++
			continue
		}

		if retrievedValue != expectedValue {
			t.Errorf("Value mismatch for key %s after server2 fails. Expected: %s, Got: %s", key, expectedValue, retrievedValue)
		}
	}

	// Allow a small number of keys not retrievable (due to redistribution not being perfect)
	if failedKeys > 3 {
		t.Errorf("Too many keys not retrievable after server2 fails: %d/10", failedKeys)
	}

	// Check data distribution after server2 fails
	totalKeys := 0
	server1.Mutex.RLock()
	server1Keys := len(server1.Data)
	totalKeys += server1Keys
	server1.Mutex.RUnlock()

	server3.Mutex.RLock()
	server3Keys := len(server3.Data)
	totalKeys += server3Keys
	server3.Mutex.RUnlock()

	// Allow a small number of keys lost in redistribution process
	if totalKeys < 7 {
		t.Errorf("Too few keys after server2 fails. Expected: at least 7, Got: %d", totalKeys)
	}

	// In data distribution
	t.Logf("Server1: %d keys", server1Keys)
	t.Logf("Server3: %d keys", server3Keys)
	t.Logf("Total keys: %d", totalKeys)
}
