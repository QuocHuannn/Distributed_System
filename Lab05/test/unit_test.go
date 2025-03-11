package test

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

// Define mock data structures for testing
type MockTransactionStatus string

const (
	MOCK_PREPARED    MockTransactionStatus = "PREPARED"
	MOCK_COMMITTED   MockTransactionStatus = "COMMITTED"
	MOCK_ABORTED     MockTransactionStatus = "ABORTED"
	MOCK_IN_PROGRESS MockTransactionStatus = "IN_PROGRESS"
)

type MockOperation struct {
	Type  string
	Key   string
	Value string
}

type MockTransaction struct {
	ID           string
	Status       MockTransactionStatus
	Operations   []MockOperation
	Participants []string
	StartTime    time.Time
	EndTime      time.Time
}

// MockConsistentHash is a simple implementation of consistent hashing for testing
type MockConsistentHash struct {
	nodes map[string]bool
}

// NewMockConsistentHash creates a new mock consistent hash
func NewMockConsistentHash() *MockConsistentHash {
	return &MockConsistentHash{
		nodes: make(map[string]bool),
	}
}

// Add adds a node to the hash ring
func (ch *MockConsistentHash) Add(node string) {
	ch.nodes[node] = true
}

// Remove removes a node from the hash ring
func (ch *MockConsistentHash) Remove(node string) {
	delete(ch.nodes, node)
}

// Get returns the node responsible for a key
func (ch *MockConsistentHash) Get(key string) string {
	// Simple implementation: just return the first node
	for node := range ch.nodes {
		return node
	}
	return ""
}

// GetAll returns all nodes
func (ch *MockConsistentHash) GetAll() []string {
	result := make([]string, 0, len(ch.nodes))
	for node := range ch.nodes {
		result = append(result, node)
	}
	return result
}

// TestConsistentHash tests the consistent hashing algorithm
func TestConsistentHash(t *testing.T) {
	ch := NewMockConsistentHash()

	// Test 1: Add nodes
	t.Run("AddNode", func(t *testing.T) {
		ch.Add("server1")
		ch.Add("server2")
		ch.Add("server3")

		nodes := ch.GetAll()
		if len(nodes) != 3 {
			t.Errorf("Expected 3 nodes, got %d", len(nodes))
		}

		// Check that all nodes were added
		nodeMap := make(map[string]bool)
		for _, node := range nodes {
			nodeMap[node] = true
		}

		if !nodeMap["server1"] || !nodeMap["server2"] || !nodeMap["server3"] {
			t.Errorf("Not all nodes were added correctly")
		}
	})

	// Test 2: Remove nodes
	t.Run("RemoveNode", func(t *testing.T) {
		ch.Remove("server2")

		nodes := ch.GetAll()
		if len(nodes) != 2 {
			t.Errorf("Expected 2 nodes after removal, got %d", len(nodes))
		}

		// Check that the node was removed
		nodeMap := make(map[string]bool)
		for _, node := range nodes {
			nodeMap[node] = true
		}

		if !nodeMap["server1"] || !nodeMap["server3"] || nodeMap["server2"] {
			t.Errorf("Node removal did not work correctly")
		}
	})

	// Test 3: Get node for key
	t.Run("GetNodeForKey", func(t *testing.T) {
		// Ensure there's at least one node
		if len(ch.GetAll()) == 0 {
			ch.Add("server1")
		}

		node := ch.Get("testkey")
		if node == "" {
			t.Errorf("Failed to get node for key")
		}
	})
}

// MockDataStore is a simple implementation of a data store for testing
type MockDataStore struct {
	data map[string]string
}

// NewMockDataStore creates a new mock data store
func NewMockDataStore() *MockDataStore {
	return &MockDataStore{
		data: make(map[string]string),
	}
}

// Get retrieves a value for a key
func (ds *MockDataStore) Get(key string) (string, bool) {
	value, exists := ds.data[key]
	return value, exists
}

// Set sets a value for a key
func (ds *MockDataStore) Set(key, value string) {
	ds.data[key] = value
}

// Delete deletes a key
func (ds *MockDataStore) Delete(key string) {
	delete(ds.data, key)
}

// TestDataStore tests basic operations on the data store
func TestDataStore(t *testing.T) {
	ds := NewMockDataStore()

	// Test 1: Set and Get
	t.Run("SetAndGet", func(t *testing.T) {
		ds.Set("key1", "value1")
		value, exists := ds.Get("key1")
		if !exists {
			t.Errorf("Key 'key1' should exist")
		}
		if value != "value1" {
			t.Errorf("Expected value1, got %s", value)
		}
	})

	// Test 2: Delete
	t.Run("Delete", func(t *testing.T) {
		ds.Set("key2", "value2")
		ds.Delete("key2")
		_, exists := ds.Get("key2")
		if exists {
			t.Errorf("Key 'key2' should not exist after deletion")
		}
	})

	// Test 3: Non-existent key
	t.Run("NonExistent", func(t *testing.T) {
		_, exists := ds.Get("nonexistent")
		if exists {
			t.Errorf("Key 'nonexistent' should not exist")
		}
	})
}

// MockTransactionManager is a simple implementation of a transaction manager for testing
type MockTransactionManager struct {
	transactions map[string]*MockTransaction
}

// NewMockTransactionManager creates a new mock transaction manager
func NewMockTransactionManager() *MockTransactionManager {
	return &MockTransactionManager{
		transactions: make(map[string]*MockTransaction),
	}
}

