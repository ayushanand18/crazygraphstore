
// Package sstable provides SSTable reader implementation.
package sstable

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/edsrzf/mmap-go"
)

// Reader reads data from an SSTable file using memory-mapped I/O.
type Reader struct {
	file        *os.File
	mmap        mmap.MMap
	footer      *Footer
	indexCache  []*IndexEntry
	bloomFilter *bloom.BloomFilter
	fileSize    int64
}

// NewReader opens an SSTable for reading.
func NewReader(path string) (*Reader, error) {
	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSTable: %w", err)
	}

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := stat.Size()

	if fileSize < FooterSize {
		file.Close()
		return nil, fmt.Errorf("file too small to be valid SSTable: %d bytes", fileSize)
	}

	// Memory-map the file
	mmapData, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap file: %w", err)
	}

	reader := &Reader{
		file:     file,
		mmap:     mmapData,
		fileSize: fileSize,
	}

	// Read and parse footer
	if err := reader.readFooter(); err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	// Read index
	if err := reader.readIndex(); err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	// Read bloom filter
	if err := reader.readBloomFilter(); err != nil {
		reader.Close()
		return nil, fmt.Errorf("failed to read bloom filter: %w", err)
	}

	return reader, nil
}

// readFooter reads and validates the footer.
func (r *Reader) readFooter() error {
	footerOffset := r.fileSize - FooterSize
	footerData := r.mmap[footerOffset:]

	r.footer = &Footer{}
	if err := r.footer.Decode(footerData); err != nil {
		return err
	}

	// Verify checksum
	expectedChecksum := crc32.ChecksumIEEE(footerData[:44])
	if r.footer.Checksum != expectedChecksum {
		return fmt.Errorf("footer checksum mismatch")
	}

	return nil
}

// readIndex reads and caches the index block.
func (r *Reader) readIndex() error {
	offset := r.footer.IndexBlockOffset
	size := r.footer.IndexBlockSize

	if offset+size > uint64(r.fileSize) {
		return fmt.Errorf("index block out of bounds")
	}

	indexData := r.mmap[offset : offset+size]

	// Read entry count
	if len(indexData) < 4 {
		return fmt.Errorf("index block too small")
	}
	entryCount := binary.LittleEndian.Uint32(indexData[0:4])

	r.indexCache = make([]*IndexEntry, 0, entryCount)
	dataOffset := 4

	// Decode all index entries
	for i := uint32(0); i < entryCount; i++ {
		entry, bytesRead, err := DecodeIndexEntry(indexData[dataOffset:])
		if err != nil {
			return fmt.Errorf("failed to decode index entry %d: %w", i, err)
		}
		r.indexCache = append(r.indexCache, entry)
		dataOffset += bytesRead
	}

	return nil
}

// readBloomFilter reads the bloom filter.
func (r *Reader) readBloomFilter() error {
	offset := r.footer.BloomFilterOffset
	size := r.footer.BloomFilterSize

	if offset+size > uint64(r.fileSize) {
		return fmt.Errorf("bloom filter out of bounds")
	}

	bloomData := r.mmap[offset : offset+size]

	r.bloomFilter = &bloom.BloomFilter{}
	if err := r.bloomFilter.GobDecode(bloomData); err != nil {
		return fmt.Errorf("failed to decode bloom filter: %w", err)
	}

	return nil
}

// Get retrieves a value by key.
func (r *Reader) Get(key string) ([]byte, error) {
	// Check bloom filter first
	if !r.bloomFilter.TestString(key) {
		return nil, fmt.Errorf("key not found (bloom filter)")
	}

	// Find the block that might contain the key
	blockHandle := r.findBlock(key)
	if blockHandle == nil {
		return nil, fmt.Errorf("key not found (index)")
	}

	// Read and search the block
	blockData, err := r.readBlock(blockHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to read block: %w", err)
	}

	// Search within the block
	return r.searchBlock(blockData, key)
}

