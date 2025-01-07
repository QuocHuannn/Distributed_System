package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	kvs "Lab01/Shared"
)

type KV struct {
	mu        sync.Mutex
	data      map[string]string
	timestamp map[string]int64
	servers   []kvs.ServerInfo
	myInfo    kvs.ServerInfo
	primary   *kvs.ServerInfo
}

func (kv *KV) Put(args *kvs.PutArgs, reply *kvs.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Only primary can write
	if !kv.myInfo.IsPrimary {
		reply.Err = "NotPrimary"
		return nil
	}

	// Update timestamp
	now := time.Now().UnixNano()
	kv.timestamp[args.Key] = now
	kv.data[args.Key] = args.Value

	// Replicate to backups
	for _, server := range kv.servers {
		if server.ID != kv.myInfo.ID {
			go kv.syncToBackup(server, args.Key, args.Value, now)
		}
	}

	reply.Err = kvs.OK
	return nil
}

func (kv *KV) syncToBackup(server kvs.ServerInfo, key, value string, timestamp int64) {
	client, err := rpc.Dial("tcp", server.Address)
	if err != nil {
		log.Printf("[S] Failed to connect to backup %d: %v", server.ID, err)
		go kv.startElection() // Start election if backup is down
		return
	}
	defer client.Close()

	msg := kvs.ReplicationMessage{
		Type:      kvs.Sync,
		Key:       key,
		Value:     value,
		Timestamp: timestamp,
	}
	var reply kvs.PutReply
	err = client.Call("KV.SyncData", &msg, &reply)
	if err != nil {
		log.Printf("[S] Failed to sync with backup %d: %v", server.ID, err)
	}
}

func (kv *KV) SyncData(msg *kvs.ReplicationMessage, reply *kvs.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Check timestamp to ensure sequential consistency
	currentTS, exists := kv.timestamp[msg.Key]
	if !exists || currentTS < msg.Timestamp {
		kv.data[msg.Key] = msg.Value
		kv.timestamp[msg.Key] = msg.Timestamp
	}

	reply.Err = kvs.OK
	return nil
}

func (kv *KV) Get(args *kvs.GetArgs, reply *kvs.GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	val, ok := kv.data[args.Key]
	if ok {
		reply.Err = kvs.OK
		reply.Value = val
	} else {
		reply.Err = kvs.ErrNoKey
		reply.Value = ""
	}
	return nil
}

func getAvailablePort(preferredPort string) string {
	listener, err := net.Listen("tcp", preferredPort)
	if err != nil {
		fmt.Printf("Port %s is already in use. Choosing a different port...\n", preferredPort)
		for port := 1235; port <= 1300; port++ {
			altPort := fmt.Sprintf(":%d", port)
			listener, err = net.Listen("tcp", altPort)
			if err == nil {
				err := listener.Close()
				if err != nil {
					return ""
				}
				return altPort
			}
		}
		log.Fatal("No available port found in the range 1235-1300")
	}
	defer listener.Close()
	return preferredPort
}

func server() {
	port := getAvailablePort(":1234")
	fmt.Printf("[S] Using port %s\n", port)

	kv := new(KV)
	kv.data = make(map[string]string)
	newServer := rpc.NewServer()
	errRegister := newServer.Register(kv)
	if errRegister != nil {
		log.Fatal("[S] Error registering RPC server:", errRegister)
	}

	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal("[S] Listen error:", err)
	}
	defer listener.Close()

	fmt.Println("[S] Wait for connections...")

	var count int
	var wg sync.WaitGroup
	stop := make(chan struct{}) // Channel to send stop signal

	go func() {
		for {
			select {
			case <-stop:
				// Stop listening when signal received
				fmt.Println("[S] Stopping server...")
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					log.Println("[S] Connection error:", err)
					continue
				}

				count++
				fmt.Printf("[S] Handle connection %d.\n", count)
				wg.Add(1)

				go func(c net.Conn) {
					defer wg.Done()
					newServer.ServeConn(c)
					c.Close()
				}(conn)
			}
		}
	}()

	// Stop server when user presses Enter
	fmt.Println("Press Enter to stop the server...")
	fmt.Scanln()

	close(stop) // Send stop signal
	wg.Wait()   // Wait for all goroutines to complete
	fmt.Println("[S] Server has shut down.")
}

func (kv *KV) startElection() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Send Election message to servers with higher IDs
	for _, server := range kv.servers {
		if server.ID > kv.myInfo.ID {
			go kv.sendElectionMessage(server)
		}
	}

	// Wait for responses for a period of time
	time.Sleep(2 * time.Second)

	// If no response received, declare victory
	kv.declareVictory()
}

func (kv *KV) sendElectionMessage(server kvs.ServerInfo) {
	client, err := rpc.Dial("tcp", server.Address)
	if err != nil {
		return
	}
	defer client.Close()

	msg := kvs.ElectionMessage{
		Type:     kvs.Election,
		ServerID: kv.myInfo.ID,
	}
	var reply kvs.PutReply
	_ = client.Call("KV.HandleElection", &msg, &reply)
}

func (kv *KV) HandleElection(msg *kvs.ElectionMessage, reply *kvs.PutReply) error {
	if msg.Type == kvs.Election {
		// If Election message received, start new election
		go kv.startElection()
		reply.Err = kvs.OK
	} else if msg.Type == kvs.Victory {
		// Update new primary
		kv.mu.Lock()
		for i := range kv.servers {
			if kv.servers[i].ID == msg.ServerID {
				kv.servers[i].IsPrimary = true
				kv.primary = &kv.servers[i]
			} else {
				kv.servers[i].IsPrimary = false
			}
		}
		kv.mu.Unlock()
	}
	return nil
}

func (kv *KV) declareVictory() {
	kv.myInfo.IsPrimary = true
	kv.primary = &kv.myInfo

	// Notify all other servers
	for _, server := range kv.servers {
		if server.ID != kv.myInfo.ID {
			go func(s kvs.ServerInfo) {
				client, err := rpc.Dial("tcp", s.Address)
				if err != nil {
					return
				}
				defer client.Close()

				msg := kvs.ElectionMessage{
					Type:     kvs.Victory,
					ServerID: kv.myInfo.ID,
				}
				var reply kvs.PutReply
				_ = client.Call("KV.HandleElection", &msg, &reply)
			}(server)
		}
	}
}

func main() {
	server()
}
