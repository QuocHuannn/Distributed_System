package shared

import (
	"fmt"
	"testing"
)

// TestAddNode tests adding a node to the consistent hash
func TestAddNode(t *testing.T) {
	ch := NewConsistentHash(3)
	
	// Add first node
	ch.Add("node1")
	if len(ch.GetAll()) != 1 {
		t.Errorf("Incorrect number of nodes after adding. Expected: 1, Got: %d", len(ch.GetAll()))
	}
	
	// Add second node
	ch.Add("node2")
	if len(ch.GetAll()) != 2 {
		t.Errorf("Incorrect number of nodes after adding. Expected: 2, Got: %d", len(ch.GetAll()))
	}
	
	// Check number of virtual nodes
	if len(ch.sortedKeys) != 6 { // 2 nodes * 3 replicas
		t.Errorf("Incorrect number of virtual nodes. Expected: 6, Got: %d", len(ch.sortedKeys))
	}
	
	fmt.Println("TC01: AddNode - PASSED")
}

// TestRemoveNode tests removing a node from the consistent hash
func TestRemoveNode(t *testing.T) {
	ch := NewConsistentHash(3)
	
	// Add 3 nodes
	ch.Add("node1")
	ch.Add("node2")
	ch.Add("node3")
	
	if len(ch.GetAll()) != 3 {
		t.Errorf("Incorrect number of nodes after adding. Expected: 3, Got: %d", len(ch.GetAll()))
	}
	
	// Remove node2
	ch.Remove("node2")
	
	if len(ch.GetAll()) != 2 {
		t.Errorf("Incorrect number of nodes after removal. Expected: 2, Got: %d", len(ch.GetAll()))
	}
	
	// Check node2 is removed
	nodes := ch.GetAll()
	for _, node := range nodes {
		if node == "node2" {
			t.Errorf("Node2 still exists after removal")
		}
	}
	
	// Check number of virtual nodes
	if len(ch.sortedKeys) != 6 { // 2 nodes * 3 replicas
		t.Errorf("Incorrect number of virtual nodes. Expected: 6, Got: %d", len(ch.sortedKeys))
	}
	
	fmt.Println("TC02: RemoveNode - PASSED")
}

// TestGetNodeId tests getting the responsible node for a key
func TestGetNodeId(t *testing.T) {
	ch := NewConsistentHash(3)
	
	// Add 3 nodes
	ch.Add("node1")
	ch.Add("node2")
	ch.Add("node3")
	
	// Test some keys
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	
	for _, key := range keys {
		node := ch.Get(key)
		if node == "" {
			t.Errorf("No node found for key %s", key)
		}
		fmt.Printf("Key %s is assigned to node %s\n", key, node)
	}
	
	// Test consistency
	node1 := ch.Get("testkey")
	node2 := ch.Get("testkey")
	
	if node1 != node2 {
		t.Errorf("Inconsistent when getting node for the same key. Got: %s and %s", node1, node2)
	}
	
	fmt.Println("TC03: GetNodeId - PASSED")
}

// TestDistribution tests the distribution of keys
func TestDistribution(t *testing.T) {
	ch := NewConsistentHash(10)
	
	// Add 5 nodes
	nodes := []string{"node1", "node2", "node3", "node4", "node5"}
	for _, node := range nodes {
		ch.Add(node)
	}
	
	// Create 1000 keys and check distribution
	distribution := make(map[string]int)
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		node := ch.Get(key)
		distribution[node]++
	}
	
	// Print distribution
	fmt.Println("Distribution of 1000 keys:")
	for node, count := range distribution {
		fmt.Printf("%s: %d keys (%.2f%%)\n", node, count, float64(count)/10.0)
	}
	
	// Check each node has at least some keys
	for _, node := range nodes {
		if distribution[node] == 0 {
			t.Errorf("Node %s was not assigned any keys", node)
		}
	}
}

// TestConsistencyAfterNodeChange tests consistency after changing nodes
func TestConsistencyAfterNodeChange(t *testing.T) {
	ch := NewConsistentHash(10)
	
	// Add 3 initial nodes
	ch.Add("node1")
	ch.Add("node2")
	ch.Add("node3")
	
	// Create 100 keys and store responsible node
	keyToNodeMap := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		keyToNodeMap[key] = ch.Get(key)
	}
	
	// Add new node
	ch.Add("node4")
	
	// Check how many keys changed node
	changedKeys := 0
	for key, originalNode := range keyToNodeMap {
		newNode := ch.Get(key)
		if originalNode != newNode {
			changedKeys++
		}
	}
	
	// Change rate should be less than 50%
	changeRate := float64(changedKeys) / 100.0
	fmt.Printf("Key change rate after adding new node: %.2f%%\n", changeRate*100)
	
	if changeRate > 0.5 {
		t.Errorf("Change rate too high: %.2f%%", changeRate*100)
	}
	
	// Remove a node
	ch.Remove("node2")
	
	// Check how many keys changed node compared to initial state
	changedKeys = 0
	for key, originalNode := range keyToNodeMap {
		newNode := ch.Get(key)
		if originalNode != newNode {
			changedKeys++
		}
	}
	
	// Change rate should be less than 70%
	changeRate = float64(changedKeys) / 100.0
	fmt.Printf("Key change rate after adding and removing node: %.2f%%\n", changeRate*100)
	
	if changeRate > 0.7 {
		t.Errorf("Change rate too high: %.2f%%", changeRate*100)
	}
} 