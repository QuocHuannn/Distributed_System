package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	shared "Lab02/Shared"
)

type KV struct {
	mu        sync.Mutex
	data      map[string]string
	timestamp map[string]int64
	servers   []shared.ServerInfo
	myInfo    shared.ServerInfo
	primary   *shared.ServerInfo
}

func (kv *KV) Put(args *shared.PutArgs, reply *shared.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Chỉ primary mới được phép ghi
	if !kv.myInfo.IsPrimary {
		reply.Err = shared.NotPrimary
		return nil
	}

	// Cập nhật timestamp
	now := time.Now().UnixNano()
	kv.timestamp[args.Key] = now
	kv.data[args.Key] = args.Value

	// Replicate to backups
	for _, server := range kv.servers {
		if server.ID != kv.myInfo.ID {
			go kv.syncToBackup(server, args.Key, args.Value, now)
		}
	}

	reply.Err = shared.OK
	return nil
}

func (kv *KV) Get(args *shared.GetArgs, reply *shared.GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	val, ok := kv.data[args.Key]
	if ok {
		reply.Err = shared.OK
		reply.Value = val
	} else {
		reply.Err = shared.ErrNoKey
		reply.Value = ""
	}
	return nil
}

func (kv *KV) syncToBackup(server shared.ServerInfo, key, value string, timestamp int64) {
	client, err := rpc.Dial("tcp", server.Address)
	if err != nil {
		log.Printf("[S] Failed to connect to backup %d: %v", server.ID, err)
		go kv.startElection()
		return
	}
	defer client.Close()

	msg := shared.ReplicationMessage{
		Type:      shared.Sync,
		Key:       key,
		Value:     value,
		Timestamp: timestamp,
	}
	var reply shared.PutReply
	err = client.Call("KV.SyncData", &msg, &reply)
	if err != nil {
		log.Printf("[S] Failed to sync with backup %d: %v", server.ID, err)
	}
}

func (kv *KV) SyncData(msg *shared.ReplicationMessage, reply *shared.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	currentTS, exists := kv.timestamp[msg.Key]
	if !exists || currentTS < msg.Timestamp {
		kv.data[msg.Key] = msg.Value
		kv.timestamp[msg.Key] = msg.Timestamp
	}

	reply.Err = shared.OK
	return nil
}

func (kv *KV) startElection() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	for _, server := range kv.servers {
		if server.ID > kv.myInfo.ID {
			go kv.sendElectionMessage(server)
		}
	}

	time.Sleep(2 * time.Second)
	kv.declareVictory()
}

func (kv *KV) sendElectionMessage(server shared.ServerInfo) {
	client, err := rpc.Dial("tcp", server.Address)
	if err != nil {
		return
	}
	defer client.Close()

	msg := shared.ElectionMessage{
		Type:     shared.Election,
		ServerID: kv.myInfo.ID,
	}
	var reply shared.PutReply
	_ = client.Call("KV.HandleElection", &msg, &reply)
}

func (kv *KV) HandleElection(msg *shared.ElectionMessage, reply *shared.PutReply) error {
	if msg.Type == shared.Election {
		go kv.startElection()
		reply.Err = shared.OK
	} else if msg.Type == shared.Victory {
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

	for _, server := range kv.servers {
		if server.ID != kv.myInfo.ID {
			go func(s shared.ServerInfo) {
				client, err := rpc.Dial("tcp", s.Address)
				if err != nil {
					return
				}
				defer client.Close()

				msg := shared.ElectionMessage{
					Type:     shared.Victory,
					ServerID: kv.myInfo.ID,
				}
				var reply shared.PutReply
				_ = client.Call("KV.HandleElection", &msg, &reply)
			}(server)
		}
	}
}

func main() {
	serverID := flag.Int("id", 1, "Server ID")
	port := flag.Int("port", 1234, "Server port")
	flag.Parse()

	serverAddr := fmt.Sprintf(":%d", *port)

	kv := &KV{
		data:      make(map[string]string),
		timestamp: make(map[string]int64),
		myInfo: shared.ServerInfo{
			ID:        *serverID,
			Address:   serverAddr,
			IsPrimary: *serverID == 1, // Server 1 starts as primary
		},
		servers: []shared.ServerInfo{
			{ID: 1, Address: ":1234"},
			{ID: 2, Address: ":1235"},
			{ID: 3, Address: ":1236"},
		},
	}

	rpc.Register(kv)
	listener, err := net.Listen("tcp", serverAddr)
	if err != nil {
		log.Fatal("Listen error:", err)
	}

	fmt.Printf("Server %d starting on %s\n", *serverID, serverAddr)

	if *serverID == 1 {
		kv.primary = &kv.myInfo
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Accept error:", err)
		}
		go rpc.ServeConn(conn)
	}
}
