package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Define shared types here since we're having import issues
type TransactionStatus string

const (
	// Transaction statuses
	PREPARED    TransactionStatus = "PREPARED"
	COMMITTED   TransactionStatus = "COMMITTED"
	ABORTED     TransactionStatus = "ABORTED"
	IN_PROGRESS TransactionStatus = "IN_PROGRESS"
)

// Operation represents a single operation in a transaction
type Operation struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// Message represents a message exchanged between components
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

// TransactionLog represents a log entry for a transaction
type TransactionLog struct {
	TxID       string            `json:"tx_id"`
	Status     TransactionStatus `json:"status"`
	Operations []Operation       `json:"operations"`
	Timestamp  time.Time         `json:"timestamp"`
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

// ConsistentHash implements the consistent hashing algorithm
type ConsistentHash struct {
	ring       map[uint32]string // Hash ring
	sortedKeys []uint32          // Sorted keys
	replicas   int               // Number of virtual replicas for each node
	mutex      sync.RWMutex      // Mutex to ensure thread-safety
}

// TransactionManager manages distributed transactions
type TransactionManager struct {
	transactions    map[string]*Transaction
	mutex           sync.RWMutex
	consistentHash  *ConsistentHash
	serverAddresses []string
}

// Coordinator manages distributed transactions using Two-Phase Commit
type Coordinator struct {
	Address            string
	TransactionManager *TransactionManager
	ConsistentHash     *ConsistentHash
	ServerAddresses    []string
	Logger             *log.Logger
	DataStore          map[string]string
	DataMutex          sync.RWMutex
}

// NewCoordinator creates a new coordinator instance
func NewCoordinator(address string, serverAddresses []string) *Coordinator {
	consistentHash := &ConsistentHash{
		ring:       make(map[uint32]string),
		sortedKeys: []uint32{},
		replicas:   10,
		mutex:      sync.RWMutex{},
	}
	for _, addr := range serverAddresses {
		consistentHash.Add(addr)
	}

	txManager := &TransactionManager{
		transactions:    make(map[string]*Transaction),
		mutex:           sync.RWMutex{},
		consistentHash:  consistentHash,
		serverAddresses: serverAddresses,
	}

	return &Coordinator{
		Address:            address,
		TransactionManager: txManager,
		ConsistentHash:     consistentHash,
		ServerAddresses:    serverAddresses,
		Logger:             log.New(os.Stdout, "[COORDINATOR] ", log.LstdFlags),
		DataStore:          make(map[string]string),
	}
}

// Start starts the coordinator server
func (c *Coordinator) Start() error {
	listener, err := net.Listen("tcp", c.Address)
	if err != nil {
		return fmt.Errorf("failed to start coordinator: %v", err)
	}
	defer listener.Close()

	c.Logger.Printf("Coordinator started on %s", c.Address)
	c.Logger.Printf("Server addresses: %v", c.ServerAddresses)

	// Debug print the consistent hash ring
	c.debugPrintConsistentHash()

	// Start a goroutine to periodically print the coordinator state
	go c.debugPrintState()

	for {
		conn, err := listener.Accept()
		if err != nil {
			c.Logger.Printf("Error accepting connection: %v", err)
			continue
		}

		go c.handleConnection(conn)
	}
}

// handleConnection handles client connections
func (c *Coordinator) handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	c.Logger.Printf("New connection from %s", remoteAddr)

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		c.Logger.Printf("Waiting for message from %s", remoteAddr)
		err := decoder.Decode(&msg)
		if err != nil {
			if err != io.EOF {
				c.Logger.Printf("Error decoding message from %s: %v", remoteAddr, err)
			} else {
				c.Logger.Printf("Connection closed by %s", remoteAddr)
			}
			break
		}

		c.Logger.Printf("Received message from %s: %+v", remoteAddr, msg)

		var response Message

		switch msg.Type {
		case "BEGIN_TX":
			c.Logger.Printf("Handling BEGIN_TX message from %s", remoteAddr)
			response = c.handleBeginTransaction()
		case "PREPARE_TX":
			c.Logger.Printf("Handling PREPARE_TX message from %s", remoteAddr)
			response = c.handlePrepareTransaction(msg)
		case "COMMIT_TX":
			c.Logger.Printf("Handling COMMIT_TX message from %s", remoteAddr)
			response = c.handleCommitTransaction(msg)
		case "ABORT_TX":
			c.Logger.Printf("Handling ABORT_TX message from %s", remoteAddr)
			response = c.handleAbortTransaction(msg)
		case "TRANSFER_MONEY":
			c.Logger.Printf("Handling TRANSFER_MONEY message from %s", remoteAddr)
			response = c.handleTransferMoney(msg)
		case "BOOK_TRIP":
			c.Logger.Printf("Handling BOOK_TRIP message from %s", remoteAddr)
			response = c.handleBookTrip(msg)
		case "GET":
			c.Logger.Printf("Handling GET message from %s for key %s", remoteAddr, msg.Key)
			// Forward GET request to the appropriate server
			server := c.ConsistentHash.Get(msg.Key)
			if server == "" {
				c.Logger.Printf("No server available for key %s", msg.Key)
				response = Message{
					Type:      "RESPONSE",
					Key:       msg.Key,
					Status:    "ERROR",
					Error:     "No server available",
					Timestamp: time.Now(),
				}
			} else {
				c.Logger.Printf("Forwarding GET request for key %s to server %s", msg.Key, server)
				response = c.forwardRequest(server, msg)
			}
		case "SET":
			c.Logger.Printf("Handling SET message from %s for key %s with value %s", remoteAddr, msg.Key, msg.Value)
			// Forward SET request to the appropriate server
			server := c.ConsistentHash.Get(msg.Key)
			if server == "" {
				c.Logger.Printf("No server available for key %s", msg.Key)
				response = Message{
					Type:      "RESPONSE",
					Key:       msg.Key,
					Status:    "ERROR",
					Error:     "No server available",
					Timestamp: time.Now(),
				}
			} else {
				c.Logger.Printf("Forwarding SET request for key %s to server %s", msg.Key, server)
				response = c.forwardRequest(server, msg)
			}
		case "DELETE":
			c.Logger.Printf("Handling DELETE message from %s for key %s", remoteAddr, msg.Key)
			// Forward DELETE request to the appropriate server
			server := c.ConsistentHash.Get(msg.Key)
			if server == "" {
				c.Logger.Printf("No server available for key %s", msg.Key)
				response = Message{
					Type:      "RESPONSE",
					Key:       msg.Key,
					Status:    "ERROR",
					Error:     "No server available",
					Timestamp: time.Now(),
				}
			} else {
				c.Logger.Printf("Forwarding DELETE request for key %s to server %s", msg.Key, server)
				response = c.forwardRequest(server, msg)
			}
		default:
			c.Logger.Printf("Unknown message type from %s: %s", remoteAddr, msg.Type)
			response = Message{
				Type:      "RESPONSE",
				Status:    "ERROR",
				Error:     fmt.Sprintf("Unknown message type: %s", msg.Type),
				Timestamp: time.Now(),
			}
		}

		c.Logger.Printf("Sending response to %s: %+v", remoteAddr, response)
		err = encoder.Encode(response)
		if err != nil {
			c.Logger.Printf("Error encoding response to %s: %v", remoteAddr, err)
			break
		}
		c.Logger.Printf("Response sent to %s", remoteAddr)
	}
}

