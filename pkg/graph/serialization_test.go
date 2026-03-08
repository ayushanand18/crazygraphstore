
package graph

import (
	"bytes"
	"testing"
)

func TestSerializer_SerializeNode_RoundTrip(t *testing.T) {
	serializer := NewSerializer()
	
	// Create node with various properties
	node := NewNode("test-node-1", []string{"Person", "User"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))
	node.SetProperty("score", float64(95.5))
	node.SetProperty("active", true)
	node.SetProperty("data", []byte("binary data"))
	
	// Serialize
	data, err := serializer.SerializeNode(node)
	if err != nil {
		t.Fatalf("Failed to serialize node: %v", err)
	}
	
	if len(data) == 0 {
		t.Fatal("Serialized data is empty")
	}
	
	// Deserialize
	deserialized, err := serializer.DeserializeNode(data)
	if err != nil {
		t.Fatalf("Failed to deserialize node: %v", err)
	}
	
	// Verify fields
	if deserialized.ID != node.ID {
		t.Errorf("ID mismatch: got %s, want %s", deserialized.ID, node.ID)
	}
	
	if len(deserialized.Labels) != len(node.Labels) {
		t.Errorf("Labels count mismatch: got %d, want %d", len(deserialized.Labels), len(node.Labels))
	}
	
	for i, label := range node.Labels {
		if deserialized.Labels[i] != label {
			t.Errorf("Label %d mismatch: got %s, want %s", i, deserialized.Labels[i], label)
		}
	}
	
	// Verify properties
	if deserialized.GetProperty("name") != "Alice" {
		t.Errorf("Property 'name' mismatch: got %v", deserialized.GetProperty("name"))
	}
	
	if deserialized.GetProperty("age") != int64(30) {
		t.Errorf("Property 'age' mismatch: got %v", deserialized.GetProperty("age"))
	}
	
	if deserialized.GetProperty("score") != float64(95.5) {
		t.Errorf("Property 'score' mismatch: got %v", deserialized.GetProperty("score"))
	}
	
	if deserialized.GetProperty("active") != true {
		t.Errorf("Property 'active' mismatch: got %v", deserialized.GetProperty("active"))
	}
	
	dataBytes := deserialized.GetProperty("data").([]byte)
	if !bytes.Equal(dataBytes, []byte("binary data")) {
		t.Errorf("Property 'data' mismatch: got %v", dataBytes)
	}
}

