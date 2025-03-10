package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
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

// Server represents a server in the distributed system
type Server struct {
	Address           string
	CoordinatorAddr   string
	DataStore         map[string]string
	DataMutex         sync.RWMutex
	TransactionLogs   map[string]TransactionLog
	TransactionsMutex sync.RWMutex
	Logger            *log.Logger
}

// NewServer creates a new server instance
func NewServer(address, coordinatorAddr string) *Server {
	return &Server{
		Address:         address,
		CoordinatorAddr: coordinatorAddr,
		DataStore:       make(map[string]string),
		TransactionLogs: make(map[string]TransactionLog),
		Logger:          log.New(os.Stdout, fmt.Sprintf("[SERVER %s] ", address), log.LstdFlags),
	}
}

// Start starts the server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.Address)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}
	defer listener.Close()

	s.Logger.Printf("Server started on %s", s.Address)

	// Start a goroutine to periodically print the server state
	go s.debugPrintState()

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.Logger.Printf("Error accepting connection: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// debugPrintState periodically prints the server state for debugging
func (s *Server) debugPrintState() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.DataMutex.RLock()
		s.Logger.Printf("Current DataStore state: %+v", s.DataStore)
		s.DataMutex.RUnlock()

		s.TransactionsMutex.RLock()
		s.Logger.Printf("Current TransactionLogs state: %d transactions", len(s.TransactionLogs))
		for txID, txLog := range s.TransactionLogs {
			s.Logger.Printf("  - Transaction %s: Status=%s, Operations=%d", txID, txLog.Status, len(txLog.Operations))
		}
		s.TransactionsMutex.RUnlock()
	}
}

// handleConnection handles client connections
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	s.Logger.Printf("New connection from %s", remoteAddr)

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		s.Logger.Printf("Waiting for message from %s", remoteAddr)
		err := decoder.Decode(&msg)
		if err != nil {
			if err != io.EOF {
				s.Logger.Printf("Error decoding message from %s: %v", remoteAddr, err)
			} else {
				s.Logger.Printf("Connection closed by %s", remoteAddr)
			}
			break
		}

		s.Logger.Printf("Received message from %s: %+v", remoteAddr, msg)

		var response Message

		switch msg.Type {
		case "PREPARE":
			s.Logger.Printf("Handling PREPARE message from %s", remoteAddr)
			response = s.handlePrepare(msg)
		case "COMMIT":
			s.Logger.Printf("Handling COMMIT message from %s", remoteAddr)
			response = s.handleCommit(msg)
		case "ABORT":
			s.Logger.Printf("Handling ABORT message from %s", remoteAddr)
			response = s.handleAbort(msg)
		case "GET":
			s.Logger.Printf("Handling GET message from %s for key %s", remoteAddr, msg.Key)
			// Handle GET request
			s.DataMutex.RLock()
			value, exists := s.DataStore[msg.Key]
			s.DataMutex.RUnlock()

			if exists {
				s.Logger.Printf("Key %s found with value %s", msg.Key, value)
				response = Message{
					Type:      "RESPONSE",
					Key:       msg.Key,
					Value:     value,
					Status:    "OK",
					Timestamp: time.Now(),
				}
			} else {
				s.Logger.Printf("Key %s not found", msg.Key)
				response = Message{
					Type:      "RESPONSE",
					Key:       msg.Key,
					Status:    "ERROR",
					Error:     fmt.Sprintf("Key %s not found", msg.Key),
					Timestamp: time.Now(),
				}
			}
		case "SET":
			s.Logger.Printf("Handling SET message from %s for key %s with value %s", remoteAddr, msg.Key, msg.Value)
			// Handle SET request
			s.DataMutex.Lock()
			s.DataStore[msg.Key] = msg.Value
			s.DataMutex.Unlock()
			s.Logger.Printf("Key %s set to value %s", msg.Key, msg.Value)

			response = Message{
				Type:      "RESPONSE",
				Key:       msg.Key,
				Status:    "OK",
				Timestamp: time.Now(),
			}
		case "DELETE":
			s.Logger.Printf("Handling DELETE message from %s for key %s", remoteAddr, msg.Key)
			// Handle DELETE request
			s.DataMutex.Lock()
			delete(s.DataStore, msg.Key)
			s.DataMutex.Unlock()
			s.Logger.Printf("Key %s deleted", msg.Key)

			response = Message{
				Type:      "RESPONSE",
				Key:       msg.Key,
				Status:    "OK",
				Timestamp: time.Now(),
			}
		default:
			s.Logger.Printf("Unknown message type from %s: %s", remoteAddr, msg.Type)
			response = Message{
				Type:      "RESPONSE",
				Status:    "ERROR",
				Error:     fmt.Sprintf("Unknown message type: %s", msg.Type),
				Timestamp: time.Now(),
			}
		}

		s.Logger.Printf("Sending response to %s: %+v", remoteAddr, response)
		err = encoder.Encode(response)
		if err != nil {
			s.Logger.Printf("Error encoding response to %s: %v", remoteAddr, err)
			break
		}
		s.Logger.Printf("Response sent to %s", remoteAddr)
	}
}