// handleBeginTransaction starts a new transaction
func (c *Coordinator) handleBeginTransaction() Message {
	tx := c.TransactionManager.BeginTransaction()
	return Message{
		Type:      "RESPONSE",
		TxID:      tx.ID,
		Status:    "OK",
		Timestamp: time.Now(),
	}
}

// handlePrepareTransaction handles the prepare phase of 2PC
func (c *Coordinator) handlePrepareTransaction(msg Message) Message {
	tx, err := c.TransactionManager.GetTransaction(msg.TxID)
	if err != nil {
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	// Get all participants
	participants := tx.GetParticipants()
	if len(participants) == 0 {
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     "No participants in transaction",
			Timestamp: time.Now(),
		}
	}

	// Send PREPARE message to all participants
	allPrepared := true
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, participant := range participants {
		wg.Add(1)
		go func(serverAddr string) {
			defer wg.Done()
			prepared := c.sendPrepareToServer(msg.TxID, tx.GetOperations(), serverAddr)
			if !prepared {
				mu.Lock()
				allPrepared = false
				mu.Unlock()
			}
		}(participant)
	}

	wg.Wait()

	if allPrepared {
		tx.SetStatus(COMMITTED)
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "OK",
			Timestamp: time.Now(),
		}
	} else {
		// If any server failed to prepare, abort the transaction
		c.abortTransaction(tx)
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     "Failed to prepare transaction",
			Timestamp: time.Now(),
		}
	}
}

