package main

import (
	"Lab04/Shared"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Client defines the structure of a client
type Client struct {
	ServerAddr string // Server address (host:port)
	Conn       net.Conn
	Encoder    *json.Encoder
	Decoder    *json.Decoder
}

func main() {
	// Read command line parameters
	serverAddr := flag.String("server", "localhost:8081", "Server address (host:port)")
	flag.Parse()

	// Initialize client
	client := &Client{
		ServerAddr: *serverAddr,
	}

	// Connect to server
	err := client.Connect()
	if err != nil {
		fmt.Printf("Cannot connect to server: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("Connected to server", client.ServerAddr)
	fmt.Println("Enter command (GET key, SET key value, DELETE key, EXIT):")

	// Read commands from user
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		command := scanner.Text()
		if strings.ToUpper(command) == "EXIT" {
			break
		}

		// Process command
		err := client.ProcessCommand(command)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

// Connect connects to the server
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

// Close closes the connection
func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

// ProcessCommand processes a user command
func (c *Client) ProcessCommand(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("invalid command")
	}

	cmd := strings.ToUpper(parts[0])
	switch cmd {
	case "GET":
		if len(parts) != 2 {
			return fmt.Errorf("syntax: GET key")
		}
		return c.Get(parts[1])

	case "SET":
		if len(parts) < 3 {
			return fmt.Errorf("syntax: SET key value")
		}
		value := strings.Join(parts[2:], " ")
		return c.Set(parts[1], value)

	case "DELETE":
		if len(parts) != 2 {
			return fmt.Errorf("syntax: DELETE key")
		}
		return c.Delete(parts[1])

	default:
		return fmt.Errorf("invalid command: %s", cmd)
	}
}

// Get sends a GET request to the server
func (c *Client) Get(key string) error {
	msg := shared.Message{
		Type:      "GET",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}

	var response shared.Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("error receiving response: %v", err)
	}

	if response.Status == "SUCCESS" {
		fmt.Printf("Value of %s: %s\n", key, response.Value)
	} else {
		fmt.Printf("Error: %s\n", response.Error)
	}

	return nil
}

// Set sends a SET request to the server
func (c *Client) Set(key, value string) error {
	msg := shared.Message{
		Type:      "SET",
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}

	var response shared.Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("error receiving response: %v", err)
	}

	if response.Status == "SUCCESS" {
		fmt.Printf("Set %s = %s\n", key, value)
	} else {
		fmt.Printf("Error: %s\n", response.Error)
	}

	return nil
}

// Delete sends a DELETE request to the server
func (c *Client) Delete(key string) error {
	msg := shared.Message{
		Type:      "DELETE",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}

	var response shared.Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("error receiving response: %v", err)
	}

	if response.Status == "SUCCESS" {
		fmt.Printf("Deleted %s\n", key)
	} else {
		fmt.Printf("Error: %s\n", response.Error)
	}

	return nil
}

// GetValue returns the value for testing
func (c *Client) GetValue(key string) (string, error) {
	msg := shared.Message{
		Type:      "GET",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}

	var response shared.Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return "", fmt.Errorf("error receiving response: %v", err)
	}

	if response.Status == "SUCCESS" {
		return response.Value, nil
	} else {
		return "", fmt.Errorf(response.Error)
	}
} 