// handlePrepare handles the prepare phase of 2PC
func (s *Server) handlePrepare(msg Message) Message {
	s.Logger.Printf("Handling PREPARE for transaction %s with %d operations", msg.TxID, len(msg.Operations))

	// Check if we can prepare all operations
	canPrepare := true
	resources := []string{}

	for _, op := range msg.Operations {
		s.Logger.Printf("Checking operation: %s on key %s", op.Type, op.Key)
		
		switch op.Type {
		case "DEBIT":
			// Check if account exists and has sufficient funds
			s.DataMutex.RLock()
			balanceStr, exists := s.DataStore[op.Key]
			s.DataMutex.RUnlock()

			if !exists {
				s.Logger.Printf("Account %s does not exist", op.Key)
				return Message{
					Type:      "RESPONSE",
					TxID:      msg.TxID,
					Status:    "ERROR",
					Error:     fmt.Sprintf("Account %s does not exist", op.Key),
					Timestamp: time.Now(),
				}
			}

			balance, err := strconv.ParseFloat(balanceStr, 64)
			if err != nil {
				s.Logger.Printf("Invalid balance format for account %s: %s", op.Key, balanceStr)
				return Message{
					Type:      "RESPONSE",
					TxID:      msg.TxID,
					Status:    "ERROR",
					Error:     fmt.Sprintf("Invalid balance format for account %s", op.Key),
					Timestamp: time.Now(),
				}
			}

			amount, err := strconv.ParseFloat(op.Value, 64)
			if err != nil {
				s.Logger.Printf("Invalid amount format: %s", op.Value)
				return Message{
					Type:      "RESPONSE",
					TxID:      msg.TxID,
					Status:    "ERROR",
					Error:     fmt.Sprintf("Invalid amount format: %s", op.Value),
					Timestamp: time.Now(),
				}
			}

			if balance < amount {
				s.Logger.Printf("Insufficient funds in account %s: %.2f < %.2f", op.Key, balance, amount)
				return Message{
					Type:      "RESPONSE",
					TxID:      msg.TxID,
					Status:    "ERROR",
					Error:     fmt.Sprintf("Insufficient funds in account %s: %.2f < %.2f", op.Key, balance, amount),
					Timestamp: time.Now(),
				}
			}
			
			s.Logger.Printf("Account %s has sufficient funds: %.2f >= %.2f", op.Key, balance, amount)
			resources = append(resources, op.Key)
			
		case "CREDIT":
			// Just check if account exists
			s.DataMutex.RLock()
			_, exists := s.DataStore[op.Key]
			s.DataMutex.RUnlock()

			if !exists {
				s.Logger.Printf("Account %s does not exist", op.Key)
				return Message{
					Type:      "RESPONSE",
					TxID:      msg.TxID,
					Status:    "ERROR",
					Error:     fmt.Sprintf("Account %s does not exist", op.Key),
					Timestamp: time.Now(),
				}
			}
			
			s.Logger.Printf("Account %s exists for credit operation", op.Key)
			resources = append(resources, op.Key)
			
		case "SET":
			// No validation needed for SET
			s.Logger.Printf("No validation needed for SET operation on key %s", op.Key)
			resources = append(resources, op.Key)
			
		default:
			s.Logger.Printf("Unknown operation type: %s", op.Type)
			return Message{
				Type:      "RESPONSE",
				TxID:      msg.TxID,
				Status:    "ERROR",
				Error:     fmt.Sprintf("Unknown operation type: %s", op.Type),
				Timestamp: time.Now(),
			}
		}
	}

	if canPrepare {
		// Log the transaction as prepared
		s.TransactionsMutex.Lock()
		s.TransactionLogs[msg.TxID] = TransactionLog{
			TxID:       msg.TxID,
			Status:     PREPARED,
			Operations: msg.Operations,
			Timestamp:  time.Now(),
		}
		s.TransactionsMutex.Unlock()
		
		s.Logger.Printf("Transaction %s prepared successfully with %d resources", msg.TxID, len(resources))

		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "OK",
			Resources: resources,
			Timestamp: time.Now(),
		}
	}

	s.Logger.Printf("Failed to prepare transaction %s", msg.TxID)
	return Message{
		Type:      "RESPONSE",
		TxID:      msg.TxID,
		Status:    "ERROR",
		Error:     "Failed to prepare transaction",
		Timestamp: time.Now(),
	}
}

