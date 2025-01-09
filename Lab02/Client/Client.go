package main

import (
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"sync"

	shared "Lab02/Shared"
)

type Client struct {
	servers []string
	primary string
}

func (c *Client) findPrimary() {
	for _, addr := range c.servers {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			continue
		}
		defer client.Close()

		args := &shared.PutArgs{Key: "test", Value: "test"}
		reply := &shared.PutReply{}

		err = client.Call("KV.Put", args, reply)
		if err != nil {
			continue
		}

		if reply.Err != shared.NotPrimary {
			c.primary = addr
			return
		}
	}
}

func (c *Client) put(key, value string) error {
	if c.primary == "" {
		c.findPrimary()
	}

	client, err := rpc.Dial("tcp", c.primary)
	if err != nil {
		c.primary = ""
		return err
	}
	defer client.Close()

	args := &shared.PutArgs{Key: key, Value: value}
	reply := &shared.PutReply{}

	err = client.Call("KV.Put", args, reply)
	if err != nil {
		c.primary = ""
		return err
	}

	if reply.Err == shared.NotPrimary {
		c.primary = ""
		return fmt.Errorf("not primary")
	}

	return nil
}

func (c *Client) get(key string) (string, error) {
	// Try all servers for GET
	for _, addr := range c.servers {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			continue
		}
		defer client.Close()

		args := &shared.GetArgs{Key: key}
		reply := &shared.GetReply{}

		err = client.Call("KV.Get", args, reply)
		if err != nil {
			continue
		}

		if reply.Err == shared.OK {
			return reply.Value, nil
		}
	}

	return "", fmt.Errorf("no server available")
}

func (c *Client) checkServerStatus() {
	fmt.Println("\n=== Server Status ===")
	for _, addr := range c.servers {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			fmt.Printf("Server %s: Unreachable\n", addr)
			continue
		}
		defer client.Close()

		args := &shared.StatusArgs{}
		reply := &shared.StatusReply{}

		err = client.Call("KV.GetStatus", args, reply)
		if err != nil {
			fmt.Printf("Server %s: Error getting status\n", addr)
			continue
		}

		role := "Backup"
		if reply.IsPrimary {
			role = "Primary"
		}
		fmt.Printf("Server %d (%s): %s\n", reply.ServerID, reply.Address, role)
	}
	fmt.Println("==================\n")
}

func main() {
	client := &Client{
		servers: []string{":1234", ":1235", ":1236"},
	}

	// Check initial status
	client.checkServerStatus()

	key := "24C11058-NGUYEN_THIEN_PHUC"
	var wg sync.WaitGroup

	// Test concurrent puts
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			value := strconv.Itoa(n)
			fmt.Printf("[C] Putting %s = %s\n", key, value)
			err := client.put(key, value)
			if err != nil {
				log.Printf("Put error: %v\n", err)
			}
		}(i)
	}

	wg.Wait()

	// Test get from all servers
	for _, addr := range client.servers {
		value, err := client.get(key)
		if err != nil {
			log.Printf("Get error from %s: %v\n", addr, err)
		} else {
			fmt.Printf("[C] Got %s = %s from %s\n", key, value, addr)
		}
	}

	// Check status after operations
	client.checkServerStatus()
}
