package pebbledb

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DB represents the main database instance
type DB struct {
	config    *Config
	memtable  *Memtable
	sstables  []*SSTable
	wal       *WAL
	walID     int
	sstableID int
	mu        sync.RWMutex
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// Open opens or creates a new database
func Open(config *Config) (*DB, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db := &DB{
		config:    config,
		memtable:  NewMemtable(config.MemtableSize),
		sstables:  []*SSTable{},
		walID:     0,
		sstableID: 0,
		stopChan:  make(chan struct{}),
	}

	// Load existing SSTables
	if err := db.loadSSTables(); err != nil {
		return nil, fmt.Errorf("failed to load SSTables: %w", err)
	}

	// Create a new WAL
	wal, err := NewWAL(config.DataDir, db.walID)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}
	db.wal = wal

	// Start background compaction
	if config.CompactionPeriod > 0 {
		db.wg.Add(1)
		go db.runCompaction()
	}

	return db, nil
}

// Put adds or updates a key-value pair
func (db *DB) Put(key string, value []byte) error {
	entry := NewEntry(key, value, false)

	// Write to WAL first for durability
	db.mu.Lock()
	if err := db.wal.Write(entry); err != nil {
		db.mu.Unlock()
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Add to memtable
	db.memtable.Put(entry)

	// Check if memtable is full
	needsFlush := db.memtable.IsFull()
	db.mu.Unlock()

	if needsFlush {
		if err := db.flushMemtable(); err != nil {
			return fmt.Errorf("failed to flush memtable: %w", err)
		}
	}

	return nil
}

// Get retrieves a value by key
func (db *DB) Get(key string) ([]byte, error) {
	// Check memtable first
	db.mu.RLock()
	if entry, ok := db.memtable.Get(key); ok {
		db.mu.RUnlock()
		if entry.Deleted {
			return nil, nil // Key was deleted
		}
		return entry.Value, nil
	}

	// Check SSTables (from newest to oldest)
	sstables := make([]*SSTable, len(db.sstables))
	copy(sstables, db.sstables)
	db.mu.RUnlock()

	for i := len(sstables) - 1; i >= 0; i-- {
		entry, err := sstables[i].Get(key)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			if entry.Deleted {
				return nil, nil // Key was deleted
			}
			return entry.Value, nil
		}
	}

	return nil, nil // Key not found
}

// Delete removes a key (uses tombstone)
func (db *DB) Delete(key string) error {
	entry := NewEntry(key, nil, true)

	// Write to WAL
	db.mu.Lock()
	if err := db.wal.Write(entry); err != nil {
		db.mu.Unlock()
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Add to memtable
	db.memtable.Put(entry)

	// Check if memtable is full
	needsFlush := db.memtable.IsFull()
	db.mu.Unlock()

	if needsFlush {
		if err := db.flushMemtable(); err != nil {
			return fmt.Errorf("failed to flush memtable: %w", err)
		}
	}

	return nil
}

// flushMemtable writes the current memtable to disk as an SSTable
func (db *DB) flushMemtable() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Get sorted entries from memtable
	entries := db.memtable.GetSortedEntries()
	if len(entries) == 0 {
		return nil
	}

	// Create new SSTable
	sstable, err := NewSSTable(db.config.DataDir, db.sstableID, entries)
	if err != nil {
		return err
	}
	db.sstableID++

	// Add to list of SSTables
	db.sstables = append(db.sstables, sstable)

	// Clear memtable and create new WAL
	db.memtable.Clear()
	db.wal.Delete()
	db.walID++
	wal, err := NewWAL(db.config.DataDir, db.walID)
	if err != nil {
		return err
	}
	db.wal = wal

	return nil
}

// loadSSTables loads existing SSTables from disk
func (db *DB) loadSSTables() error {
	files, err := os.ReadDir(db.config.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".db" {
			path := filepath.Join(db.config.DataDir, file.Name())
			var id int
			fmt.Sscanf(file.Name(), "sstable-%d.db", &id)

			sstable, err := LoadSSTable(path, id)
			if err != nil {
				return err
			}

			db.sstables = append(db.sstables, sstable)
			if id >= db.sstableID {
				db.sstableID = id + 1
			}
		}
	}

	return nil
}

// runCompaction periodically merges SSTables
func (db *DB) runCompaction() {
	defer db.wg.Done()
	ticker := time.NewTicker(time.Duration(db.config.CompactionPeriod) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db.compact()
		case <-db.stopChan:
			return
		}
	}
}

// Close closes the database
func (db *DB) Close() error {
	close(db.stopChan)
	db.wg.Wait()

	db.mu.Lock()
	defer db.mu.Unlock()

	// Flush memtable if it has data
	if db.memtable.Size() > 0 {
		entries := db.memtable.GetSortedEntries()
		if len(entries) > 0 {
			sstable, err := NewSSTable(db.config.DataDir, db.sstableID, entries)
			if err != nil {
				return err
			}
			db.sstables = append(db.sstables, sstable)
		}
	}

	// Close WAL
	if db.wal != nil {
		if err := db.wal.Close(); err != nil {
			return err
		}
	}

	return nil
}
