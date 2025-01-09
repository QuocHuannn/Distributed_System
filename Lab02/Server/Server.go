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
	mu                    sync.Mutex
	data                  map[string]string
	timestamp             map[string]int64
	servers               []shared.ServerInfo
	myInfo                shared.ServerInfo
	primary               *shared.ServerInfo
	isParticipatingInRing bool
	nextServerInRing      *shared.ServerInfo
	lastHeartbeat         int64
	primaryAlive          bool
}

type ServerInfo struct {
	ID        int
	Address   string
	IsPrimary bool
	StartTime time.Time // server start
}

func (kv *KV) Put(args *shared.PutArgs, reply *shared.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if !kv.myInfo.IsPrimary || kv.primary == nil || kv.primary.ID != kv.myInfo.ID {
		reply.Err = shared.NotPrimary
		return nil
	}

	now := time.Now().UnixNano()
	kv.timestamp[args.Key] = now
	kv.data[args.Key] = args.Value

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
		kv.mu.Lock()
		defer kv.mu.Unlock()
		kv.primaryAlive = false
		go kv.handlePrimaryFailure()
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
	_ = client.Call("KV.SyncData", &msg, &reply)
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
	if kv.isParticipatingInRing || kv.primaryAlive {
		kv.mu.Unlock()
		return
	}
	kv.isParticipatingInRing = true
	kv.mu.Unlock()

	// Find the server with the lowest ID among the living servers
	lowestID := kv.myInfo.ID
	for _, server := range kv.servers {
		if server.ID < lowestID {
			// Check if the server is alive
			if client, err := rpc.Dial("tcp", server.Address); err == nil {
				client.Close()
				lowestID = server.ID
			}
		}
	}

	// If the current server has the lowest ID, become primary
	if lowestID == kv.myInfo.ID {
		kv.declareVictory()
	}

	kv.isParticipatingInRing = false
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
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if msg.Type == shared.Victory {
		for i := range kv.servers {
			if kv.servers[i].ID == msg.ServerID {
				kv.servers[i].IsPrimary = true
				kv.primary = &kv.servers[i]
				if kv.myInfo.ID != msg.ServerID {
					kv.myInfo.IsPrimary = false
				}
			} else {
				kv.servers[i].IsPrimary = false
			}
		}
	}
	reply.Err = shared.OK
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
				err = client.Call("KV.HandleElection", &msg, &reply)
				if err != nil {
					log.Printf("Error sending Victory to %s: %v", s.Address, err)
				}
			}(server)
		}
	}
}

func (kv *KV) handlePrimaryFailure() {
	kv.mu.Lock()
	if kv.primary != nil && !kv.primaryAlive {
		log.Printf("[S%d] Primary failure confirmed, starting election", kv.myInfo.ID)
		kv.primary = nil // Clear primary reference
		kv.mu.Unlock()

		// Attempt election a few times if necessary
		for i := 0; i < 3; i++ {
			kv.startRingElection()
			time.Sleep(time.Second)

			kv.mu.Lock()
			if kv.primary != nil {
				kv.mu.Unlock()
				return
			}
			kv.mu.Unlock()
		}
	} else {
		kv.mu.Unlock()
	}
}

func (kv *KV) startRingElection() {
	kv.mu.Lock()
	if kv.isParticipatingInRing {
		kv.mu.Unlock()
		return
	}
	kv.isParticipatingInRing = true
	kv.mu.Unlock()

	log.Printf("[S%d] Starting Ring Election", kv.myInfo.ID)

	electionMsg := shared.ElectionMessage{
		Type:     shared.Election,
		ServerID: kv.myInfo.ID,
	}

	// Forward election message to the next server
	go kv.forwardElectionMessage(electionMsg)
}

