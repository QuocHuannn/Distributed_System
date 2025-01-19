package main

import (
	kvs "Lab02/Shared"
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

type KV struct {
	mu            sync.Mutex
	data          map[string]string
	ID            int
	Peers         []string
	IsPrimary     bool
	leaderID      int
	primaryAddr   string
	rpcServer     *rpc.Server
	lastHeartBeat time.Time
	Port          string
}

func NewKV(id int, port string, peers []string) *KV {
	kv := &KV{
		ID:            id,
		Port:          port,
		Peers:         peers,
		data:          make(map[string]string),
		leaderID:      -1, // Start with “no known leader”
		lastHeartBeat: time.Now(),
	}

	// OPTIONAL: Make ID=3 the "default" leader
	// So if you run:
	//   go run main.go -id=3 -port=1236
	//   go run main.go -id=2 -port=1235
	//   go run main.go -id=1 -port=1234
	// Then S3 starts as leader. If you kill S3, S2 can take over.
	if id == 3 {
		kv.IsPrimary = true
		kv.leaderID = 3
		kv.primaryAddr = "localhost:" + port
		fmt.Printf("[S%d] I am primary!\n", id)
	}

	return kv
}

// GetStatus returns basic info (ID, Address, IsPrimary) about this server.
func (kv *KV) GetStatus(args *kvs.StatusArgs, reply *kvs.StatusReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	reply.ID = kv.ID
	reply.Address = kv.primaryAddr
	reply.IsPrimary = kv.IsPrimary
	return nil
}

// Put handles write requests (only valid on the primary).
func (kv *KV) Put(args *kvs.PutArgs, reply *kvs.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Reject if not leader
	if !kv.IsPrimary {
		reply.Err = kvs.NotPrimary
		log.Printf("[S%d] PUT rejected (not leader)\n", kv.ID)
		return nil
	}

	// Update local data
	kv.data[args.Key] = args.Value
	reply.Err = kvs.OK
	log.Printf("[S%d] PUT: %s = %s\n", kv.ID, args.Key, args.Value)

	// Sync to backups asynchronously
	go kv.syncDataToPeers(args.Key, args.Value)
	return nil
}

// Get reads a value from local data (any server can respond).
func (kv *KV) Get(args *kvs.GetArgs, reply *kvs.GetReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	val, ok := kv.data[args.Key]
	if ok {
		reply.Err = kvs.OK
		reply.Value = val
		log.Printf("[S%d] GET success: %s = %s\n", kv.ID, args.Key, val)
	} else {
		reply.Err = kvs.ErrNoKey
		reply.Value = ""
		log.Printf("[S%d] GET failed: key=%s not found\n", kv.ID, args.Key)
	}
	return nil
}

// SyncData is the RPC method that backups implement to update their local store.
func (kv *KV) SyncData(args *kvs.SyncArgs, reply *kvs.SyncReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data[args.Key] = args.Value
	reply.Err = kvs.OK
	log.Printf("[S%d] Backup updated: %s = %s\n", kv.ID, args.Key, args.Value)
	return nil
}

// syncDataToPeers sends the updated key/value to all other peers.
func (kv *KV) syncDataToPeers(key, value string) {
	args := kvs.SyncArgs{Key: key, Value: value}
	reply := kvs.SyncReply{}

	for _, peer := range kv.Peers {
		// Skip calling ourselves
		if peer == kv.primaryAddr && kv.IsPrimary {
			continue
		}

		client, err := rpc.Dial("tcp", peer)
		if err != nil {
			log.Printf("[S%d] Error syncing to %s: %v\n", kv.ID, peer, err)
			continue
		}
		err = client.Call("KV.SyncData", &args, &reply)
		client.Close()

		if err == nil {
			log.Printf("[S%d] Sync to backup %s: %s = %s\n", kv.ID, peer, key, value)
		} else {
			log.Printf("[S%d] Error SyncData RPC to backup %s: %v\n", kv.ID, peer, err)
		}
	}
}

// StartElection initiates a Bully election.
func (kv *KV) StartElection() {
	kv.mu.Lock()
	fmt.Printf("[S%d] Start election...\n", kv.ID)
	kv.leaderID = kv.ID
	kv.mu.Unlock()

	receivedResponse := false
	timeout := time.After(5 * time.Second)

	// Send Election RPC to all peers
	for _, peer := range kv.Peers {
		client, err := rpc.Dial("tcp", peer)
		if err != nil {
			continue
		}

		args := kvs.ElectionArgs{CandidateID: kv.ID}
		reply := kvs.ElectionReply{}
		err = client.Call("KV.HandleElection", &args, &reply)
		client.Close()

		if err == nil && reply.Success {
			// Means a higher ID is alive
			receivedResponse = true
		}
	}

	select {
	case <-timeout:
		fmt.Printf("[S%d] Election timeout! Becoming leader.\n", kv.ID)
		kv.BecomeLeader()
	default:
		if !receivedResponse {
			fmt.Printf("[S%d] No higher-ID response. I'm the leader!\n", kv.ID)
			kv.BecomeLeader()
		} else {
			fmt.Printf("[S%d] A higher-ID server responded. Waiting for coordinator...\n", kv.ID)
		}
	}
}

// HandleElection replies "Success" if we have a higher ID than the candidate.
func (kv *KV) HandleElection(args *kvs.ElectionArgs, reply *kvs.ElectionReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if kv.ID > args.CandidateID {
		reply.Success = true
	} else {
		reply.Success = false
	}
	return nil
}

func (kv *KV) BecomeLeader() {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	fmt.Printf("[S%d] => BECOME LEADER!\n", kv.ID)

	kv.IsPrimary = true
	kv.leaderID = kv.ID
	kv.primaryAddr = fmt.Sprintf("localhost:%s", kv.Port)

	// Notify others that we are the new coordinator
	for _, peer := range kv.Peers {
		client, err := rpc.Dial("tcp", peer)
		if err != nil {
			continue
		}
		args := kvs.CoordinatorArgs{NewLeaderID: kv.ID}
		reply := kvs.CoordinatorReply{}
		_ = client.Call("KV.NewCoordinator", &args, &reply)
		client.Close()
	}
}

// NewCoordinator is called by the newly elected leader so everyone updates leader info.
func (kv *KV) NewCoordinator(args *kvs.CoordinatorArgs, reply *kvs.CoordinatorReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Accept the new leader ID unconditionally
	kv.leaderID = args.NewLeaderID
	kv.IsPrimary = (kv.ID == kv.leaderID)
	if kv.IsPrimary {
		kv.primaryAddr = fmt.Sprintf("localhost:%s", kv.Port)
	}

	fmt.Printf("[S%d] Updated leader info: leader = S%d\n", kv.ID, kv.leaderID)
	return nil
}

// sendHeartbeat periodically sends heartbeats if this server is the leader.
func (kv *KV) sendHeartbeat() {
	for {
		time.Sleep(2 * time.Second)

		if kv.IsPrimary {
			for _, peerAddr := range kv.Peers {
				if peerAddr == kv.primaryAddr {
					// Skip ourselves
					continue
				}
				client, err := rpc.Dial("tcp", peerAddr)
				if err != nil {
					continue
				}

				args := kvs.HeartbeatArgs{LeaderID: kv.ID}
				var r kvs.HeartbeatReply
				callErr := client.Call("KV.Heartbeat", &args, &r)
				client.Close()

				if callErr != nil {
					log.Printf("[S%d] Heartbeat RPC error to %s: %v\n", kv.ID, peerAddr, callErr)
				} else if r.OK {
					log.Printf("[S%d] Heartbeat acknowledged by %s\n", kv.ID, peerAddr)
				}
			}
		}
	}
}

// Heartbeat is called by the leader. Follower updates lastHeartBeat & knows the leader’s ID.
func (kv *KV) Heartbeat(args *kvs.HeartbeatArgs, reply *kvs.HeartbeatReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.lastHeartBeat = time.Now()

	// Always accept the new leader ID (can be smaller if the bigger ID died)
	kv.leaderID = args.LeaderID
	kv.IsPrimary = (kv.ID == kv.leaderID)

	log.Printf("[S%d] Received heartbeat from S%d\n", kv.ID, args.LeaderID)
	reply.OK = true
	return nil
}

// monitorLeader checks if the leader has timed out (5 seconds).
func (kv *KV) monitorLeader() {
	for {
		time.Sleep(3 * time.Second)
		kv.mu.Lock()

		if !kv.IsPrimary {
			// If we haven't heard from a leader for > 5s, we assume they're dead
			if time.Since(kv.lastHeartBeat) > 5*time.Second {
				fmt.Printf("[S%d] Leader heartbeat timeout => start election\n", kv.ID)
				kv.mu.Unlock()
				kv.StartElection()
				continue
			}
		}
		kv.mu.Unlock()
	}
}

// startServer sets up the RPC server and spawns the monitoring/heartbeat goroutines.
func (kv *KV) startServer(port string) {
	kv.rpcServer = rpc.NewServer()
	if err := kv.rpcServer.Register(kv); err != nil {
		log.Fatalf("[S%d] RPC register error: %v", kv.ID, err)
	}

	listener, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		log.Fatalf("[S%d] listen error on port %s: %v", kv.ID, port, err)
	}
	defer listener.Close()

	fmt.Printf("[S%d] Listening on port %s\n", kv.ID, port)

	// Launch background goroutines
	go kv.monitorLeader()
	go kv.sendHeartbeat()

	// Serve incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[S%d] Accept error: %v\n", kv.ID, err)
			continue
		}
		go kv.rpcServer.ServeConn(conn)
	}
}

func main() {
	serverID := flag.Int("id", 0, "ID server")
	port := flag.String("port", "1234", "Port server")
	flag.Parse()

	// All known ports in the cluster for demonstration
	allPorts := []string{"1234", "1235", "1236"}

	var peers []string
	for _, p := range allPorts {
		if p != *port {
			peers = append(peers, "localhost:"+p)
		}
	}

	s := NewKV(*serverID, *port, peers)
	s.startServer(*port)
}