// handleCommitTransaction handles the commit phase of 2PC
func (c *Coordinator) handleCommitTransaction(msg Message) Message {
	tx, err := c.TransactionManager.GetTransaction(msg.TxID)
	if err != nil {
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     fmt.Sprintf("Transaction %s not found", msg.TxID),
			Timestamp: time.Now(),
		}
	}

	participants := tx.GetParticipants()
	if len(participants) == 0 {
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     "No participants in transaction",
			Timestamp: time.Now(),
		}
	}

	// Send COMMIT to all participants
	var wg sync.WaitGroup
	var mu sync.Mutex
	allCommitted := true

	c.Logger.Printf("Sending COMMIT to %d participants for transaction %s", len(participants), msg.TxID)

	for _, server := range participants {
		wg.Add(1)
		go func(serverAddr string) {
			defer wg.Done()
			committed := c.sendCommitToServer(msg.TxID, serverAddr)
			if !committed {
				mu.Lock()
				allCommitted = false
				mu.Unlock()
			}
		}(server)
	}

	wg.Wait()

	if allCommitted {
		tx.SetStatus(COMMITTED)
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "OK",
			Timestamp: time.Now(),
		}
	} else {
		// In a real system, we would need recovery procedures
		c.Logger.Printf("CRITICAL ERROR: Partial commit of transaction %s", msg.TxID)
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     "Failed to commit transaction on all servers",
			Timestamp: time.Now(),
		}
	}
}

// handleAbortTransaction handles transaction abort
func (c *Coordinator) handleAbortTransaction(msg Message) Message {
	tx, err := c.TransactionManager.GetTransaction(msg.TxID)
	if err != nil {
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
	}

	c.abortTransaction(tx)
	return Message{
		Type:      "RESPONSE",
		TxID:      msg.TxID,
		Status:    "OK",
		Timestamp: time.Now(),
	}
}

// abortTransaction aborts a transaction
func (c *Coordinator) abortTransaction(tx *Transaction) {
	// Send ABORT message to all participants
	var wg sync.WaitGroup

	for _, participant := range tx.GetParticipants() {
		wg.Add(1)
		go func(serverAddr string) {
			defer wg.Done()
			c.sendAbortToServer(tx.ID, serverAddr)
		}(participant)
	}

	wg.Wait()
	tx.SetStatus(ABORTED)
}

