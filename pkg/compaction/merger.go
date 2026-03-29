// Package compaction provides merger functionality for k-way merge of SSTables.
package compaction

import (
	"container/heap"
	"errors"
	"fmt"
	"os"

	"github.com/ayushanand18/crazygraphstore/pkg/sstable"
)

var ErrMergerDone = errors.New("merger exhausted")

// merger implements k-way merge of multiple SSTables.
type merger struct {
	readers []*sstable.Reader
	heap    *mergeHeap
}

// mergeEntry represents an entry in the merge heap.
type mergeEntry struct {
	key       string
	value     []byte
	readerIdx int
	scanners  []*sstableScanner
}

// sstableScanner wraps an SSTable reader with scanning state.
type sstableScanner struct {
	reader       *sstable.Reader
	idx          int
	entries      []mergeItem
	pos          int
	currentKey   string
	currentValue []byte
	done         bool
}

type mergeItem struct {
	key   string
	value []byte
}

// mergeHeap implements heap.Interface for k-way merge.
type mergeHeap []*mergeEntry

func (h mergeHeap) Len() int           { return len(h) }
func (h mergeHeap) Less(i, j int) bool { return h[i].key < h[j].key }
func (h mergeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *mergeHeap) Push(x interface{}) {
	*h = append(*h, x.(*mergeEntry))
}

func (h *mergeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// newMerger creates a new k-way merger.
func newMerger(tables []*TableInfo) *merger {
	readers := make([]*sstable.Reader, 0, len(tables))
	scanners := make([]*sstableScanner, 0, len(tables))

	// Open all readers
	for _, table := range tables {
		reader, err := sstable.NewReader(table.Path)
		if err != nil {
			// Skip tables that fail to open (log in production)
			fmt.Printf("Failed to open table %s: %v\n", table.Path, err)
			continue
		}
		readers = append(readers, reader)

		scanner := &sstableScanner{
			reader: reader,
			idx:    len(scanners),
		}
		scanners = append(scanners, scanner)
	}

	// Initialize heap with first entry from each scanner
	h := &mergeHeap{}
	heap.Init(h)

	for _, scanner := range scanners {
		if err := scanner.advance(); err == nil {
			heap.Push(h, &mergeEntry{
				key:       scanner.currentKey,
				value:     scanner.currentValue,
				readerIdx: scanner.idx,
				scanners:  scanners,
			})
		}
	}

	return &merger{
		readers: readers,
		heap:    h,
	}
}

// Next returns the next key-value pair in sorted order.
func (m *merger) Next() (string, []byte, error) {
	if m.heap.Len() == 0 {
		return "", nil, ErrMergerDone
	}

	// Pop smallest entry
	entry := heap.Pop(m.heap).(*mergeEntry)
	key := entry.key
	value := entry.value

	// Skip duplicate keys (keep newest, which comes from higher-indexed reader)
	for m.heap.Len() > 0 && (*m.heap)[0].key == key {
		heap.Pop(m.heap)
	}

	// Advance the scanner that provided this entry
	scanner := entry.scanners[entry.readerIdx]
	if err := scanner.advance(); err == nil {
		heap.Push(m.heap, &mergeEntry{
			key:       scanner.currentKey,
			value:     scanner.currentValue,
			readerIdx: scanner.idx,
			scanners:  entry.scanners,
		})
	}

	return key, value, nil
}

// Close closes all readers.
func (m *merger) Close() error {
	for _, reader := range m.readers {
		reader.Close()
	}
	return nil
}

// Merge performs k-way merge of SSTables and writes to output path.
func (m *merger) Merge(inputPaths []string, outputPath string) error {
	if len(inputPaths) == 0 {
		return fmt.Errorf("no input files to merge")
	}

	// Create output writer
	writer, err := sstable.NewWriter(outputPath, 4096)
	if err != nil {
		return fmt.Errorf("failed to create output writer: %w", err)
	}
	defer writer.Close()

	// Create table info for each input
	tables := make([]*TableInfo, 0, len(inputPaths))
	for _, path := range inputPaths {
		stat, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat input file %s: %w", path, err)
		}
		tables = append(tables, &TableInfo{
			Path:  path,
			Size:  stat.Size(),
			Level: 0, // Assume level 0 for input files
		})
	}

	// Create a new merger with these tables
	mergeMerger := newMerger(tables)
	defer mergeMerger.Close()

	// Merge all entries and write to output
	for {
		key, value, err := mergeMerger.Next()
		if err != nil {
			if err == ErrMergerDone {
				break
			}
			return fmt.Errorf("merge error: %w", err)
		}

		if err := writer.Add(key, value); err != nil {
			return fmt.Errorf("failed to write merged entry: %w", err)
		}
	}

	return nil
}

// advance moves the scanner to the next entry.
func (s *sstableScanner) advance() error {
	if s.done {
		return fmt.Errorf("scanner exhausted")
	}

	if s.entries == nil {
		s.entries = make([]mergeItem, 0)
		err := s.reader.Scan(func(k string, v []byte) error {
			value := make([]byte, len(v))
			copy(value, v)
			s.entries = append(s.entries, mergeItem{key: k, value: value})
			return nil
		})
		if err != nil {
			s.done = true
			return err
		}
	}

	if s.pos >= len(s.entries) {
		s.done = true
		return fmt.Errorf("no more entries")
	}

	entry := s.entries[s.pos]
	s.pos++
	s.currentKey = entry.key
	s.currentValue = entry.value
	return nil
}
