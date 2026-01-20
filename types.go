package pebbledb

import "time"

// Entry represents a key-value pair with metadata
type Entry struct {
	Key       string
	Value     []byte
	Timestamp int64
	Deleted   bool // Tombstone marker for deletions
}

// NewEntry creates a new entry with the current timestamp
func NewEntry(key string, value []byte, deleted bool) *Entry {
	return &Entry{
		Key:       key,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
		Deleted:   deleted,
	}
}

// Config holds configuration for the database
type Config struct {
	DataDir          string // Directory to store data files
	MemtableSize     int    // Max size of memtable before flush (in entries)
	CompactionPeriod int    // How often to run compaction (in seconds)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DataDir:          "./data",
		MemtableSize:     1000,
		CompactionPeriod: 60,
	}
}
