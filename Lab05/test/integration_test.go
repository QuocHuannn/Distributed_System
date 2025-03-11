package test

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Define necessary data structures for testing
type TransactionStatus string

const (
	PREPARED    TransactionStatus = "PREPARED"
	COMMITTED   TransactionStatus = "COMMITTED"
	ABORTED     TransactionStatus = "ABORTED"
	IN_PROGRESS TransactionStatus = "IN_PROGRESS"
)

type Operation struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type Message struct {
	Type       string      `json:"type"`
	TxID       string      `json:"tx_id,omitempty"`
	Key        string      `json:"key,omitempty"`
	Value      string      `json:"value,omitempty"`
	Status     string      `json:"status,omitempty"`
	Error      string      `json:"error,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
	Operations []Operation `json:"operations,omitempty"`
	Resources  []string    `json:"resources,omitempty"`
}

// TestClient is a simple client for testing
type TestClient struct {
	CoordinatorAddr string
	Conn            net.Conn
	Encoder         *json.Encoder
	Decoder         *json.Decoder
	t               *testing.T
}

// NewTestClient creates a new test client
func NewTestClient(coordinatorAddr string, t *testing.T) *TestClient {
	return &TestClient{
		CoordinatorAddr: coordinatorAddr,
		t:               t,
	}
}

// Connect connects to the coordinator
func (c *TestClient) Connect() error {
	conn, err := net.Dial("tcp", c.CoordinatorAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator: %v", err)
	}

	c.Conn = conn
	c.Encoder = json.NewEncoder(conn)
	c.Decoder = json.NewDecoder(conn)

	return nil
}

// Close closes the connection
func (c *TestClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

// TransferMoney transfers money between two accounts
func (c *TestClient) TransferMoney(fromAccount, toAccount string, amount float64) error {
	operations := []Operation{
		{
			Type:  "DEBIT",
			Key:   fromAccount,
			Value: fmt.Sprintf("%.2f", amount),
		},
		{
			Type:  "CREDIT",
			Key:   toAccount,
			Value: fmt.Sprintf("%.2f", amount),
		},
	}

	msg := Message{
		Type:       "TRANSFER_MONEY",
		Operations: operations,
		Timestamp:  time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to send transfer request: %v", err)
	}

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return fmt.Errorf("transfer failed: %s", response.Error)
	}

	return nil
}

// BookTrip books a trip with car, hotel, and flight
func (c *TestClient) BookTrip(carType string, carDays int, hotelType string, hotelNights int, flightClass string, flightSeats int) error {
	operations := []Operation{
		{
			Type:  "SET",
			Key:   "Car",
			Value: fmt.Sprintf("%s:%d", carType, carDays),
		},
		{
			Type:  "SET",
			Key:   "HotelRoom",
			Value: fmt.Sprintf("%s:%d", hotelType, hotelNights),
		},
		{
			Type:  "SET",
			Key:   "Flight",
			Value: fmt.Sprintf("%s:%d", flightClass, flightSeats),
		},
	}

	msg := Message{
		Type:       "BOOK_TRIP",
		Operations: operations,
		Timestamp:  time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to send booking request: %v", err)
	}

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return fmt.Errorf("booking failed: %s", response.Error)
	}

	return nil
}

// Get retrieves a value for a key
func (c *TestClient) Get(key string) (string, error) {
	log.Printf("Client sending GET request for key: %s", key)
	msg := Message{
		Type:      "GET",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return "", fmt.Errorf("failed to send GET request: %v", err)
	}
	log.Printf("GET request sent for key: %s", key)

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return "", fmt.Errorf("failed to receive response: %v", err)
	}
	log.Printf("Received response for GET %s: %+v", key, response)

	if response.Status != "OK" {
		return "", fmt.Errorf("GET failed: %s", response.Error)
	}

	return response.Value, nil
}

// Set sets a value for a key
func (c *TestClient) Set(key, value string) error {
	log.Printf("Client sending SET request for key: %s, value: %s", key, value)
	msg := Message{
		Type:      "SET",
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to send SET request: %v", err)
	}
	log.Printf("SET request sent for key: %s", key)

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}
	log.Printf("Received response for SET %s: %+v", key, response)

	if response.Status != "OK" {
		return fmt.Errorf("SET failed: %s", response.Error)
	}

	return nil
}

// waitForServers waits for all servers to be ready
func waitForServers(t *testing.T, addresses []string) {
	log.Println("Checking if all servers are ready...")

	for _, addr := range addresses {
		ready := false
		for i := 0; i < 20; i++ { // Increase retry count to 20
			log.Printf("Attempt %d to connect to %s", i+1, addr)
			conn, err := net.DialTimeout("tcp", addr, 1*time.Second) // Increase timeout to 1 second
			if err == nil {
				conn.Close()
				ready = true
				log.Printf("Server %s is ready", addr)
				break
			}
			log.Printf("Server %s not ready yet, retrying... (%v)", addr, err)
			time.Sleep(1 * time.Second) // Increase wait time to 1 second
		}

		if !ready {
			t.Fatalf("Server %s did not start properly after multiple attempts", addr)
		}
	}

	log.Println("All servers are ready")
	// Add an additional wait time after confirming all servers are ready
	log.Println("Waiting an additional 2 seconds for system stabilization...")
	time.Sleep(2 * time.Second)
}

// TestSystem tests the entire system
func TestSystem(t *testing.T) {
	// Start processes
	log.Println("Starting coordinator...")
	coordinatorCmd := startCoordinator(t)
	defer coordinatorCmd.Process.Kill()

	// Start servers - coordinator already has a 2 second delay built in
	log.Println("Starting server 1...")
	server1Cmd := startServer(t, "localhost:8001", "localhost:8000")
	defer server1Cmd.Process.Kill()

	log.Println("Starting server 2...")
	server2Cmd := startServer(t, "localhost:8002", "localhost:8000")
	defer server2Cmd.Process.Kill()

	log.Println("Starting server 3...")
	server3Cmd := startServer(t, "localhost:8003", "localhost:8000")
	defer server3Cmd.Process.Kill()

	// Wait for all servers to be ready
	waitForServers(t, []string{
		"localhost:8000",
		"localhost:8001",
		"localhost:8002",
		"localhost:8003",
	})

	// Create client for testing
	log.Println("Creating test client...")
	client := NewTestClient("localhost:8000", t)
	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect to coordinator: %v", err)
	}
	defer client.Close()
	log.Println("Client connected successfully")

	// Test 1: Initialize accounts
	t.Run("InitializeAccounts", func(t *testing.T) {
		log.Println("Test 1: Initializing accounts...")
		err := client.Set("account1", "1000.00")
		if err != nil {
			t.Fatalf("Failed to initialize account1: %v", err)
		}
		log.Println("Set account1 = 1000.00")

		err = client.Set("account2", "500.00")
		if err != nil {
			t.Fatalf("Failed to initialize account2: %v", err)
		}
		log.Println("Set account2 = 500.00")

		// Check initial values
		val1, err := client.Get("account1")
		if err != nil {
			t.Fatalf("Failed to get account1: %v", err)
		}
		if val1 != "1000.00" {
			t.Errorf("Expected account1 = 1000.00, got %s", val1)
		}
		log.Println("Verified account1 =", val1)

		val2, err := client.Get("account2")
		if err != nil {
			t.Fatalf("Failed to get account2: %v", err)
		}
		if val2 != "500.00" {
			t.Errorf("Expected account2 = 500.00, got %s", val2)
		}
		log.Println("Verified account2 =", val2)
	})

	// Test 2: Transfer money
	t.Run("TransferMoney", func(t *testing.T) {
		err := client.TransferMoney("account1", "account2", 200.00)
		if err != nil {
			t.Fatalf("Failed to transfer money: %v", err)
		}

		// Check balances after transfer
		val1, err := client.Get("account1")
		if err != nil {
			t.Fatalf("Failed to get account1: %v", err)
		}
		balance1, _ := strconv.ParseFloat(val1, 64)
		if balance1 != 800.00 {
			t.Errorf("Expected account1 = 800.00, got %.2f", balance1)
		}

		val2, err := client.Get("account2")
		if err != nil {
			t.Fatalf("Failed to get account2: %v", err)
		}
		balance2, _ := strconv.ParseFloat(val2, 64)
		if balance2 != 700.00 {
			t.Errorf("Expected account2 = 700.00, got %.2f", balance2)
		}
	})

	// Test 3: Book a trip
	t.Run("BookTrip", func(t *testing.T) {
		err := client.BookTrip("SUV", 3, "Deluxe", 2, "Economy", 2)
		if err != nil {
			t.Fatalf("Failed to book trip: %v", err)
		}

		// Check booking information
		car, err := client.Get("Car")
		if err != nil {
			t.Fatalf("Failed to get car: %v", err)
		}
		if car != "SUV:3" {
			t.Errorf("Expected Car = SUV:3, got %s", car)
		}

		hotel, err := client.Get("HotelRoom")
		if err != nil {
			t.Fatalf("Failed to get hotel: %v", err)
		}
		if hotel != "Deluxe:2" {
			t.Errorf("Expected HotelRoom = Deluxe:2, got %s", hotel)
		}

		flight, err := client.Get("Flight")
		if err != nil {
			t.Fatalf("Failed to get flight: %v", err)
		}
		if flight != "Economy:2" {
			t.Errorf("Expected Flight = Economy:2, got %s", flight)
		}
	})

	// Test 4: Transfer money with insufficient funds (should fail)
	t.Run("TransferMoneyInsufficientFunds", func(t *testing.T) {
		err := client.TransferMoney("account1", "account2", 1000.00)
		if err == nil {
			t.Errorf("Expected transfer to fail due to insufficient funds, but it succeeded")
		}

		// Check balances remain unchanged
		val1, err := client.Get("account1")
		if err != nil {
			t.Fatalf("Failed to get account1: %v", err)
		}
		balance1, _ := strconv.ParseFloat(val1, 64)
		if balance1 != 800.00 {
			t.Errorf("Expected account1 = 800.00, got %.2f", balance1)
		}

		val2, err := client.Get("account2")
		if err != nil {
			t.Fatalf("Failed to get account2: %v", err)
		}
		balance2, _ := strconv.ParseFloat(val2, 64)
		if balance2 != 700.00 {
			t.Errorf("Expected account2 = 700.00, got %.2f", balance2)
		}
	})

	// Test 5: Concurrent transactions
	t.Run("ConcurrentTransactions", func(t *testing.T) {
		var wg sync.WaitGroup
		numTransactions := 5
		wg.Add(numTransactions)

		for i := 0; i < numTransactions; i++ {
			go func(idx int) {
				defer wg.Done()
				client := NewTestClient("localhost:8000", t)
				err := client.Connect()
				if err != nil {
					t.Errorf("Failed to connect to coordinator: %v", err)
					return
				}
				defer client.Close()

				err = client.Set(fmt.Sprintf("concurrent_key_%d", idx), fmt.Sprintf("value_%d", idx))
				if err != nil {
					t.Errorf("Failed to set concurrent key %d: %v", idx, err)
				}
			}(i)
		}

		wg.Wait()

		// Check that all keys were set
		for i := 0; i < numTransactions; i++ {
			val, err := client.Get(fmt.Sprintf("concurrent_key_%d", i))
			if err != nil {
				t.Errorf("Failed to get concurrent_key_%d: %v", i, err)
				continue
			}
			if val != fmt.Sprintf("value_%d", i) {
				t.Errorf("Expected concurrent_key_%d = value_%d, got %s", i, i, val)
			}
		}
	})
}

// startCoordinator starts the coordinator
func startCoordinator(t *testing.T) *exec.Cmd {
	// Use go run directly instead of building
	cmd := exec.Command("go", "run", "../Coordinator/coordinator.go", "--address", "localhost:8000", "--servers", "localhost:8001,localhost:8002,localhost:8003")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Starting coordinator with command:", cmd.String())
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start coordinator: %v", err)
	}

	// Give the coordinator time to initialize
	log.Println("Waiting for coordinator to initialize (2 seconds)...")
	time.Sleep(2 * time.Second)
	log.Println("Coordinator should be ready now")
	return cmd
}

// startServer starts a server
func startServer(t *testing.T, address, coordinatorAddr string) *exec.Cmd {
	cmd := exec.Command("go", "run", "../Server/server.go", "--address", address, "--coordinator", coordinatorAddr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Starting server with command:", cmd.String())
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start server at %s: %v", address, err)
	}

	// Give the server time to initialize
	log.Println("Waiting for server", address, "to initialize (1 second)...")
	time.Sleep(1 * time.Second)
	log.Println("Server", address, "should be ready now")
	return cmd
}

func TestMain(m *testing.M) {
	log.Println("Setting up tests...")

	// Kill any existing processes on test ports
	killExistingProcesses()

	// Run tests
	exitCode := m.Run()

	// Cleanup after tests
	log.Println("Cleaning up after tests...")

	// Kill any processes started by tests
	killExistingProcesses()

	os.Exit(exitCode)
}

// killExistingProcesses kills any processes using the test ports
func killExistingProcesses() {
	ports := []string{"8000", "8001", "8002", "8003"}

	for _, port := range ports {
		log.Printf("Checking for processes on port %s...", port)
		// Sử dụng cú pháp đúng cho Windows PowerShell
		killCmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-NetTCPConnection -LocalPort %s -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }", port))
		killCmd.Stdout = os.Stdout
		killCmd.Stderr = os.Stderr
		_ = killCmd.Run() // Ignore errors
	}

	// Wait a moment for ports to be released
	time.Sleep(1 * time.Second)
}