func TestSerializer_SerializeEdge_RoundTrip(t *testing.T) {
	serializer := NewSerializer()
	
	// Create edge with properties
	edge := NewEdge("edge-1", "node-1", "node-2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	edge.SetProperty("weight", float64(0.8))
	edge.SetProperty("verified", true)
	
	// Serialize
	data, err := serializer.SerializeEdge(edge)
	if err != nil {
		t.Fatalf("Failed to serialize edge: %v", err)
	}
	
	if len(data) == 0 {
		t.Fatal("Serialized data is empty")
	}
	
	// Deserialize
	deserialized, err := serializer.DeserializeEdge(data)
	if err != nil {
		t.Fatalf("Failed to deserialize edge: %v", err)
	}
	
	// Verify fields
	if deserialized.ID != edge.ID {
		t.Errorf("ID mismatch: got %s, want %s", deserialized.ID, edge.ID)
	}
	
	if deserialized.FromNodeID != edge.FromNodeID {
		t.Errorf("FromNodeID mismatch: got %s, want %s", deserialized.FromNodeID, edge.FromNodeID)
	}
	
	if deserialized.ToNodeID != edge.ToNodeID {
		t.Errorf("ToNodeID mismatch: got %s, want %s", deserialized.ToNodeID, edge.ToNodeID)
	}
	
	if deserialized.Type != edge.Type {
		t.Errorf("Type mismatch: got %s, want %s", deserialized.Type, edge.Type)
	}
	
	// Verify properties
	if deserialized.Properties["since"] != int64(2020) {
		t.Errorf("Property 'since' mismatch: got %v", deserialized.Properties["since"])
	}
	
	if deserialized.Properties["weight"] != float64(0.8) {
		t.Errorf("Property 'weight' mismatch: got %v", deserialized.Properties["weight"])
	}
	
	if deserialized.Properties["verified"] != true {
		t.Errorf("Property 'verified' mismatch: got %v", deserialized.Properties["verified"])
	}
}

func TestSerializer_EmptyNode(t *testing.T) {
	serializer := NewSerializer()
	
	// Node with no labels or properties
	node := NewNode("empty-node", []string{})
	
	data, err := serializer.SerializeNode(node)
	if err != nil {
		t.Fatalf("Failed to serialize empty node: %v", err)
	}
	
	deserialized, err := serializer.DeserializeNode(data)
	if err != nil {
		t.Fatalf("Failed to deserialize empty node: %v", err)
	}
	
	if deserialized.ID != node.ID {
		t.Errorf("ID mismatch for empty node")
	}
	
	if len(deserialized.Labels) != 0 {
		t.Errorf("Expected no labels, got %d", len(deserialized.Labels))
	}
	
	if len(deserialized.Properties) != 0 {
		t.Errorf("Expected no properties, got %d", len(deserialized.Properties))
	}
}

func TestSerializer_EmptyEdge(t *testing.T) {
	serializer := NewSerializer()
	
	// Edge with no properties
	edge := NewEdge("empty-edge", "n1", "n2", "LINKS_TO")
	
	data, err := serializer.SerializeEdge(edge)
	if err != nil {
		t.Fatalf("Failed to serialize empty edge: %v", err)
	}
	
	deserialized, err := serializer.DeserializeEdge(data)
	if err != nil {
		t.Fatalf("Failed to deserialize empty edge: %v", err)
	}
	
	if deserialized.ID != edge.ID {
		t.Errorf("ID mismatch for empty edge")
	}
	
	if len(deserialized.Properties) != 0 {
		t.Errorf("Expected no properties, got %d", len(deserialized.Properties))
	}
}

func TestSerializer_LargeNode(t *testing.T) {
	serializer := NewSerializer()
	
	node := NewNode("large-node", []string{"Type1", "Type2", "Type3"})
	
	// Add many properties
	for i := 0; i < 100; i++ {
		node.SetProperty("prop"+string(rune(i)), int64(i))
	}
	
	// Add large binary data
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	node.SetProperty("largeData", largeData)
	
	// Serialize and deserialize
	data, err := serializer.SerializeNode(node)
	if err != nil {
		t.Fatalf("Failed to serialize large node: %v", err)
	}
	
	deserialized, err := serializer.DeserializeNode(data)
	if err != nil {
		t.Fatalf("Failed to deserialize large node: %v", err)
	}
	
	if deserialized.ID != node.ID {
		t.Errorf("ID mismatch for large node")
	}
	
	// Verify large data
	deserializedData := deserialized.GetProperty("largeData").([]byte)
	if !bytes.Equal(deserializedData, largeData) {
		t.Error("Large binary data mismatch")
	}
}

func TestSerializer_SpecialCharacters(t *testing.T) {
	serializer := NewSerializer()
	
	// Test with special characters and unicode
	node := NewNode("node-🎉", []string{"类型", "тип"})
	node.SetProperty("name", "Alice 👩‍💻")
	node.SetProperty("description", "Special chars: \n\t\r\"'\\")
	
	data, err := serializer.SerializeNode(node)
	if err != nil {
		t.Fatalf("Failed to serialize node with special chars: %v", err)
	}
	
	deserialized, err := serializer.DeserializeNode(data)
	if err != nil {
		t.Fatalf("Failed to deserialize node with special chars: %v", err)
	}
	
	if deserialized.ID != node.ID {
		t.Errorf("ID mismatch for special chars node")
	}
	
	if deserialized.GetProperty("name") != "Alice 👩‍💻" {
		t.Errorf("Unicode property mismatch")
	}
}

func TestSerializer_InvalidData(t *testing.T) {
	serializer := NewSerializer()
	
	// Try to deserialize garbage data
	invalidData := []byte{0x00, 0x01, 0x02, 0x03}
	
	_, err := serializer.DeserializeNode(invalidData)
	if err == nil {
		t.Error("Expected error when deserializing invalid data, got nil")
	}
	
	_, err = serializer.DeserializeEdge(invalidData)
	if err == nil {
		t.Error("Expected error when deserializing invalid edge data, got nil")
	}
}

func BenchmarkSerializer_SerializeNode(b *testing.B) {
	serializer := NewSerializer()
	node := NewNode("bench-node", []string{"Person", "User"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))
	node.SetProperty("score", float64(95.5))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.SerializeNode(node)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSerializer_DeserializeNode(b *testing.B) {
	serializer := NewSerializer()
	node := NewNode("bench-node", []string{"Person", "User"})
	node.SetProperty("name", "Alice")
	node.SetProperty("age", int64(30))
	
	data, _ := serializer.SerializeNode(node)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.DeserializeNode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSerializer_SerializeEdge(b *testing.B) {
	serializer := NewSerializer()
	edge := NewEdge("bench-edge", "node1", "node2", "KNOWS")
	edge.SetProperty("since", int64(2020))
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.SerializeEdge(edge)
		if err != nil {
			b.Fatal(err)
		}
	}
}