// handleTransferMoney handles money transfer between accounts
func (c *Coordinator) handleTransferMoney(msg Message) Message {
	c.Logger.Printf("Handling money transfer with %d operations", len(msg.Operations))

	// Parse the transfer details
	var fromAccount, toAccount, amount string
	if len(msg.Operations) >= 2 {
		fromAccount = msg.Operations[0].Key
		amount = msg.Operations[0].Value
		toAccount = msg.Operations[1].Key
	} else {
		return Message{
			Type:      "RESPONSE",
			Status:    "ERROR",
			Error:     "Invalid transfer request format",
			Timestamp: time.Now(),
		}
	}

	// Begin a new transaction
	tx := c.TransactionManager.BeginTransaction()
	c.Logger.Printf("Started transaction %s for money transfer from %s to %s", tx.ID, fromAccount, toAccount)

	// Add operations to the transaction
	tx.AddOperation(Operation{
		Type:  "DEBIT",
		Key:   fromAccount,
		Value: amount,
	})
	tx.AddOperation(Operation{
		Type:  "CREDIT",
		Key:   toAccount,
		Value: amount,
	})

	// Determine which servers are involved
	fromServer := c.ConsistentHash.Get(fromAccount)
	toServer := c.ConsistentHash.Get(toAccount)

	// Add participants
	if fromServer != "" {
		tx.AddParticipant(fromServer)
	}
	if toServer != "" && toServer != fromServer {
		tx.AddParticipant(toServer)
	}

	participants := tx.GetParticipants()
	c.Logger.Printf("Transaction %s has %d participants: %v", tx.ID, len(participants), participants)

	if len(participants) == 0 {
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "ERROR",
			Error:     "No servers available for the accounts",
			Timestamp: time.Now(),
		}
	}

	// Prepare phase - send directly to servers
	prepareSuccess := true
	var prepareError string

	for _, server := range participants {
		c.Logger.Printf("Sending PREPARE to server %s for transaction %s", server, tx.ID)
		success := c.sendPrepareToServer(tx.ID, tx.GetOperations(), server)
		if !success {
			prepareSuccess = false
			prepareError = "Failed to prepare transaction"
			break
		}
	}

	if !prepareSuccess {
		// Abort the transaction
		c.abortTransaction(tx)
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "ERROR",
			Error:     prepareError,
			Timestamp: time.Now(),
		}
	}

	// Set transaction status to PREPARED
	tx.SetStatus(PREPARED)

	// Commit phase - send directly to servers
	commitSuccess := true

	for _, server := range participants {
		c.Logger.Printf("Sending COMMIT to server %s for transaction %s", server, tx.ID)
		success := c.sendCommitToServer(tx.ID, server)
		if !success {
			commitSuccess = false
			break
		}
	}

	if commitSuccess {
		tx.SetStatus(COMMITTED)
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "OK",
			Timestamp: time.Now(),
		}
	} else {
		c.Logger.Printf("CRITICAL ERROR: Partial commit of transaction %s", tx.ID)
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "ERROR",
			Error:     "Failed to commit transaction on all servers",
			Timestamp: time.Now(),
		}
	}
}

// handleBookTrip handles booking a trip (car, hotel, flight)
func (c *Coordinator) handleBookTrip(msg Message) Message {
	c.Logger.Printf("Handling trip booking with %d operations", len(msg.Operations))

	if len(msg.Operations) < 3 {
		return Message{
			Type:      "RESPONSE",
			Status:    "ERROR",
			Error:     "Invalid booking request format",
			Timestamp: time.Now(),
		}
	}

	// Begin a new transaction
	tx := c.TransactionManager.BeginTransaction()
	c.Logger.Printf("Started transaction %s for trip booking", tx.ID)

	// Add all operations to the transaction
	for _, op := range msg.Operations {
		tx.AddOperation(op)

		// Add the server responsible for this resource as a participant
		server := c.ConsistentHash.Get(op.Key)
		if server != "" {
			tx.AddParticipant(server)
		}
	}

	participants := tx.GetParticipants()
	c.Logger.Printf("Transaction %s has %d participants: %v", tx.ID, len(participants), participants)

	if len(participants) == 0 {
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "ERROR",
			Error:     "No servers available for the resources",
			Timestamp: time.Now(),
		}
	}

	// Prepare phase - send directly to servers
	prepareSuccess := true
	var prepareError string

	for _, server := range participants {
		c.Logger.Printf("Sending PREPARE to server %s for transaction %s", server, tx.ID)
		success := c.sendPrepareToServer(tx.ID, tx.GetOperations(), server)
		if !success {
			prepareSuccess = false
			prepareError = "Failed to prepare transaction"
			break
		}
	}

	if !prepareSuccess {
		// Abort the transaction
		c.abortTransaction(tx)
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "ERROR",
			Error:     prepareError,
			Timestamp: time.Now(),
		}
	}

	// Set transaction status to PREPARED
	tx.SetStatus(PREPARED)

	// Commit phase - send directly to servers
	commitSuccess := true

	for _, server := range participants {
		c.Logger.Printf("Sending COMMIT to server %s for transaction %s", server, tx.ID)
		success := c.sendCommitToServer(tx.ID, server)
		if !success {
			commitSuccess = false
			break
		}
	}

	if commitSuccess {
		tx.SetStatus(COMMITTED)
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "OK",
			Timestamp: time.Now(),
		}
	} else {
		c.Logger.Printf("CRITICAL ERROR: Partial commit of transaction %s", tx.ID)
		return Message{
			Type:      "RESPONSE",
			TxID:      tx.ID,
			Status:    "ERROR",
			Error:     "Failed to commit transaction on all servers",
			Timestamp: time.Now(),
		}
	}
}

