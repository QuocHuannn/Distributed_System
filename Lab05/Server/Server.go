package main

import (
	"Lab05/Shared"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

// Server defines the structure of a server in the system
type Server struct {
	ID            string                // Server ID
	Address       string                // Server address (host:port)
	Data          map[string]string     // Stored data
	Mutex         sync.RWMutex          // Mutex to ensure thread-safety
	ConsistentMap *shared.ConsistentHash // Consistent hash map
	Servers       []shared.ServerInfo    // List of servers in the system
	Logger        *log.Logger           // Logger
}

func main() {
	// Read command line parameters
	serverID := flag.String("id", "server1", "Server ID")
	serverAddr := flag.String("addr", "localhost:8081", "Server address (host:port)")
	logFile := flag.String("log", "server.log", "Log file path")
	flag.Parse()

	// Create logger
	file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer file.Close()
	logger := log.New(file, fmt.Sprintf("[%s] ", *serverID), log.LstdFlags)

	// Initialize server
	server := &Server{
		ID:            *serverID,
		Address:       *serverAddr,
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
		logger.Fatalf("Cannot start server: %v", err)
	}
	defer listener.Close()

	logger.Printf("Server started at %s", server.Address)

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Printf("Error accepting connection: %v", err)
			continue
		}
		go server.handleConnection(conn)
	}
}

// handleConnection handles a client connection
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

		// Determine the server responsible for this key
		responsibleServer := s.ConsistentMap.Get(msg.Key)
		
		// If the current server is not the responsible server
		if responsibleServer != s.Address && responsibleServer != "" {
			s.Logger.Printf("Forwarding request to server %s", responsibleServer)
			
			// Forward the request to the responsible server
			response := s.forwardRequest(responsibleServer, msg)
			encoder.Encode(response)
			continue
		}

		// Process the request if the current server is the responsible server
		response := s.handleRequest(msg)
		s.Logger.Printf("Sending response: %+v", response)
		encoder.Encode(response)
	}
}

// handleRequest processes a client request
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

	return response
}

// forwardRequest forwards a request to another server
func (s *Server) forwardRequest(serverAddr string, msg shared.Message) shared.Message {
	// Connect to the target server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		s.Logger.Printf("Error connecting to server %s: %v", serverAddr, err)
		return shared.Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error connecting to responsible server",
		}
	}
	defer conn.Close()

	// Send the request
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	err = encoder.Encode(msg)
	if err != nil {
		s.Logger.Printf("Error sending request to server %s: %v", serverAddr, err)
		return shared.Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error sending request to responsible server",
		}
	}

	// Receive the response
	var response shared.Message
	err = decoder.Decode(&response)
	if err != nil {
		s.Logger.Printf("Error receiving response from server %s: %v", serverAddr, err)
		return shared.Message{
			Type:      "RESPONSE",
			Key:       msg.Key,
			Timestamp: time.Now(),
			Status:    "ERROR",
			Error:     "Error receiving response from responsible server",
		}
	}

	return response
}

// redistributeData redistributes data when the system configuration changes
func (s *Server) redistributeData() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	// Create a copy of the current data
	dataCopy := make(map[string]string)
	for k, v := range s.Data {
		dataCopy[k] = v
	}

	// Clear all current data
	s.Data = make(map[string]string)

	// Redistribute data
	for key, value := range dataCopy {
		responsibleServer := s.ConsistentMap.Get(key)
		
		// If the current server is the responsible server, keep the data
		if responsibleServer == s.Address {
			s.Data[key] = value
		} else {
			// Send data to the responsible server
			msg := shared.Message{
				Type:      "SET",
				Key:       key,
				Value:     value,
				Timestamp: time.Now(),
			}
			s.forwardRequest(responsibleServer, msg)
		}
	}

	s.Logger.Printf("Data redistributed after system configuration change")
}

// Export HandleConnection method for testing
func (s *Server) HandleConnection(conn net.Conn) {
	s.handleConnection(conn)
}

// Export RedistributeData method for testing
func (s *Server) RedistributeData() {
	s.redistributeData()
} 