// Example application demonstrating Phase 2 features (SSTable + Cache)
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ayushanand18/crazygraphstore/pkg/graph"
	"github.com/ayushanand18/crazygraphstore/pkg/storage"
)

func main() {
	fmt.Println("=== Storage Engine Phase 2 Example ===")
	fmt.Println("Features: SSTable persistence + Multi-level caching")
	fmt.Println()

	// Create engine with Phase 2 features enabled
	config := storage.DefaultConfig()
	config.CacheSize = 512 * 1024 * 1024 // 512MB cache
	config.DataDir = "./data-phase2"

	fmt.Printf("Creating engine with:\n")
	fmt.Printf("  - %d write lanes\n", config.NumWriteLanes)
	fmt.Printf("  - %d MB memtable size per lane\n", config.MemtableSize/(1024*1024))
	fmt.Printf("  - %d MB total cache\n", config.CacheSize/(1024*1024))
	fmt.Printf("  - Data directory: %s\n", config.DataDir)
	fmt.Println()

	engine, err := storage.NewEngine(config)
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		return
	}

	// Start engine (starts background flusher)
	if err := engine.Start(); err != nil {
		fmt.Printf("Error starting engine: %v\n", err)
		return
	}
	defer engine.Stop()

	ctx := context.Background()

	// Phase 1: Write many nodes to trigger memtable rotation and flushing
	fmt.Println("1. Writing 1000 nodes to trigger flushing...")
	startWrite := time.Now()

	nodeIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		node := graph.NewNode(nodeID, []string{"TestNode"})
		node.SetProperty("index", int64(i))
		node.SetProperty("data", fmt.Sprintf("Large data string for node %d to fill memtable", i))

		if err := engine.CreateNode(ctx, node); err != nil {
			fmt.Printf("Error creating node: %v\n", err)
			return
		}
		nodeIDs[i] = nodeID

		if (i+1)%100 == 0 {
			fmt.Printf("   Written %d nodes...\n", i+1)
		}
	}

	writeTime := time.Since(startWrite)
	fmt.Printf("   Write completed in %v (%.2f writes/sec)\n", writeTime, float64(1000)/writeTime.Seconds())
	fmt.Println()

	// Wait a bit for background flushing
	fmt.Println("2. Waiting for background flushing...")
	time.Sleep(2 * time.Second)
	fmt.Println()

	// Show initial stats
	stats := engine.Stats()
	fmt.Println("3. Engine statistics after writes:")
	fmt.Printf("   Active memtables: %d entries, %.2f MB\n",
		stats.TotalActiveCount, float64(stats.TotalActiveSize)/(1024*1024))
	fmt.Printf("   Old memtables pending flush: %d\n", stats.TotalOldMemtables)
	fmt.Printf("   Flusher: %d flushes, %.2f MB written\n",
		stats.FlusherStats.TotalFlushes, float64(stats.FlusherStats.TotalBytes)/(1024*1024))
	fmt.Println()

	// Phase 2: Read nodes (cold cache)
	fmt.Println("4. Reading 100 nodes (cold cache)...")
	startRead := time.Now()

	for i := 0; i < 100; i++ {
		nodeID := nodeIDs[i*10] // Read every 10th node
		_, err := engine.GetNode(ctx, nodeID)
		if err != nil {
			fmt.Printf("Error reading node %s: %v\n", nodeID, err)
		}
	}

	coldReadTime := time.Since(startRead)
	fmt.Printf("   Cold reads: %v (%.2f reads/sec)\n", coldReadTime, float64(100)/coldReadTime.Seconds())
	fmt.Println()

	// Show cache stats after cold reads
	cacheStats := stats.CacheStats
	fmt.Println("5. Cache statistics after cold reads:")
	fmt.Printf("   L0 (Hot) Cache: %d entries, %.2f%% hit rate\n",
		cacheStats.HotTier.Entries, cacheStats.L0HitRate*100)
	fmt.Printf("   Overall hit rate: %.2f%%\n", cacheStats.OverallHitRate*100)
	fmt.Println()

	// Phase 3: Read same nodes again (warm cache)
	fmt.Println("6. Reading same 100 nodes (warm cache)...")
	startWarmRead := time.Now()

	for i := 0; i < 100; i++ {
		nodeID := nodeIDs[i*10]
		_, err := engine.GetNode(ctx, nodeID)
		if err != nil {
			fmt.Printf("Error reading node %s: %v\n", nodeID, err)
		}
	}

	warmReadTime := time.Since(startWarmRead)
	fmt.Printf("   Warm reads: %v (%.2f reads/sec)\n", warmReadTime, float64(100)/warmReadTime.Seconds())
	fmt.Printf("   Speed improvement: %.1fx faster\n", coldReadTime.Seconds()/warmReadTime.Seconds())
	fmt.Println()

	// Show final cache stats
	finalStats := engine.Stats()
	finalCache := finalStats.CacheStats
	fmt.Println("7. Final cache statistics:")
	fmt.Printf("   L0 (Hot) Cache: %d entries, %.2f%% hit rate\n",
		finalCache.HotTier.Entries, finalCache.L0HitRate*100)
	fmt.Printf("   Overall hit rate: %.2f%%\n", finalCache.OverallHitRate*100)
	fmt.Printf("   Cache utilization: %.1f%%\n", finalCache.HotTier.Utilization*100)
	fmt.Println()

	// Create some edges
	fmt.Println("8. Creating edges between nodes...")
	edgesCreated := 0
	for i := 0; i < 50; i++ {
		fromID := nodeIDs[i]
		toID := nodeIDs[i+50]
		edgeID := fmt.Sprintf("edge-%d-to-%d", i, i+50)

		edge := graph.NewEdge(edgeID, fromID, toID, "CONNECTS_TO")
		edge.SetProperty("weight", int64(i))

		if err := engine.CreateEdge(ctx, edge); err != nil {
			fmt.Printf("Error creating edge: %v\n", err)
		} else {
			edgesCreated++
		}
	}
	fmt.Printf("   Created %d edges\n", edgesCreated)
	fmt.Println()

	// Test graph traversal
	fmt.Println("9. Testing graph traversal...")
	testNodeID := nodeIDs[10]
	neighbors, err := engine.GetNeighbors(ctx, testNodeID, graph.DirectionOut)
	if err != nil {
		fmt.Printf("Error getting neighbors: %v\n", err)
	} else {
		fmt.Printf("   Node %s has %d outgoing neighbors\n", testNodeID, len(neighbors))
	}
	fmt.Println()

	// Final statistics
	fmt.Println("10. Final engine statistics:")
	finalStats = engine.Stats()
	fmt.Printf("   Total active entries: %d\n", finalStats.TotalActiveCount)
	fmt.Printf("   Total active size: %.2f MB\n", float64(finalStats.TotalActiveSize)/(1024*1024))
	fmt.Printf("   Cache hit rate: %.2f%%\n", finalStats.CacheStats.OverallHitRate*100)
	fmt.Printf("   Total flushes: %d\n", finalStats.FlusherStats.TotalFlushes)
	fmt.Printf("   Total flushed: %.2f MB\n", float64(finalStats.FlusherStats.TotalBytes)/(1024*1024))
	fmt.Println()

	fmt.Println("=== Phase 2 Example Complete ===")
	fmt.Println()
	fmt.Println("Key Phase 2 Features Demonstrated:")
	fmt.Println("  Automatic memtable flushing to SSTables")
	fmt.Println("  Multi-level cache with hit rate tracking")
	fmt.Println("  Cache performance (warm vs cold reads)")
	fmt.Println("  Background workers for persistence")
	fmt.Println("  Graceful shutdown with flush completion")
}
