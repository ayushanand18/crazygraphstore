
// Package compaction provides merger functionality for k-way merge of SSTables.
package compaction

import (
	"container/heap"
	"errors"
	"fmt"

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
	reader   *sstable.Reader
	idx      int
	currentKey   string
	currentValue []byte
	done         bool
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
	for i, table := range tables {
		reader, err := sstable.NewReader(table.Path)
		if err != nil {
			// Skip tables that fail to open (log in production)
			fmt.Printf("Failed to open table %s: %v\n", table.Path, err)
			continue
		}
		readers = append(readers, reader)
		
		scanner := &sstableScanner{
			reader: reader,
			idx:    i,
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

// advance moves the scanner to the next entry.
func (s *sstableScanner) advance() error {
	if s.done {
		return fmt.Errorf("scanner exhausted")
	}

	var key string
	var value []byte
	found := false

	// Scan through the SSTable
	err := s.reader.Scan(func(k string, v []byte) error {
		if !found {
			key = k
			value = make([]byte, len(v))
			copy(value, v)
			found = true
			return fmt.Errorf("stop") // Stop after first entry
		}
		return nil
	})

	if err != nil && err.Error() != "stop" {
		s.done = true
		return err
	}

	if !found {
		s.done = true
		return fmt.Errorf("no more entries")
	}

	s.currentKey = key
	s.currentValue = value
	return nil
}
