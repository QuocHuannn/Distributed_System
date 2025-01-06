package main

import (
	kvs "Lab01/Shared"
	"fmt"
	kvs "Lab01/Shared"
	"log"
	"net/rpc"
	"strconv"
	"sync"
)

func connect() *rpc.Client {
	client, err := rpc.Dial("tcp", "localhost:1234")
	if err != nil {
		log.Fatal("[C] Dialing error:", err)
	}
	return client
}

func put(key, value string) {
	client := connect()
	args := kvs.PutArgs{Key: key, Value: value}
	reply := kvs.PutReply{}
	err := client.Call("KV.Put", &args, &reply)
	if err != nil {
		log.Fatal("[C] Error: ", err)
	}
	errClose := client.Close()

	if errClose != nil {
		log.Println("[C] Error closing connection:", err)
	}
}

func get(key string) string {
	client := connect()
	args := kvs.GetArgs{Key: key}
	reply := kvs.GetReply{}
	errCall := client.Call("KV.Get", &args, &reply)
	if errCall != nil {
		log.Fatal("[C] Error calling service: ", errCall)
	}
	errClose := client.Close()
	if errClose != nil {
		log.Println("[C] Error closing connection:", errClose)
	}
	return reply.Value
}

func main() {
	key := "24C11058-NGUYEN_THIEN_PHUC"
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			putValue := strconv.Itoa(n)
			fmt.Printf("[C] Calling put(\"%s\",\"%s\")...\n",
				key, putValue)
			put(key, putValue)
		}(i)
	}

	wg.Wait()
	value := get(key)
	fmt.Printf("[C] get (%s): %s\n", key, value)
}