// handleCommit handles the commit phase of 2PC
func (s *Server) handleCommit(msg Message) Message {
	s.Logger.Printf("Handling COMMIT for transaction %s", msg.TxID)

	// Check if the transaction is prepared
	s.TransactionsMutex.RLock()
	txLog, exists := s.TransactionLogs[msg.TxID]
	s.TransactionsMutex.RUnlock()

	if !exists {
		s.Logger.Printf("Transaction %s not found", msg.TxID)
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     fmt.Sprintf("Transaction %s not found", msg.TxID),
			Timestamp: time.Now(),
		}
	}
	
	if txLog.Status != PREPARED {
		s.Logger.Printf("Transaction %s not in PREPARED state, current state: %s", msg.TxID, txLog.Status)
		return Message{
			Type:      "RESPONSE",
			TxID:      msg.TxID,
			Status:    "ERROR",
			Error:     fmt.Sprintf("Transaction %s not prepared", msg.TxID),
			Timestamp: time.Now(),
		}
	}

	// Apply all operations
	s.Logger.Printf("Applying %d operations for transaction %s", len(txLog.Operations), msg.TxID)
	for _, op := range txLog.Operations {
		switch op.Type {
		case "DEBIT":
			s.DataMutex.Lock()
			balanceStr := s.DataStore[op.Key]
			balance, _ := strconv.ParseFloat(balanceStr, 64)
			amount, _ := strconv.ParseFloat(op.Value, 64)
			newBalance := balance - amount
			s.DataStore[op.Key] = fmt.Sprintf("%.2f", newBalance)
			s.Logger.Printf("DEBIT: Updated account %s balance from %.2f to %.2f", op.Key, balance, newBalance)
			s.DataMutex.Unlock()
		case "CREDIT":
			s.DataMutex.Lock()
			balanceStr := s.DataStore[op.Key]
			balance, _ := strconv.ParseFloat(balanceStr, 64)
			amount, _ := strconv.ParseFloat(op.Value, 64)
			newBalance := balance + amount
			s.DataStore[op.Key] = fmt.Sprintf("%.2f", newBalance)
			s.Logger.Printf("CREDIT: Updated account %s balance from %.2f to %.2f", op.Key, balance, newBalance)
			s.DataMutex.Unlock()
		case "SET":
			s.DataMutex.Lock()
			oldValue, exists := s.DataStore[op.Key]
			s.DataStore[op.Key] = op.Value
			if exists {
				s.Logger.Printf("SET: Updated key %s from %s to %s", op.Key, oldValue, op.Value)
			} else {
				s.Logger.Printf("SET: Created key %s with value %s", op.Key, op.Value)
			}
			s.DataMutex.Unlock()
		}
	}

	// Update transaction status
	s.TransactionsMutex.Lock()
	txLog.Status = COMMITTED
	s.TransactionLogs[msg.TxID] = txLog
	s.TransactionsMutex.Unlock()
	
	s.Logger.Printf("Transaction %s committed successfully", msg.TxID)

	return Message{
		Type:      "RESPONSE",
		TxID:      msg.TxID,
		Status:    "OK",
		Timestamp: time.Now(),
	}
}

// handleAbort handles the abort phase of 2PC
func (s *Server) handleAbort(msg Message) Message {
	s.Logger.Printf("Handling ABORT for transaction %s", msg.TxID)

	// Update transaction status if it exists
	s.TransactionsMutex.Lock()
	if txLog, exists := s.TransactionLogs[msg.TxID]; exists {
		txLog.Status = ABORTED
		s.TransactionLogs[msg.TxID] = txLog
	}
	s.TransactionsMutex.Unlock()

	return Message{
		Type:      "RESPONSE",
		TxID:      msg.TxID,
		Status:    "OK",
		Timestamp: time.Now(),
	}
}

func main() {
	address := flag.String("address", "localhost:8001", "Server address")
	coordinator := flag.String("coordinator", "localhost:8000", "Coordinator address")
	flag.Parse()

	server := NewServer(*address, *coordinator)
	err := server.Start()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
