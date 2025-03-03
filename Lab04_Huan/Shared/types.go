package shared

import (
	"time"
)

// Message defines the structure of communication messages between client and server
type Message struct {
	Type      string    // Message type: "GET", "SET", "DELETE", "RESPONSE"
	Key       string    // Data key
	Value     string    // Data value
	Timestamp time.Time // Message timestamp
	Status    string    // Status: "SUCCESS", "ERROR"
	Error     string    // Error message if any
}

// ServerInfo contains information about a server in the system
type ServerInfo struct {
	ID      string // Server ID
	Address string // Server address (host:port)
} 