package Hash

import (
	"fmt"
	"sort"
	"hash/fnv"
	"sync"
)

type HashRing struct {
	servers   []string
	replicas  int
	ring      []uint32
	hashToServer map[uint32]string
	mu        sync.Mutex
}

func NewHashRing(servers []string, replicas int) *HashRing {
	hr := &HashRing{
		servers:   servers,
		replicas:  replicas,
		ring:      make([]uint32, 0),
		hashToServer: make(map[uint32]string),
	}
	hr.buildRing()
	return hr
}

func (hr *HashRing) buildRing() {
	hr.ring = make([]uint32, 0)
	hr.hashToServer = make(map[uint32]string)

	for _, server := range hr.servers {
		for i := 0; i < hr.replicas; i++ {
			key := fmt.Sprintf("%s-%d", server, i)
			hash := hashFn(key)
			hr.ring = append(hr.ring, hash)
			hr.hashToServer[hash] = server
		}
	}
	sort.Slice(hr.ring, func(i, j int) bool {
		return hr.ring[i] < hr.ring[j]
	})
}

// Thêm node mới vào ring
func (hr *HashRing) AddNode(node string) {
	hr.servers = append(hr.servers, node)
	hr.buildRing()
}

// Xóa node khỏi ring
func (hr *HashRing) RemoveNode(server string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	for i, s := range hr.servers {
		if s == server {
			hr.servers = append(hr.servers[:i], hr.servers[i+1:]...)
			break
		}
	}
	hr.buildRing()
}

// trả về các server available
func (hr *HashRing) GetServer(key string) string {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	if len(hr.ring) == 0 {
		return ""
	}
	hash := hashFn(key)
	idx := sort.Search(len(hr.ring), func(i int) bool { return hr.ring[i] >= hash })
	if idx == len(hr.ring) {
		idx = 0
	}
	return hr.hashToServer[hr.ring[idx]]
}

// hashFn trả về giá trị hash
func hashFn(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
	