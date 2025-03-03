package Server

import (
	kvs "Lab04/Shared"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
)

// KVServer handles key-value storage
type KVServer struct {
	mu   sync.Mutex
	data map[string]string
}

// NewKVServer creates a new KVServer
func NewKVServer() *KVServer {
	return &KVServer{
		data: make(map[string]string),
	}
}

// Set sets a key-value pair
func (kv *KVServer) Set(args *kvs.SetArgs, reply *kvs.SetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.data[args.Key] = args.Value
	log.Printf("Set %s: %s", args.Key, args.Value)
	return nil
}

// Get gets the value for a key
func (kv *KVServer) Get(args *kvs.GetArgs, reply *kvs.GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	value, exists := kv.data[args.Key]
	if !exists {
		return fmt.Errorf("key %s not found", args.Key)
	}
	reply.Value = value
	log.Printf("Get %s: %s", args.Key, value)
	return nil
}

// StartServer starts the RPC server
func StartServer(addr string) {
	kv := NewKVServer()
	rpc.Register(kv)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	log.Printf("Server started at %s", addr)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}