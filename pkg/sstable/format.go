
// Package sstable provides disk-based sorted string table (SSTable) storage.
package sstable

import (
	"encoding/binary"
	"fmt"
)

// SSTable file format:
// 
// [Data Blocks]
// [Index Block]
// [Bloom Filter]
// [Footer]
//
// Each data block:
// - Key-value pairs (sorted by key)
// - Block trailer (compression type, CRC32)
//
// Index block:
// - Maps key ranges to block offsets
//
// Footer (fixed 48 bytes):
// - Index block offset (8 bytes)
// - Index block size (8 bytes)
// - Bloom filter offset (8 bytes)
// - Bloom filter size (8 bytes)
// - Magic number (8 bytes)
// - Version (4 bytes)
// - CRC32 (4 bytes)

const (
	// FooterSize is the size of the SSTable footer in bytes.
	FooterSize = 48

	// MagicNumber identifies valid SSTable files.
	MagicNumber = 0x5353544142314556 // "SSTABLE1" in hex

	// Version is the current SSTable format version.
	Version = 1

	// BlockSize is the target size for data blocks (64KB).
	BlockSize = 64 * 1024

	// MaxBlockSize is the maximum size for a data block (128KB).
	MaxBlockSize = 128 * 1024
)

// CompressionType represents the compression algorithm used.
type CompressionType byte

const (
	CompressionNone   CompressionType = 0
	CompressionSnappy CompressionType = 1
	CompressionLZ4    CompressionType = 2
)

// Footer contains metadata about the SSTable.
type Footer struct {
	IndexBlockOffset  uint64 // Offset to index block
	IndexBlockSize    uint64 // Size of index block
	BloomFilterOffset uint64 // Offset to bloom filter
	BloomFilterSize   uint64 // Size of bloom filter
	MagicNumber       uint64 // Magic number for validation
	Version           uint32 // Format version
	Checksum          uint32 // CRC32 checksum of footer
}

// Encode serializes the footer to bytes.
func (f *Footer) Encode() []byte {
	buf := make([]byte, FooterSize)
	
	binary.LittleEndian.PutUint64(buf[0:8], f.IndexBlockOffset)
	binary.LittleEndian.PutUint64(buf[8:16], f.IndexBlockSize)
	binary.LittleEndian.PutUint64(buf[16:24], f.BloomFilterOffset)
	binary.LittleEndian.PutUint64(buf[24:32], f.BloomFilterSize)
	binary.LittleEndian.PutUint64(buf[32:40], f.MagicNumber)
	binary.LittleEndian.PutUint32(buf[40:44], f.Version)
	binary.LittleEndian.PutUint32(buf[44:48], f.Checksum)
	
	return buf
}

// Decode deserializes the footer from bytes.
func (f *Footer) Decode(data []byte) error {
	if len(data) != FooterSize {
		return fmt.Errorf("invalid footer size: expected %d, got %d", FooterSize, len(data))
	}
	
	f.IndexBlockOffset = binary.LittleEndian.Uint64(data[0:8])
	f.IndexBlockSize = binary.LittleEndian.Uint64(data[8:16])
	f.BloomFilterOffset = binary.LittleEndian.Uint64(data[16:24])
	f.BloomFilterSize = binary.LittleEndian.Uint64(data[24:32])
	f.MagicNumber = binary.LittleEndian.Uint64(data[32:40])
	f.Version = binary.LittleEndian.Uint32(data[40:44])
	f.Checksum = binary.LittleEndian.Uint32(data[44:48])
	
	// Validate magic number
	if f.MagicNumber != MagicNumber {
		return fmt.Errorf("invalid magic number: 0x%x", f.MagicNumber)
	}
	
	// Validate version
	if f.Version != Version {
		return fmt.Errorf("unsupported version: %d", f.Version)
	}
	
	return nil
}

// BlockHandle represents a reference to a block in the SSTable.
type BlockHandle struct {
	Offset uint64 // File offset
	Size   uint64 // Block size in bytes
}

// Encode serializes the block handle to bytes.
func (bh *BlockHandle) Encode() []byte {
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint64(buf[0:8], bh.Offset)
	binary.LittleEndian.PutUint64(buf[8:16], bh.Size)
	return buf
}

// Decode deserializes the block handle from bytes.
func (bh *BlockHandle) Decode(data []byte) error {
	if len(data) < 16 {
		return fmt.Errorf("insufficient data for block handle")
	}
	
	bh.Offset = binary.LittleEndian.Uint64(data[0:8])
	bh.Size = binary.LittleEndian.Uint64(data[8:16])
	
	return nil
}

// Block represents a data block in the SSTable.
type Block struct {
	Data            []byte
	CompressionType CompressionType
	Checksum        uint32
}

// IndexEntry represents an entry in the index block.
type IndexEntry struct {
	Key    string      // Last key in the block
	Handle BlockHandle // Block location
}

// EncodeIndexEntry serializes an index entry.
func EncodeIndexEntry(entry *IndexEntry) []byte {
	keyLen := uint32(len(entry.Key))
	buf := make([]byte, 4+keyLen+16)
	
	// Key length
	binary.LittleEndian.PutUint32(buf[0:4], keyLen)
	
	// Key
	copy(buf[4:4+keyLen], entry.Key)
	
	// Block handle
	copy(buf[4+keyLen:], entry.Handle.Encode())
	
	return buf
}

// DecodeIndexEntry deserializes an index entry.
func DecodeIndexEntry(data []byte) (*IndexEntry, int, error) {
	if len(data) < 4 {
		return nil, 0, fmt.Errorf("insufficient data for index entry")
	}
	
	keyLen := binary.LittleEndian.Uint32(data[0:4])
	if len(data) < int(4+keyLen+16) {
		return nil, 0, fmt.Errorf("insufficient data for index entry")
	}
	
	entry := &IndexEntry{
		Key: string(data[4 : 4+keyLen]),
	}
	
	if err := entry.Handle.Decode(data[4+keyLen:]); err != nil {
		return nil, 0, err
	}
	
	bytesRead := int(4 + keyLen + 16)
	return entry, bytesRead, nil
}
