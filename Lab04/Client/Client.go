package Client

import (
	kvs "Lab04/Shared"
	"fmt"
	// "log"
	"net/rpc"
	"sync"
	"Lab04/Hash"
)

// Client handles interactions with the KVServer
type Client struct {
	mu      sync.Mutex
	servers []string
	hr      *Hash.HashRing
}

// NewClient creates a new client
func NewClient(servers []string, replicas int) *Client {
	return &Client{
		servers: servers,
		hr:      Hash.NewHashRing(servers, replicas),
	}
}

// AddNode adds a new server
func (c *Client) AddNode(serverAddr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hr.AddNode(serverAddr)
	c.servers = append(c.servers, serverAddr)
	return nil
}

// RemoveNode removes a server
func (c *Client) RemoveNode(serverAddr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hr.RemoveNode(serverAddr)
	for i, s := range c.servers {
		if s == serverAddr {
			c.servers = append(c.servers[:i], c.servers[i+1:]...)
			break
		}
	}
	return nil
}

// GetNodeId gets the server address for a key
func (c *Client) GetNodeId(key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	server := c.hr.GetServer(key)
	if server == "" {
		return "", fmt.Errorf("no server available")
	}
	return server, nil
}

// Set sets a key-value pair
func (c *Client) Set(key, value string) error {
	server := c.hr.GetServer(key)
	if server == "" {
		return fmt.Errorf("no server available")
	}
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		return err
	}
	defer client.Close()
	args := &kvs.SetArgs{Key: key, Value: value}
	reply := &kvs.SetReply{}
	return client.Call("KVServer.Set", args, reply)
}

// Get gets the value for a key
func (c *Client) Get(key string) (string, error) {
	server := c.hr.GetServer(key)
	if server == "" {
		return "", fmt.Errorf("no server available")
	}
	client, err := rpc.Dial("tcp", server)
	if err != nil {
		return "", err
	}
	defer client.Close()
	args := &kvs.GetArgs{Key: key}
	reply := &kvs.GetReply{}
	err = client.Call("KVServer.Get", args, reply)
	if err != nil {
		return "", err
	}
	return reply.Value, nil
}