// sendPrepareToServer sends a PREPARE message to a server
func (c *Coordinator) sendPrepareToServer(txID string, operations []Operation, serverAddr string) bool {
	c.Logger.Printf("Sending PREPARE to server %s for transaction %s with %d operations", serverAddr, txID, len(operations))

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		c.Logger.Printf("Error connecting to server %s: %v", serverAddr, err)
		return false
	}
	defer conn.Close()

	// Send PREPARE message
	msg := Message{
		Type:       "PREPARE",
		TxID:       txID,
		Operations: operations,
		Timestamp:  time.Now(),
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	err = encoder.Encode(msg)
	if err != nil {
		c.Logger.Printf("Error sending PREPARE to server %s: %v", serverAddr, err)
		return false
	}
	c.Logger.Printf("PREPARE message sent to server %s", serverAddr)

	// Receive response
	var response Message
	err = decoder.Decode(&response)
	if err != nil {
		c.Logger.Printf("Error receiving PREPARE response from server %s: %v", serverAddr, err)
		return false
	}
	c.Logger.Printf("Received PREPARE response from server %s: %+v", serverAddr, response)

	return response.Status == "OK"
}

// sendCommitToServer sends a COMMIT message to a server
func (c *Coordinator) sendCommitToServer(txID string, serverAddr string) bool {
	c.Logger.Printf("Sending COMMIT to server %s for transaction %s", serverAddr, txID)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		c.Logger.Printf("Error connecting to server %s: %v", serverAddr, err)
		return false
	}
	defer conn.Close()

	// Send COMMIT message
	msg := Message{
		Type:      "COMMIT",
		TxID:      txID,
		Timestamp: time.Now(),
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	err = encoder.Encode(msg)
	if err != nil {
		c.Logger.Printf("Error sending COMMIT to server %s: %v", serverAddr, err)
		return false
	}
	c.Logger.Printf("COMMIT message sent to server %s", serverAddr)

	// Receive response
	var response Message
	err = decoder.Decode(&response)
	if err != nil {
		c.Logger.Printf("Error receiving COMMIT response from server %s: %v", serverAddr, err)
		return false
	}
	c.Logger.Printf("Received COMMIT response from server %s: %+v", serverAddr, response)

	return response.Status == "OK"
}

// sendAbortToServer sends an ABORT message to a server
func (c *Coordinator) sendAbortToServer(txID string, serverAddr string) bool {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		c.Logger.Printf("Error connecting to server %s: %v", serverAddr, err)
		return false
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	// Send ABORT message
	msg := Message{
		Type:      "ABORT",
		TxID:      txID,
		Timestamp: time.Now(),
	}

	err = encoder.Encode(msg)
	if err != nil {
		c.Logger.Printf("Error sending ABORT to server %s: %v", serverAddr, err)
		return false
	}

	// Receive response
	var response Message
	err = decoder.Decode(&response)
	if err != nil {
		c.Logger.Printf("Error receiving response from server %s: %v", serverAddr, err)
		return false
	}

	return response.Status == "OK"
}