// findBlock uses binary search to find the block that might contain the key.
func (r *Reader) findBlock(key string) *BlockHandle {
	left, right := 0, len(r.indexCache)-1

	for left <= right {
		mid := (left + right) / 2
		lastKey := r.indexCache[mid].Key

		if key <= lastKey {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	if left >= len(r.indexCache) {
		return nil
	}

	return &r.indexCache[left].Handle
}

// readBlock reads and validates a data block.
func (r *Reader) readBlock(handle *BlockHandle) ([]byte, error) {
	if handle.Offset+handle.Size > uint64(r.fileSize) {
		return nil, fmt.Errorf("block out of bounds")
	}

	blockData := r.mmap[handle.Offset : handle.Offset+handle.Size]

	// Extract components
	dataLen := len(blockData) - 1 - 4 // Subtract compression type and checksum
	if dataLen < 0 {
		return nil, fmt.Errorf("invalid block size")
	}

	data := blockData[:dataLen]
	compressionType := CompressionType(blockData[dataLen])
	checksum := binary.LittleEndian.Uint32(blockData[dataLen+1:])

	// Verify checksum
	expectedChecksum := crc32.ChecksumIEEE(data)
	if checksum != expectedChecksum {
		return nil, fmt.Errorf("block checksum mismatch")
	}

	// Decompress if needed
	switch compressionType {
	case CompressionNone:
		return data, nil
	case CompressionSnappy, CompressionLZ4:
		return nil, fmt.Errorf("compression not yet implemented")
	default:
		return nil, fmt.Errorf("unknown compression type: %d", compressionType)
	}
}

// searchBlock performs a linear search within a block for the key.
func (r *Reader) searchBlock(blockData []byte, key string) ([]byte, error) {
	offset := 0

	for offset < len(blockData) {
		// Read key length
		if offset+4 > len(blockData) {
			break
		}
		keyLen := binary.LittleEndian.Uint32(blockData[offset : offset+4])
		offset += 4

		// Read key
		if offset+int(keyLen) > len(blockData) {
			break
		}
		entryKey := string(blockData[offset : offset+int(keyLen)])
		offset += int(keyLen)

		// Read value length
		if offset+4 > len(blockData) {
			break
		}
		valueLen := binary.LittleEndian.Uint32(blockData[offset : offset+4])
		offset += 4

		// Read value
		if offset+int(valueLen) > len(blockData) {
			break
		}
		value := blockData[offset : offset+int(valueLen)]
		offset += int(valueLen)

		// Check if this is the key we're looking for
		if entryKey == key {
			// Check for tombstone (nil value)
			if valueLen == 0 {
				return nil, fmt.Errorf("key deleted")
			}
			return value, nil
		}

		// Since block is sorted, we can stop early
		if entryKey > key {
			break
		}
	}

	return nil, fmt.Errorf("key not found in block")
}

// Scan iterates over all entries in the SSTable.
func (r *Reader) Scan(fn func(key string, value []byte) error) error {
	for _, indexEntry := range r.indexCache {
		blockData, err := r.readBlock(&indexEntry.Handle)
		if err != nil {
			return fmt.Errorf("failed to read block: %w", err)
		}

		if err := r.scanBlock(blockData, fn); err != nil {
			return err
		}
	}

	return nil
}

// scanBlock iterates over entries in a single block.
func (r *Reader) scanBlock(blockData []byte, fn func(key string, value []byte) error) error {
	offset := 0

	for offset < len(blockData) {
		// Read key length
		if offset+4 > len(blockData) {
			break
		}
		keyLen := binary.LittleEndian.Uint32(blockData[offset : offset+4])
		offset += 4

		// Read key
		if offset+int(keyLen) > len(blockData) {
			break
		}
		key := string(blockData[offset : offset+int(keyLen)])
		offset += int(keyLen)

		// Read value length
		if offset+4 > len(blockData) {
			break
		}
		valueLen := binary.LittleEndian.Uint32(blockData[offset : offset+4])
		offset += 4

		// Read value
		if offset+int(valueLen) > len(blockData) {
			break
		}
		value := blockData[offset : offset+int(valueLen)]
		offset += int(valueLen)

		// Skip tombstones
		if valueLen > 0 {
			if err := fn(key, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// Close closes the reader and unmaps the file.
func (r *Reader) Close() error {
	var firstErr error

	if r.mmap != nil {
		if err := r.mmap.Unmap(); err != nil {
			firstErr = err
		}
	}

	if r.file != nil {
		if err := r.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// FileSize returns the size of the SSTable file.
func (r *Reader) FileSize() int64 {
	return r.fileSize
}

// EntryCount returns the number of data blocks in the SSTable.
func (r *Reader) EntryCount() int {
	return len(r.indexCache)
}
