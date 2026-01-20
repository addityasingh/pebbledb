package pebbledb

import (
	"fmt"
	"sort"
)

// compact merges multiple SSTables into fewer SSTables
func (db *DB) compact() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Need at least 2 SSTables to compact
	if len(db.sstables) < 2 {
		return nil
	}

	fmt.Printf("Starting compaction of %d SSTables...\n", len(db.sstables))

	// Collect all entries from all SSTables
	allEntries := make(map[string]*Entry)

	for _, sstable := range db.sstables {
		entries, err := sstable.GetAll()
		if err != nil {
			return fmt.Errorf("failed to read SSTable: %w", err)
		}

		// Merge entries, keeping the most recent version of each key
		for _, entry := range entries {
			existing, ok := allEntries[entry.Key]
			if !ok || entry.Timestamp > existing.Timestamp {
				allEntries[entry.Key] = entry
			}
		}
	}

	// Convert to sorted slice, filtering out deleted entries
	sortedEntries := []*Entry{}
	keys := make([]string, 0, len(allEntries))
	for k := range allEntries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		entry := allEntries[k]
		// Only keep non-deleted entries during compaction
		if !entry.Deleted {
			sortedEntries = append(sortedEntries, entry)
		}
	}

	// If no entries left, just delete all SSTables
	if len(sortedEntries) == 0 {
		for _, sstable := range db.sstables {
			if err := sstable.Delete(); err != nil {
				return err
			}
		}
		db.sstables = []*SSTable{}
		fmt.Println("Compaction complete: all entries were deleted")
		return nil
	}

	// Delete old SSTables
	for _, sstable := range db.sstables {
		if err := sstable.Delete(); err != nil {
			return err
		}
	}

	// Create new compacted SSTables
	// For simplicity, we'll create one large SSTable
	// In production, you'd want to split into multiple tables
	newSSTable, err := NewSSTable(db.config.DataDir, db.sstableID, sortedEntries)
	if err != nil {
		return fmt.Errorf("failed to create compacted SSTable: %w", err)
	}
	db.sstableID++

	db.sstables = []*SSTable{newSSTable}

	fmt.Printf("Compaction complete: merged into 1 SSTable with %d entries\n", len(sortedEntries))
	return nil
}

// CompactNow triggers an immediate compaction (useful for testing/manual control)
func (db *DB) CompactNow() error {
	return db.compact()
}
