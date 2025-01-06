package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"

	kvs "Golang/Lab01/Shared"
)

type KV struct {
	mu   sync.Mutex
	data map[string]string
}

func (kv *KV) Put(args *kvs.PutArgs, reply *kvs.PutReply) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.data[args.Key] = args.Value
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
	stop := make(chan struct{}) // Kênh để gửi tín hiệu dừng

	go func() {
		for {
			select {
			case <-stop:
				// Dừng lắng nghe khi nhận tín hiệu
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

	// Dừng server khi người dùng nhấn Enter
	fmt.Println("Press Enter to stop the server...")
	fmt.Scanln()

	close(stop) // Gửi tín hiệu dừng
	wg.Wait()   // Đợi tất cả goroutines hoàn thành
	fmt.Println("[S] Server has shut down.")
}
func main() {
	server()
}
