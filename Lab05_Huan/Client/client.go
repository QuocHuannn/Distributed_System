package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Operation represents a single operation in a transaction
type Operation struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// Message represents a message exchanged between components
type Message struct {
	Type        string      `json:"type"`
	TxID        string      `json:"tx_id,omitempty"`
	Key         string      `json:"key,omitempty"`
	Value       string      `json:"value,omitempty"`
	Status      string      `json:"status,omitempty"`
	Error       string      `json:"error,omitempty"`
	Timestamp   time.Time   `json:"timestamp"`
	Operations  []Operation `json:"operations,omitempty"`
	Resources   []string    `json:"resources,omitempty"`
}

// Client represents a client that interacts with the distributed system
type Client struct {
	CoordinatorAddr string
	Conn            net.Conn
	Encoder         *json.Encoder
	Decoder         *json.Decoder
	Logger          *log.Logger
}

// NewClient creates a new client instance
func NewClient(coordinatorAddr string) *Client {
	return &Client{
		CoordinatorAddr: coordinatorAddr,
		Logger:          log.New(os.Stdout, "[CLIENT] ", log.LstdFlags),
	}
}

// Connect connects to the coordinator
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.CoordinatorAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator: %v", err)
	}

	c.Conn = conn
	c.Encoder = json.NewEncoder(conn)
	c.Decoder = json.NewDecoder(conn)

	c.Logger.Printf("Connected to coordinator at %s", c.CoordinatorAddr)
	return nil
}

// Close closes the connection
func (c *Client) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

// TransferMoney transfers money from one account to another
func (c *Client) TransferMoney(fromAccount, toAccount string, amount float64) error {
	// Create operations for the transaction
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

	// Send transfer request to coordinator
	msg := Message{
		Type:       "TRANSFER_MONEY",
		Operations: operations,
		Timestamp:  time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to send transfer request: %v", err)
	}

	// Receive response
	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return fmt.Errorf("transfer failed: %s", response.Error)
	}

	c.Logger.Printf("Transfer successful: %s -> %s, amount: %.2f", fromAccount, toAccount, amount)
	return nil
}

// BookTrip books a trip with car, hotel, and flight
func (c *Client) BookTrip(carType string, carDays int, hotelType string, hotelNights int, flightClass string, flightSeats int) error {
	// Create operations for the transaction
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

	// Send booking request to coordinator
	msg := Message{
		Type:       "BOOK_TRIP",
		Operations: operations,
		Timestamp:  time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to send booking request: %v", err)
	}

	// Receive response
	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return fmt.Errorf("booking failed: %s", response.Error)
	}

	c.Logger.Printf("Booking successful: Car(%s, %d days), Hotel(%s, %d nights), Flight(%s, %d seats)",
		carType, carDays, hotelType, hotelNights, flightClass, flightSeats)
	return nil
}

// Get retrieves a value for a key
func (c *Client) Get(key string) (string, error) {
	msg := Message{
		Type:      "GET",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return "", fmt.Errorf("failed to send GET request: %v", err)
	}

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return "", fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return "", fmt.Errorf("GET failed: %s", response.Error)
	}

	return response.Value, nil
}

// Set sets a value for a key
func (c *Client) Set(key, value string) error {
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

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return fmt.Errorf("SET failed: %s", response.Error)
	}

	return nil
}

// Delete deletes a key
func (c *Client) Delete(key string) error {
	msg := Message{
		Type:      "DELETE",
		Key:       key,
		Timestamp: time.Now(),
	}

	err := c.Encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to send DELETE request: %v", err)
	}

	var response Message
	err = c.Decoder.Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to receive response: %v", err)
	}

	if response.Status != "OK" {
		return fmt.Errorf("DELETE failed: %s", response.Error)
	}

	return nil
}

// RunInteractiveMode runs the client in interactive mode
func (c *Client) RunInteractiveMode() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Interactive Client Mode")
	fmt.Println("Commands:")
	fmt.Println("  transfer <from_account> <to_account> <amount>")
	fmt.Println("  book <car_type> <car_days> <hotel_type> <hotel_nights> <flight_class> <flight_seats>")
	fmt.Println("  get <key>")
	fmt.Println("  set <key> <value>")
	fmt.Println("  delete <key>")
	fmt.Println("  exit")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}

		command := args[0]

		switch command {
		case "transfer":
			if len(args) != 4 {
				fmt.Println("Usage: transfer <from_account> <to_account> <amount>")
				continue
			}
			amount, err := strconv.ParseFloat(args[3], 64)
			if err != nil {
				fmt.Printf("Invalid amount: %v\n", err)
				continue
			}
			err = c.TransferMoney(args[1], args[2], amount)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Transfer successful")
			}

		case "book":
			if len(args) != 7 {
				fmt.Println("Usage: book <car_type> <car_days> <hotel_type> <hotel_nights> <flight_class> <flight_seats>")
				continue
			}
			carDays, err := strconv.Atoi(args[2])
			if err != nil {
				fmt.Printf("Invalid car days: %v\n", err)
				continue
			}
			hotelNights, err := strconv.Atoi(args[4])
			if err != nil {
				fmt.Printf("Invalid hotel nights: %v\n", err)
				continue
			}
			flightSeats, err := strconv.Atoi(args[6])
			if err != nil {
				fmt.Printf("Invalid flight seats: %v\n", err)
				continue
			}
			err = c.BookTrip(args[1], carDays, args[3], hotelNights, args[5], flightSeats)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Booking successful")
			}

		case "get":
			if len(args) != 2 {
				fmt.Println("Usage: get <key>")
				continue
			}
			value, err := c.Get(args[1])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Printf("%s = %s\n", args[1], value)
			}

		case "set":
			if len(args) != 3 {
				fmt.Println("Usage: set <key> <value>")
				continue
			}
			err := c.Set(args[1], args[2])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Set successful")
			}

		case "delete":
			if len(args) != 2 {
				fmt.Println("Usage: delete <key>")
				continue
			}
			err := c.Delete(args[1])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println("Delete successful")
			}

		case "exit":
			return

		default:
			fmt.Println("Unknown command")
		}
	}
}

func main() {
	coordinatorAddr := flag.String("coordinator", "localhost:8000", "Coordinator address")
	interactive := flag.Bool("interactive", true, "Run in interactive mode")
	flag.Parse()

	client := NewClient(*coordinatorAddr)
	err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to coordinator: %v", err)
	}
	defer client.Close()

	if *interactive {
		client.RunInteractiveMode()
	} else {
		// Example: Transfer money
		err = client.TransferMoney("account1", "account2", 100.0)
		if err != nil {
			log.Fatalf("Transfer failed: %v", err)
		}

		// Example: Book a trip
		err = client.BookTrip("SUV", 3, "Deluxe", 2, "Economy", 2)
		if err != nil {
			log.Fatalf("Booking failed: %v", err)
		}
	}
} 