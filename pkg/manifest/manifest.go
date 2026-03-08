
// Package manifest provides SSTable manifest tracking for recovery.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manifest tracks all active SSTables for crash recovery.
type Manifest struct {
	Version   int64            `json:"version"`
	SSTables  []*SSTableEntry  `json:"sstables"`
	UpdatedAt time.Time        `json:"updated_at"`
	mu        sync.RWMutex
	path      string
}

// SSTableEntry represents an SSTable in the manifest.
type SSTableEntry struct {
	Path       string    `json:"path"`
	Level      int       `json:"level"`
	Size       int64     `json:"size"`
	NumEntries uint64    `json:"num_entries"`
	CreatedAt  time.Time `json:"created_at"`
	MinKey     string    `json:"min_key"`
	MaxKey     string    `json:"max_key"`
}

// Config holds manifest configuration.
type Config struct {
	DataDir string
}

// DefaultConfig returns default manifest configuration.
func DefaultConfig() *Config {
	return &Config{
		DataDir: "./data",
	}
}

// NewManifest creates a new manifest or loads existing one.
func NewManifest(config *Config) (*Manifest, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	path := filepath.Join(config.DataDir, "MANIFEST")

	// Try to load existing manifest
	manifest, err := loadManifest(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new manifest
			manifest = &Manifest{
				Version:   1,
				SSTables:  make([]*SSTableEntry, 0),
				UpdatedAt: time.Now(),
				path:      path,
			}
			
			// Save initial manifest
			if err := manifest.Save(); err != nil {
				return nil, fmt.Errorf("failed to save initial manifest: %w", err)
			}
			
			return manifest, nil
		}
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	manifest.path = path
	return manifest, nil
}

// loadManifest loads a manifest from disk.
func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

// AddSSTable adds a new SSTable to the manifest.
func (m *Manifest) AddSSTable(entry *SSTableEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SSTables = append(m.SSTables, entry)
	m.Version++
	m.UpdatedAt = time.Now()

	return m.save()
}

// RemoveSSTable removes an SSTable from the manifest.
func (m *Manifest) RemoveSSTable(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newTables := make([]*SSTableEntry, 0, len(m.SSTables))
	for _, table := range m.SSTables {
		if table.Path != path {
			newTables = append(newTables, table)
		}
	}

	m.SSTables = newTables
	m.Version++
	m.UpdatedAt = time.Now()

	return m.save()
}

// RemoveSSTables removes multiple SSTables from the manifest.
func (m *Manifest) RemoveSSTables(paths []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pathSet := make(map[string]bool)
	for _, path := range paths {
		pathSet[path] = true
	}

	newTables := make([]*SSTableEntry, 0, len(m.SSTables))
	for _, table := range m.SSTables {
		if !pathSet[table.Path] {
			newTables = append(newTables, table)
		}
	}

	m.SSTables = newTables
	m.Version++
	m.UpdatedAt = time.Now()

	return m.save()
}

// UpdateSSTables atomically updates the manifest with a new set of SSTables.
func (m *Manifest) UpdateSSTables(toAdd []*SSTableEntry, toRemove []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create removal set
	removeSet := make(map[string]bool)
	for _, path := range toRemove {
		removeSet[path] = true
	}

	// Filter existing tables
	newTables := make([]*SSTableEntry, 0, len(m.SSTables)+len(toAdd))
	for _, table := range m.SSTables {
		if !removeSet[table.Path] {
			newTables = append(newTables, table)
		}
	}

	// Add new tables
	newTables = append(newTables, toAdd...)

	m.SSTables = newTables
	m.Version++
	m.UpdatedAt = time.Now()

	return m.save()
}

// GetSSTables returns a copy of all SSTable entries.
func (m *Manifest) GetSSTables() []*SSTableEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tables := make([]*SSTableEntry, len(m.SSTables))
	copy(tables, m.SSTables)
	return tables
}

// Save saves the manifest to disk.
func (m *Manifest) Save() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.save()
}

// save saves the manifest to disk (caller must hold lock).
func (m *Manifest) save() error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Write to temporary file first
	tempPath := m.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, m.path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename manifest: %w", err)
	}

	return nil
}

// Stats returns manifest statistics.
type ManifestStats struct {
	Version       int64
	NumSSTables   int
	TotalSize     int64
	NumLevels     int
	TablesPerLevel map[int]int
	UpdatedAt     time.Time
}

// Stats returns current manifest statistics.
func (m *Manifest) Stats() ManifestStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSize := int64(0)
	maxLevel := 0
	tablesPerLevel := make(map[int]int)

	for _, table := range m.SSTables {
		totalSize += table.Size
		if table.Level > maxLevel {
			maxLevel = table.Level
		}
		tablesPerLevel[table.Level]++
	}

	return ManifestStats{
		Version:        m.Version,
		NumSSTables:    len(m.SSTables),
		TotalSize:      totalSize,
		NumLevels:      maxLevel + 1,
		TablesPerLevel: tablesPerLevel,
		UpdatedAt:      m.UpdatedAt,
	}
}

// Checkpoint creates a backup of the manifest.
func (m *Manifest) Checkpoint() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	checkpointPath := fmt.Sprintf("%s.%d", m.path, time.Now().Unix())
	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return nil
}

// Verify checks that all SSTables in the manifest exist.
func (m *Manifest) Verify() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, table := range m.SSTables {
		if _, err := os.Stat(table.Path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("SSTable %s does not exist", table.Path)
			}
			return fmt.Errorf("failed to stat SSTable %s: %w", table.Path, err)
		}
	}

	return nil
}