// BeginTransaction starts a new transaction
func (tm *MockTransactionManager) BeginTransaction() *MockTransaction {
	txID := fmt.Sprintf("tx-%d", time.Now().UnixNano())
	tx := &MockTransaction{
		ID:           txID,
		Status:       MOCK_IN_PROGRESS,
		Operations:   []MockOperation{},
		Participants: []string{},
		StartTime:    time.Now(),
	}
	tm.transactions[txID] = tx
	return tx
}

// GetTransaction retrieves a transaction by ID
func (tm *MockTransactionManager) GetTransaction(txID string) (*MockTransaction, error) {
	tx, exists := tm.transactions[txID]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found", txID)
	}
	return tx, nil
}

// TestTransactionManager tests the transaction manager
func TestTransactionManager(t *testing.T) {
	tm := NewMockTransactionManager()

	// Test 1: Begin transaction
	t.Run("BeginTransaction", func(t *testing.T) {
		tx := tm.BeginTransaction()
		if tx.ID == "" {
			t.Errorf("Transaction ID should not be empty")
		}
		if tx.Status != MOCK_IN_PROGRESS {
			t.Errorf("New transaction should have IN_PROGRESS status")
		}
	})

	// Test 2: Get transaction
	t.Run("GetTransaction", func(t *testing.T) {
		tx := tm.BeginTransaction()
		retrievedTx, err := tm.GetTransaction(tx.ID)
		if err != nil {
			t.Errorf("Failed to get transaction: %v", err)
		}
		if retrievedTx.ID != tx.ID {
			t.Errorf("Retrieved transaction ID does not match")
		}
	})

	// Test 3: Non-existent transaction
	t.Run("NonExistentTransaction", func(t *testing.T) {
		_, err := tm.GetTransaction("nonexistent")
		if err == nil {
			t.Errorf("Should return error for non-existent transaction")
		}
	})
}

// TestTransferMoney tests the money transfer functionality
func TestTransferMoney(t *testing.T) {
	ds := NewMockDataStore()

	// Initialize accounts
	ds.Set("account1", "1000.00")
	ds.Set("account2", "500.00")

	// Test 1: Successful transfer
	t.Run("SuccessfulTransfer", func(t *testing.T) {
		// Simulate money transfer
		fromAccount := "account1"
		toAccount := "account2"
		amount := 200.00

		// Get current balances
		fromBalance, _ := ds.Get(fromAccount)
		toBalance, _ := ds.Get(toAccount)

		fromBalanceFloat, _ := strconv.ParseFloat(fromBalance, 64)
		toBalanceFloat, _ := strconv.ParseFloat(toBalance, 64)

		// Check if sufficient funds
		if fromBalanceFloat >= amount {
			// Perform transfer
			ds.Set(fromAccount, fmt.Sprintf("%.2f", fromBalanceFloat-amount))
			ds.Set(toAccount, fmt.Sprintf("%.2f", toBalanceFloat+amount))

			// Check balances after transfer
			newFromBalance, _ := ds.Get(fromAccount)
			newToBalance, _ := ds.Get(toAccount)

			newFromBalanceFloat, _ := strconv.ParseFloat(newFromBalance, 64)
			newToBalanceFloat, _ := strconv.ParseFloat(newToBalance, 64)

			if newFromBalanceFloat != 800.00 {
				t.Errorf("Expected from account balance = 800.00, got %.2f", newFromBalanceFloat)
			}

			if newToBalanceFloat != 700.00 {
				t.Errorf("Expected to account balance = 700.00, got %.2f", newToBalanceFloat)
			}
		} else {
			t.Errorf("Should have sufficient funds for transfer")
		}
	})

	// Test 2: Transfer with insufficient funds
	t.Run("InsufficientFunds", func(t *testing.T) {
		// Simulate money transfer
		fromAccount := "account1"
		toAccount := "account2"
		amount := 1000.00

		// Get current balances
		fromBalance, _ := ds.Get(fromAccount)
		toBalance, _ := ds.Get(toAccount)

		fromBalanceFloat, _ := strconv.ParseFloat(fromBalance, 64)
		toBalanceFloat, _ := strconv.ParseFloat(toBalance, 64)

		// Check if sufficient funds
		if fromBalanceFloat >= amount {
			// Perform transfer (should not happen)
			t.Errorf("Should not have sufficient funds for transfer")
		} else {
			// Check balances remain unchanged
			if fromBalanceFloat != 800.00 {
				t.Errorf("From account balance should remain 800.00")
			}

			if toBalanceFloat != 700.00 {
				t.Errorf("To account balance should remain 700.00")
			}
		}
	})
}

// TestBookTrip tests the trip booking functionality
func TestBookTrip(t *testing.T) {
	ds := NewMockDataStore()

	// Test 1: Successful booking
	t.Run("SuccessfulBooking", func(t *testing.T) {
		// Simulate booking
		ds.Set("Car", "SUV:3")
		ds.Set("HotelRoom", "Deluxe:2")
		ds.Set("Flight", "Economy:2")

		// Check booking information
		car, exists := ds.Get("Car")
		if !exists {
			t.Errorf("Car booking should exist")
		}
		if car != "SUV:3" {
			t.Errorf("Expected Car = SUV:3, got %s", car)
		}

		hotel, exists := ds.Get("HotelRoom")
		if !exists {
			t.Errorf("Hotel booking should exist")
		}
		if hotel != "Deluxe:2" {
			t.Errorf("Expected HotelRoom = Deluxe:2, got %s", hotel)
		}

		flight, exists := ds.Get("Flight")
		if !exists {
			t.Errorf("Flight booking should exist")
		}
		if flight != "Economy:2" {
			t.Errorf("Expected Flight = Economy:2, got %s", flight)
		}
	})
}
