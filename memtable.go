package pebbledb

import (
	"sort"
	"sync"
)

// Memtable is an in-memory sorted structure for recent writes
type Memtable struct {
	mu      sync.RWMutex
	entries map[string]*Entry // Map for O(1) lookups
	maxSize int
}

// NewMemtable creates a new memtable
func NewMemtable(maxSize int) *Memtable {
	return &Memtable{
		entries: make(map[string]*Entry),
		maxSize: maxSize,
	}
}

// Put adds or updates an entry in the memtable
func (m *Memtable) Put(entry *Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[entry.Key] = entry
}

// Get retrieves an entry from the memtable
func (m *Memtable) Get(key string) (*Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[key]
	return entry, ok
}

// Size returns the number of entries in the memtable
func (m *Memtable) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// IsFull checks if the memtable has reached its maximum size
func (m *Memtable) IsFull() bool {
	return m.Size() >= m.maxSize
}

// GetSortedEntries returns all entries sorted by key
func (m *Memtable) GetSortedEntries() []*Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get all keys and sort them
	keys := make([]string, 0, len(m.entries))
	for k := range m.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build sorted entries slice
	entries := make([]*Entry, 0, len(m.entries))
	for _, k := range keys {
		entries = append(entries, m.entries[k])
	}

	return entries
}

// Clear removes all entries from the memtable
func (m *Memtable) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*Entry)
}