// forwardRequest forwards a request to a server
func (c *Coordinator) forwardRequest(serverAddr string, msg Message) Message {
	c.Logger.Printf("Forwarding %s request to server %s for key %s", msg.Type, serverAddr, msg.Key)

	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		c.Logger.Printf("Error connecting to server %s: %v", serverAddr, err)
		return Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error connecting to server",
		}
	}
	defer conn.Close()
	c.Logger.Printf("Connected to server %s", serverAddr)

	// Send the request
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	err = encoder.Encode(msg)
	if err != nil {
		c.Logger.Printf("Error sending request to server %s: %v", serverAddr, err)
		return Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error sending request to server",
		}
	}
	c.Logger.Printf("Request sent to server %s: %+v", serverAddr, msg)

	// Receive the response
	var response Message
	err = decoder.Decode(&response)
	if err != nil {
		c.Logger.Printf("Error receiving response from server %s: %v", serverAddr, err)
		return Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error receiving response from server",
		}
	}
	c.Logger.Printf("Response received from server %s: %+v", serverAddr, response)

	return response
}

// Add adds a new node to the hash ring
func (ch *ConsistentHash) Add(node string) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	// Create multiple virtual points for each node for better distribution
	for i := 0; i < ch.replicas; i++ {
		key := ch.hashKey(node + ":" + strconv.Itoa(i))
		ch.ring[key] = node
		ch.sortedKeys = append(ch.sortedKeys, key)
	}
	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

// Get returns the node responsible for a specific key
func (ch *ConsistentHash) Get(key string) string {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()

	if len(ch.ring) == 0 {
		return ""
	}

	hashKey := ch.hashKey(key)

	// Find the first position on the hash ring greater than or equal to hashKey
	idx := sort.Search(len(ch.sortedKeys), func(i int) bool {
		return ch.sortedKeys[i] >= hashKey
	})

	// If not found, wrap around to the beginning of the ring
	if idx == len(ch.sortedKeys) {
		idx = 0
	}

	return ch.ring[ch.sortedKeys[idx]]
}

// hashKey creates a hash value for a key
func (ch *ConsistentHash) hashKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
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

// debugPrintConsistentHash prints the consistent hash ring for debugging
func (c *Coordinator) debugPrintConsistentHash() {
	c.Logger.Printf("Consistent Hash Ring Configuration:")
	c.Logger.Printf("  - Number of servers: %d", len(c.ServerAddresses))
	c.Logger.Printf("  - Servers: %v", c.ServerAddresses)

	c.ConsistentHash.mutex.RLock()
	defer c.ConsistentHash.mutex.RUnlock()

	c.Logger.Printf("  - Ring size: %d", len(c.ConsistentHash.ring))
	c.Logger.Printf("  - Sorted keys: %d", len(c.ConsistentHash.sortedKeys))

	// Print the first few mappings for debugging
	count := 0
	for _, key := range c.ConsistentHash.sortedKeys {
		if count >= 10 {
			break
		}
		c.Logger.Printf("    - Hash %d -> Server %s", key, c.ConsistentHash.ring[key])
		count++
	}

	// Test a few keys to see where they would be routed
	testKeys := []string{"account1", "account2", "hotel1", "flight1", "car1"}
	for _, key := range testKeys {
		server := c.ConsistentHash.Get(key)
		c.Logger.Printf("  - Test key '%s' would be routed to server: %s", key, server)
	}
}

// debugPrintState periodically prints the coordinator state for debugging
func (c *Coordinator) debugPrintState() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.TransactionManager.mutex.RLock()
		c.Logger.Printf("Current TransactionManager state: %d active transactions", len(c.TransactionManager.transactions))
		for txID, tx := range c.TransactionManager.transactions {
			tx.mutex.RLock()
			c.Logger.Printf("  - Transaction %s: Status=%s, Operations=%d, Participants=%v",
				txID, tx.Status, len(tx.Operations), tx.Participants)
			tx.mutex.RUnlock()
		}
		c.TransactionManager.mutex.RUnlock()
	}
}

func main() {
	address := flag.String("address", "localhost:8000", "Coordinator address")
	serverList := flag.String("servers", "localhost:8001,localhost:8002,localhost:8003", "Comma-separated list of server addresses")
	flag.Parse()

	serverAddresses := []string{}
	if *serverList != "" {
		serverAddresses = strings.Split(*serverList, ",")
	}

	coordinator := NewCoordinator(*address, serverAddresses)
	err := coordinator.Start()
	if err != nil {
		log.Fatalf("Failed to start coordinator: %v", err)
	}
}
