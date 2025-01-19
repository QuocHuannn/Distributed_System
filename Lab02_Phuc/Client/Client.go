package main

import (
	kvs "Lab02/Shared"
	"fmt"
	"log"
	"net/rpc"
	"time"
)

// Possible error codes from your server side
// (If you have them in kvs, you can re-use directly.)
const (
	NotPrimaryErr = "NotPrimary"
	OK            = "OK"
)

type Client struct {
	servers []string // All known server addresses (e.g. localhost:1234, etc.)
	primary string   // The current known leader address (or "" if unknown)
}

// TryPut attempts a PUT on the current known leader. If it fails or returns NotPrimary, tries the next server.
func (c *Client) TryPut(key, value string) error {
	// If we have no known primary, or the primary fails, we try to find it.
	if c.primary == "" {
		if err := c.findPrimary(); err != nil {
			return fmt.Errorf("cannot find primary: %v", err)
		}
	}

	// Attempt the PUT on the known primary
	err := c.putOnServer(c.primary, key, value)
	if err != nil {
		// If we got a NotPrimary error or RPC dial error, we’ll try another server
		log.Printf("[Client] PUT failed on %s: %v. Retrying findPrimary...", c.primary, err)

		// reset c.primary, find a new leader
		c.primary = ""
		if err2 := c.findPrimary(); err2 != nil {
			return err2
		}
		// Once we have a new primary, try again
		return c.putOnServer(c.primary, key, value)
	}
	return nil
}

// putOnServer calls the server’s PUT RPC. Returns error if not primary or unreachable.
func (c *Client) putOnServer(serverAddr, key, value string) error {
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		return err // dial failure
	}
	defer client.Close()

	args := &kvs.PutArgs{Key: key, Value: value}
	reply := &kvs.PutReply{}
	callErr := client.Call("KV.Put", args, reply)
	if callErr != nil {
		return callErr
	}

	if reply.Err == kvs.NotPrimary {
		return fmt.Errorf(NotPrimaryErr)
	} else if reply.Err != kvs.OK {
		// Some other error code from server
		return fmt.Errorf("server error: %v", reply.Err)
	}
	// success
	log.Printf("[Client] PUT (%s=%s) succeeded on %s", key, value, serverAddr)
	return nil
}

// findPrimary tries a simple putOnServer test with a dummy key to see who is the leader.
func (c *Client) findPrimary() error {
	dummyKey := "__dummy_key__"
	dummyVal := fmt.Sprintf("t%d", time.Now().UnixNano())

	for _, serverAddr := range c.servers {
		log.Printf("[Client] Checking if %s is primary...", serverAddr)
		err := c.putOnServer(serverAddr, dummyKey, dummyVal)
		if err == nil {
			// success => serverAddr is primary
			c.primary = serverAddr
			log.Printf("[Client] Found primary: %s", serverAddr)
			return nil
		}
		if err.Error() == NotPrimaryErr {
			// This server is alive but not primary, keep searching
			continue
		}
		// If it's a dial error or something else, keep searching
	}
	return fmt.Errorf("all servers refused the dummy PUT or unreachable")
}

// TryGetFromAll attempts GET on all servers to confirm replication.
func (c *Client) TryGetFromAll(key string) {
	for _, serverAddr := range c.servers {
		value, err := c.getFromServer(serverAddr, key)
		if err != nil {
			log.Printf("[Client] GET from %s failed: %v", serverAddr, err)
		} else {
			log.Printf("[Client] GET from %s -> key=%s, value=%s", serverAddr, key, value)
		}
	}
}

// getFromServer calls the server’s GET RPC and returns the value or an error.
func (c *Client) getFromServer(serverAddr, key string) (string, error) {
	client, err := rpc.Dial("tcp", serverAddr)
	if err != nil {
		return "", err
	}
	defer client.Close()

	args := &kvs.GetArgs{Key: key}
	reply := &kvs.GetReply{}
	callErr := client.Call("KV.Get", args, reply)
	if callErr != nil {
		return "", callErr
	}
	if reply.Err != kvs.OK {
		return "", fmt.Errorf("server returned error: %v", reply.Err)
	}
	return reply.Value, nil
}

func main() {
	// Suppose we know the 3 servers:
	client := &Client{
		servers: []string{"localhost:1234", "localhost:1235", "localhost:1236"},
		primary: "", // unknown at start
	}

	// =========== TC01 ===========
	// 1) We do a PUT <key, value> to the primary
	key := "color"
	value := "blue"

	log.Println("=== TC01: PUT/GET with all 3 servers running ===")
	err := client.TryPut(key, value)
	if err != nil {
		log.Fatalf("PUT error: %v", err)
	}

	// 2) Confirm replication by GET from all servers
	client.TryGetFromAll(key)

	// =========== TC02 ===========
	// 1) We assume we kill the primary server externally (Ctrl+C on e.g. ID=1).
	// 2) Wait a few seconds for election
	log.Println("=== TC02: Simulate primary failure, wait 10s, do new PUT/GET ===")
	time.Sleep(10 * time.Second)

	// 3) Try a new PUT => should discover the new leader
	key2 := "drink"
	val2 := "coffee"
	err = client.TryPut(key2, val2)
	if err != nil {
		log.Fatalf("PUT error after primary failure: %v", err)
	}

	// 4) Try GET from all servers => they should all have the new value
	client.TryGetFromAll(key2)

	// Done
	log.Println("[Client] Testcases complete. Check logs for results.")
}
