package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TransactionManager manages distributed transactions
type TransactionManager struct {
	transactions    map[string]*Transaction
	mutex           sync.RWMutex
	consistentHash  *ConsistentHash
	serverAddresses []string
}

// Transaction represents a distributed transaction
type Transaction struct {
	ID           string
	Status       TransactionStatus
	Operations   []Operation
	Participants []string
	StartTime    time.Time
	EndTime      time.Time
	mutex        sync.RWMutex
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(consistentHash *ConsistentHash, serverAddresses []string) *TransactionManager {
	return &TransactionManager{
		transactions:    make(map[string]*Transaction),
		consistentHash:  consistentHash,
		serverAddresses: serverAddresses,
	}
}

// BeginTransaction starts a new transaction
func (tm *TransactionManager) BeginTransaction() *Transaction {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	txID := generateTransactionID()
	tx := &Transaction{
		ID:           txID,
		Status:       IN_PROGRESS,
		Operations:   []Operation{},
		Participants: []string{},
		StartTime:    time.Now(),
	}

	tm.transactions[txID] = tx
	return tx
}

// GetTransaction retrieves a transaction by ID
func (tm *TransactionManager) GetTransaction(txID string) (*Transaction, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	tx, exists := tm.transactions[txID]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found", txID)
	}
	return tx, nil
}

// AddOperation adds an operation to a transaction
func (tx *Transaction) AddOperation(op Operation) {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	tx.Operations = append(tx.Operations, op)
}

// SetStatus sets the status of a transaction
func (tx *Transaction) SetStatus(status TransactionStatus) {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	tx.Status = status
	if status == COMMITTED || status == ABORTED {
		tx.EndTime = time.Now()
	}
}

// AddParticipant adds a participant server to the transaction
func (tx *Transaction) AddParticipant(serverAddr string) {
	tx.mutex.Lock()
	defer tx.mutex.Unlock()

	// Check if the server is already a participant
	for _, p := range tx.Participants {
		if p == serverAddr {
			return
		}
	}
	tx.Participants = append(tx.Participants, serverAddr)
}

// GetParticipants returns the list of participant servers
func (tx *Transaction) GetParticipants() []string {
	tx.mutex.RLock()
	defer tx.mutex.RUnlock()

	return tx.Participants
}

// GetOperations returns the list of operations in the transaction
func (tx *Transaction) GetOperations() []Operation {
	tx.mutex.RLock()
	defer tx.mutex.RUnlock()

	return tx.Operations
}

// GetStatus returns the current status of the transaction
func (tx *Transaction) GetStatus() TransactionStatus {
	tx.mutex.RLock()
	defer tx.mutex.RUnlock()

	return tx.Status
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("tx-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
