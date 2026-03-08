
// Package sstable provides SSTable writer implementation.
package sstable

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"

	"github.com/bits-and-blooms/bloom/v3"
)

// Writer writes data to an SSTable file.
type Writer struct {
	file          *os.File
	writer        *bufio.Writer
	currentBlock  []byte
	currentOffset uint64
	indexEntries  []*IndexEntry
	bloomFilter   *bloom.BloomFilter
	lastKey       string
	entriesCount  uint64
	closed        bool
}

// NewWriter creates a new SSTable writer.
func NewWriter(path string, expectedEntries uint) (*Writer, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSTable file: %w", err)
	}

	// Create bloom filter with 1% false positive rate
	bloomFilter := bloom.NewWithEstimates(expectedEntries, 0.01)

	return &Writer{
		file:          file,
		writer:        bufio.NewWriterSize(file, 256*1024), // 256KB buffer
		currentBlock:  make([]byte, 0, BlockSize),
		indexEntries:  make([]*IndexEntry, 0),
		bloomFilter:   bloomFilter,
		currentOffset: 0,
	}, nil
}

// Add adds a key-value pair to the SSTable.
// Keys must be added in sorted order.
func (w *Writer) Add(key string, value []byte) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Validate key order
	if w.lastKey != "" && key <= w.lastKey {
		return fmt.Errorf("keys must be added in sorted order: %s <= %s", key, w.lastKey)
	}

	// Add key to bloom filter
	w.bloomFilter.AddString(key)

	// Encode entry: [keyLen(4)][key][valueLen(4)][value]
	keyLen := uint32(len(key))
	valueLen := uint32(len(value))
	entrySize := 4 + keyLen + 4 + valueLen

	// Check if we need to flush the current block
	if len(w.currentBlock)+int(entrySize) > BlockSize {
		if err := w.flushBlock(); err != nil {
			return err
		}
	}

	// Add entry to current block
	buf := make([]byte, entrySize)
	offset := 0

	// Key length
	binary.LittleEndian.PutUint32(buf[offset:offset+4], keyLen)
	offset += 4

	// Key
	copy(buf[offset:offset+int(keyLen)], key)
	offset += int(keyLen)

	// Value length
	binary.LittleEndian.PutUint32(buf[offset:offset+4], valueLen)
	offset += 4

	// Value
	copy(buf[offset:offset+int(valueLen)], value)

	w.currentBlock = append(w.currentBlock, buf...)
	w.lastKey = key
	w.entriesCount++

	return nil
}

// flushBlock writes the current block to disk and creates an index entry.
func (w *Writer) flushBlock() error {
	if len(w.currentBlock) == 0 {
		return nil
	}

	// Calculate checksum
	checksum := crc32.ChecksumIEEE(w.currentBlock)

	// Write block data
	blockOffset := w.currentOffset
	if _, err := w.writer.Write(w.currentBlock); err != nil {
		return fmt.Errorf("failed to write block: %w", err)
	}

	// Write compression type (none for now)
	if err := w.writer.WriteByte(byte(CompressionNone)); err != nil {
		return fmt.Errorf("failed to write compression type: %w", err)
	}

	// Write checksum
	checksumBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(checksumBuf, checksum)
	if _, err := w.writer.Write(checksumBuf); err != nil {
		return fmt.Errorf("failed to write checksum: %w", err)
	}

	blockSize := uint64(len(w.currentBlock) + 1 + 4) // data + compression + checksum

	// Create index entry
	indexEntry := &IndexEntry{
		Key: w.lastKey,
		Handle: BlockHandle{
			Offset: blockOffset,
			Size:   blockSize,
		},
	}
	w.indexEntries = append(w.indexEntries, indexEntry)

	// Update offset
	w.currentOffset += blockSize

	// Reset current block
	w.currentBlock = w.currentBlock[:0]

	return nil
}

// Close finalizes the SSTable and writes the footer.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}

	// Flush any remaining data in the current block
	if err := w.flushBlock(); err != nil {
		return err
	}

	// Write index block
	indexOffset := w.currentOffset
	indexData := w.encodeIndexBlock()
	if _, err := w.writer.Write(indexData); err != nil {
		return fmt.Errorf("failed to write index block: %w", err)
	}
	indexSize := uint64(len(indexData))
	w.currentOffset += indexSize

	// Write bloom filter
	bloomOffset := w.currentOffset
	bloomData, err := w.bloomFilter.GobEncode()
	if err != nil {
		return fmt.Errorf("failed to encode bloom filter: %w", err)
	}
	if _, err := w.writer.Write(bloomData); err != nil {
		return fmt.Errorf("failed to write bloom filter: %w", err)
	}
	bloomSize := uint64(len(bloomData))
	w.currentOffset += bloomSize

	// Create and write footer
	footer := &Footer{
		IndexBlockOffset:  indexOffset,
		IndexBlockSize:    indexSize,
		BloomFilterOffset: bloomOffset,
		BloomFilterSize:   bloomSize,
		MagicNumber:       MagicNumber,
		Version:           Version,
	}

	// Calculate footer checksum
	footerData := footer.Encode()
	footer.Checksum = crc32.ChecksumIEEE(footerData[:44]) // Exclude checksum field
	footerData = footer.Encode() // Re-encode with checksum

	if _, err := w.writer.Write(footerData); err != nil {
		return fmt.Errorf("failed to write footer: %w", err)
	}

	// Flush buffered writes
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	// Sync to disk
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Close file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	w.closed = true
	return nil
}

// encodeIndexBlock serializes the index entries.
func (w *Writer) encodeIndexBlock() []byte {
	var data []byte

	// Number of entries
	countBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(countBuf, uint32(len(w.indexEntries)))
	data = append(data, countBuf...)

	// Encode each entry
	for _, entry := range w.indexEntries {
		entryData := EncodeIndexEntry(entry)
		data = append(data, entryData...)
	}

	return data
}

// EntriesCount returns the number of entries written.
func (w *Writer) EntriesCount() uint64 {
	return w.entriesCount
}

// FileSize returns the current file size.
func (w *Writer) FileSize() uint64 {
	return w.currentOffset
}