func (kv *KV) forwardElectionMessage(msg shared.ElectionMessage) {
	// If the message returns to the initiator
	if msg.ServerID == kv.myInfo.ID {
		// Select the server with the lowest ID as the leader
		newLeaderID := kv.myInfo.ID
		for _, server := range kv.servers {
			if server.ID < newLeaderID {
				newLeaderID = server.ID
			}
		}

		log.Printf("[S%d] Election complete. New leader: %d", kv.myInfo.ID, newLeaderID)

		// Broadcast the result
		for _, server := range kv.servers {
			if server.ID != kv.myInfo.ID {
				client, err := rpc.Dial("tcp", server.Address)
				if err != nil {
					continue
				}
				msg := shared.ElectionMessage{
					Type:     shared.Victory,
					ServerID: newLeaderID,
				}
				var reply shared.PutReply
				_ = client.Call("KV.HandleElection", &msg, &reply)
				client.Close()
			}
		}

		// Update local state
		kv.mu.Lock()
		if newLeaderID == kv.myInfo.ID {
			kv.myInfo.IsPrimary = true
			kv.primary = &kv.myInfo
			log.Printf("[S%d] I am now the primary", kv.myInfo.ID)
		}
		kv.isParticipatingInRing = false
		kv.mu.Unlock()
		return
	}

	// Forward to the next server
	client, err := rpc.Dial("tcp", kv.nextServerInRing.Address)
	if err != nil {
		log.Printf("[S%d] Failed to forward to next server, restarting election", kv.myInfo.ID)
		kv.startRingElection()
		return
	}
	defer client.Close()

	var reply shared.PutReply
	err = client.Call("KV.HandleElection", &msg, &reply)
	if err != nil {
		log.Printf("[S%d] Error in forwarding election message: %v", kv.myInfo.ID, err)
		kv.startRingElection()
	}
}

func (kv *KV) GetStatus(args *shared.StatusArgs, reply *shared.StatusReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	reply.IsPrimary = kv.myInfo.IsPrimary
	reply.ServerID = kv.myInfo.ID
	reply.Address = kv.myInfo.Address
	return nil
}

func (kv *KV) checkPrimaryHealth() {
	for {
		kv.mu.Lock()
		if kv.primary != nil && time.Now().UnixNano()-kv.lastHeartbeat > 3e9 {
			kv.primaryAlive = false
			kv.primary = nil
			log.Printf("Primary not responding, starting election...")
			go kv.startElection()
		}
		kv.mu.Unlock()
		time.Sleep(1 * time.Second)
	}
}

func (kv *KV) sendHeartbeat() {
	for {
		if kv.myInfo.IsPrimary {
			for _, server := range kv.servers {
				if server.ID != kv.myInfo.ID {
					client, err := rpc.Dial("tcp", server.Address)
					if err != nil {
						continue
					}
					msg := &shared.HeartbeatMsg{
						PrimaryID: int64(kv.myInfo.ID),
						Timestamp: time.Now().UnixNano(),
					}
					var reply shared.PutReply
					_ = client.Call("KV.ReceiveHeartbeat", msg, &reply)
					client.Close()
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	serverID := flag.Int("id", 1, "Server ID")
	port := flag.Int("port", 1234, "Server port")
	flag.Parse()

	servers := []shared.ServerInfo{
		{ID: 1, Address: ":1234", IsPrimary: false, StartTime: time.Now()},
		{ID: 2, Address: ":1235", IsPrimary: false, StartTime: time.Now()},
		{ID: 3, Address: ":1236", IsPrimary: false, StartTime: time.Now()},
	}

	var myInfo shared.ServerInfo
	for i := range servers {
		if servers[i].ID == *serverID {
			myInfo = servers[i]
			myInfo.StartTime = time.Now()
			servers[i] = myInfo
			break
		}
	}

	kv := &KV{
		data:         make(map[string]string),
		timestamp:    make(map[string]int64),
		primaryAlive: true,
		servers:      servers,
		myInfo:       myInfo,
	}

	// Đợi một chút để cho các server khác có cơ hội khởi động
	time.Sleep(2 * time.Second)

	// Kiểm tra xem có server nào đang chạy với ID thấp hơn không
	hasLowerIDServer := false
	for _, server := range servers {
		if server.ID < myInfo.ID {
			if client, err := rpc.Dial("tcp", server.Address); err == nil {
				client.Close()
				hasLowerIDServer = true
				break
			}
		}
	}

	// Nếu không có server nào có ID thấp hơn đang chạy, trở thành primary
	if !hasLowerIDServer {
		kv.myInfo.IsPrimary = true
		kv.primary = &kv.myInfo
		log.Printf("Server %d becoming primary", myInfo.ID)
	}

	go kv.checkPrimaryHealth()
	go kv.sendHeartbeat()

	rpc.Register(kv)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatal("Listen error:", err)
	}

	log.Printf("Server %d starting on %s", *serverID, listener.Addr())
	rpc.Accept(listener)
}
