package shared

import (
	"time"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	// Transaction statuses
	PREPARED    TransactionStatus = "PREPARED"
	COMMITTED   TransactionStatus = "COMMITTED"
	ABORTED     TransactionStatus = "ABORTED"
	IN_PROGRESS TransactionStatus = "IN_PROGRESS"
)

// Operation types
const (
	GET    = "GET"
	SET    = "SET"
	DELETE = "DELETE"
)

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

// Operation represents a single operation in a transaction
type Operation struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// TransactionLog represents a log entry for a transaction
type TransactionLog struct {
	TxID       string            `json:"tx_id"`
	Status     TransactionStatus `json:"status"`
	Operations []Operation       `json:"operations"`
	Timestamp  time.Time         `json:"timestamp"`
}

// ServerInfo represents information about a server
type ServerInfo struct {
	Address string   `json:"address"`
	Keys    []string `json:"keys,omitempty"`
}
