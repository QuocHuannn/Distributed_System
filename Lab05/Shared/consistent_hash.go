package shared

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// ConsistentHash implements the consistent hashing algorithm
type ConsistentHash struct {
	ring       map[uint32]string // Hash ring
	sortedKeys []uint32          // Sorted keys
	replicas   int               // Number of virtual replicas for each node
	mutex      sync.RWMutex      // Mutex to ensure thread-safety
}

// NewConsistentHash creates a new instance of ConsistentHash
func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		ring:       make(map[uint32]string),
		sortedKeys: []uint32{},
		replicas:   replicas,
	}
}

// Add adds a new node to the hash ring
func (ch *ConsistentHash) Add(node string) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	// Create multiple virtual points for each node for better distribution
	for i := 0; i < ch.replicas; i++ {
		key := ch.hashKey(node + ":" + strconv.Itoa(i))
		ch.ring[key] = node
		ch.sortedKeys = append(ch.sortedKeys, key)
	}
	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

// Remove removes a node from the hash ring
func (ch *ConsistentHash) Remove(node string) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	for i := 0; i < ch.replicas; i++ {
		key := ch.hashKey(node + ":" + strconv.Itoa(i))
		delete(ch.ring, key)
	}

	// Update the sorted keys list
	ch.sortedKeys = []uint32{}
	for k := range ch.ring {
		ch.sortedKeys = append(ch.sortedKeys, k)
	}
	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

// Get returns the node responsible for a specific key
func (ch *ConsistentHash) Get(key string) string {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()

	if len(ch.ring) == 0 {
		return ""
	}

	hashKey := ch.hashKey(key)

	// Find the first position on the hash ring greater than or equal to hashKey
	idx := sort.Search(len(ch.sortedKeys), func(i int) bool {
		return ch.sortedKeys[i] >= hashKey
	})

	// If not found, wrap around to the beginning of the ring
	if idx == len(ch.sortedKeys) {
		idx = 0
	}

	return ch.ring[ch.sortedKeys[idx]]
}

// GetAll returns all nodes in the hash ring
func (ch *ConsistentHash) GetAll() []string {
	ch.mutex.RLock()
	defer ch.mutex.RUnlock()

	nodes := make(map[string]bool)
	for _, node := range ch.ring {
		nodes[node] = true
	}

	result := make([]string, 0, len(nodes))
	for node := range nodes {
		result = append(result, node)
	}
	return result
}

// hashKey creates a hash value for a key
func (ch *ConsistentHash) hashKey(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
} 