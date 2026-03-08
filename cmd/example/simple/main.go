// Package main demonstrates basic usage of the storage engine.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/storage"
)

func main() {
	fmt.Println("=== Storage Engine for Graph Database - Example ===\n")

	// Create storage engine with default config
	config := storage.DefaultConfig()
	fmt.Printf("Creating engine with %d write lanes, %d MB memtable size\n", 
		config.NumWriteLanes, config.MemtableSize/(1024*1024))
	
	engine, err := storage.NewEngine(config)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Start the engine
	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}
	defer engine.Stop()

	ctx := context.Background()

	fmt.Println("\n1. Creating nodes...")
	
	// Create a Person node
	alice := graph.NewNode("alice-123", []string{"Person"})
	alice.SetProperty("name", "Alice")
	alice.SetProperty("age", int64(30))
	alice.SetProperty("city", "New York")
	
	if err := engine.CreateNode(ctx, alice); err != nil {
		log.Fatalf("Failed to create Alice node: %v", err)
	}
	fmt.Printf("   Created node: %s [%v] with properties: %v\n", alice.ID, alice.Labels, alice.Properties)

	// Create another Person node
	bob := graph.NewNode("bob-456", []string{"Person"})
	bob.SetProperty("name", "Bob")
	bob.SetProperty("age", int64(25))
	bob.SetProperty("city", "San Francisco")
	
	if err := engine.CreateNode(ctx, bob); err != nil {
		log.Fatalf("Failed to create Bob node: %v", err)
	}
	fmt.Printf("   Created node: %s [%v] with properties: %v\n", bob.ID, bob.Labels, bob.Properties)

	// Create a Company node
	company := graph.NewNode("company-789", []string{"Company"})
	company.SetProperty("name", "TechCorp")
	company.SetProperty("industry", "Technology")
	company.SetProperty("founded", int64(2010))
	
	if err := engine.CreateNode(ctx, company); err != nil {
		log.Fatalf("Failed to create Company node: %v", err)
	}
	fmt.Printf("   Created node: %s [%v] with properties: %v\n", company.ID, company.Labels, company.Properties)

	fmt.Println("\n2. Reading nodes...")
	
	// Read Alice back
	retrievedAlice, err := engine.GetNode(ctx, "alice-123")
	if err != nil {
		log.Fatalf("Failed to get Alice: %v", err)
	}
	fmt.Printf("   Retrieved: %s, name=%v, age=%v\n", 
		retrievedAlice.ID, 
		retrievedAlice.GetProperty("name"),
		retrievedAlice.GetProperty("age"))

	fmt.Println("\n3. Creating edges...")
	
	// Alice KNOWS Bob
	knowsEdge := graph.NewEdge("edge-1", alice.ID, bob.ID, "KNOWS")
	knowsEdge.SetProperty("since", int64(2020))
	knowsEdge.SetProperty("strength", 0.8)
	
	if err := engine.CreateEdge(ctx, knowsEdge); err != nil {
		log.Fatalf("Failed to create KNOWS edge: %v", err)
	}
	fmt.Printf("   Created edge: %s --[%s]--> %s with properties: %v\n",
		knowsEdge.FromNodeID, knowsEdge.Type, knowsEdge.ToNodeID, knowsEdge.Properties)

	// Alice WORKS_AT TechCorp
	worksAtEdge := graph.NewEdge("edge-2", alice.ID, company.ID, "WORKS_AT")
	worksAtEdge.SetProperty("role", "Engineer")
	worksAtEdge.SetProperty("since", int64(2015))
	
	if err := engine.CreateEdge(ctx, worksAtEdge); err != nil {
		log.Fatalf("Failed to create WORKS_AT edge: %v", err)
	}
	fmt.Printf("   Created edge: %s --[%s]--> %s with properties: %v\n",
		worksAtEdge.FromNodeID, worksAtEdge.Type, worksAtEdge.ToNodeID, worksAtEdge.Properties)

	// Bob WORKS_AT TechCorp
	bobWorksAtEdge := graph.NewEdge("edge-3", bob.ID, company.ID, "WORKS_AT")
	bobWorksAtEdge.SetProperty("role", "Designer")
	bobWorksAtEdge.SetProperty("since", int64(2018))
	
	if err := engine.CreateEdge(ctx, bobWorksAtEdge); err != nil {
		log.Fatalf("Failed to create Bob's WORKS_AT edge: %v", err)
	}
	fmt.Printf("   Created edge: %s --[%s]--> %s with properties: %v\n",
		bobWorksAtEdge.FromNodeID, bobWorksAtEdge.Type, bobWorksAtEdge.ToNodeID, bobWorksAtEdge.Properties)

	fmt.Println("\n4. Traversing graph...")
	
	// Get Alice's outgoing edges
	aliceOutgoing, err := engine.GetOutgoingEdges(ctx, alice.ID)
	if err != nil {
		log.Fatalf("Failed to get Alice's outgoing edges: %v", err)
	}
	fmt.Printf("   Alice has %d outgoing edges:\n", len(aliceOutgoing))
	for _, edge := range aliceOutgoing {
		fmt.Printf("      - %s --%s--> %s\n", edge.FromNodeID, edge.Type, edge.ToNodeID)
	}

	// Get Alice's neighbors
	aliceNeighbors, err := engine.GetNeighbors(ctx, alice.ID, graph.DirectionOut)
	if err != nil {
		log.Fatalf("Failed to get Alice's neighbors: %v", err)
	}
	fmt.Printf("   Alice's neighbors (outgoing): %d nodes\n", len(aliceNeighbors))
	for _, neighbor := range aliceNeighbors {
		fmt.Printf("      - %s %v (name: %v)\n", neighbor.ID, neighbor.Labels, neighbor.GetProperty("name"))
	}

	// Get company's incoming edges
	companyIncoming, err := engine.GetIncomingEdges(ctx, company.ID)
	if err != nil {
		log.Fatalf("Failed to get company's incoming edges: %v", err)
	}
	fmt.Printf("   TechCorp has %d incoming edges:\n", len(companyIncoming))
	for _, edge := range companyIncoming {
		fmt.Printf("      - %s --%s--> %s\n", edge.FromNodeID, edge.Type, edge.ToNodeID)
	}

	fmt.Println("\n5. Updating nodes...")
	
	// Update Alice's age
	if err := engine.UpdateNode(ctx, alice.ID, map[string]interface{}{
		"age": int64(31),
		"updated": true,
	}); err != nil {
		log.Fatalf("Failed to update Alice: %v", err)
	}
	
	updatedAlice, _ := engine.GetNode(ctx, alice.ID)
	fmt.Printf("   Updated Alice: age=%v, updated=%v\n",
		updatedAlice.GetProperty("age"),
		updatedAlice.GetProperty("updated"))

	fmt.Println("\n6. Engine statistics...")
	
	stats := engine.Stats()
	fmt.Printf("   Total write lanes: %d\n", stats.NumLanes)
	fmt.Printf("   Total active entries: %d\n", stats.TotalActiveCount)
	fmt.Printf("   Total active size: %d bytes (%.2f MB)\n", 
		stats.TotalActiveSize, float64(stats.TotalActiveSize)/(1024*1024))
	fmt.Printf("   Total old memtables: %d\n", stats.TotalOldMemtables)
	
	fmt.Printf("\n   Per-lane statistics:\n")
	for _, laneStats := range stats.LaneStats {
		if laneStats.ActiveCount > 0 {
			fmt.Printf("      Lane %d: %d entries, %d bytes, %d old memtables\n",
				laneStats.ID, laneStats.ActiveCount, laneStats.ActiveSize, laneStats.OldMemtableCount)
		}
	}

	fmt.Println("\n=== Example completed successfully! ===")
}
