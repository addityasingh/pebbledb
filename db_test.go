package pebbledb

import (
	"fmt"
	"os"
	"testing"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	dir := fmt.Sprintf("./test_data_%s", t.Name())
	os.RemoveAll(dir)

	config := &Config{
		DataDir:          dir,
		MemtableSize:     3,
		CompactionPeriod: 0, // Disable automatic compaction in tests
	}

	db, err := Open(config)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return db, cleanup
}

func TestBasicPutGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Put a value
	err := db.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get the value
	value, err := db.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(value) != "value1" {
		t.Errorf("Expected value1, got %s", string(value))
	}
}

func TestGetNonExistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	value, err := db.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if value != nil {
		t.Errorf("Expected nil, got %v", value)
	}
}

func TestUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Put initial value
	db.Put("key1", []byte("value1"))

	// Update the value
	db.Put("key1", []byte("value2"))

	// Get the updated value
	value, _ := db.Get("key1")
	if string(value) != "value2" {
		t.Errorf("Expected value2, got %s", string(value))
	}
}

func TestDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Put a value
	db.Put("key1", []byte("value1"))

	// Delete it
	err := db.Delete("key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Try to get it
	value, _ := db.Get("key1")
	if value != nil {
		t.Errorf("Expected nil after delete, got %v", value)
	}
}

func TestMemtableFlush(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Put more entries than memtable size to trigger flush
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		err := db.Put(key, []byte(value))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Verify all values
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := fmt.Sprintf("value%d", i)
		value, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if string(value) != expectedValue {
			t.Errorf("Expected %s, got %s", expectedValue, string(value))
		}
	}
}

func TestCompaction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Add multiple entries to create multiple SSTables
	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		db.Put(key, []byte(value))
	}

	// Update some entries
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("updated%d", i)
		db.Put(key, []byte(value))
	}

	// Delete some entries
	db.Delete("key5")
	db.Delete("key6")

	// Run compaction
	err := db.CompactNow()
	if err != nil {
		t.Fatalf("Compaction failed: %v", err)
	}

	// Verify data after compaction
	// Updated keys should have new values
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := fmt.Sprintf("updated%d", i)
		value, _ := db.Get(key)
		if string(value) != expectedValue {
			t.Errorf("Key %s: expected %s, got %s", key, expectedValue, string(value))
		}
	}

	// Deleted keys should be gone
	value, _ := db.Get("key5")
	if value != nil {
		t.Errorf("key5 should be deleted")
	}

	value, _ = db.Get("key6")
	if value != nil {
		t.Errorf("key6 should be deleted")
	}

	// Other keys should still exist
	for i := 7; i < 15; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := fmt.Sprintf("value%d", i)
		value, _ := db.Get(key)
		if string(value) != expectedValue {
			t.Errorf("Key %s: expected %s, got %s", key, expectedValue, string(value))
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := "./test_data_persistence"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	config := &Config{
		DataDir:          dir,
		MemtableSize:     3,
		CompactionPeriod: 0,
	}

	// Create DB and add data
	db, err := Open(config)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	db.Put("key1", []byte("value1"))
	db.Put("key2", []byte("value2"))
	db.Put("key3", []byte("value3"))
	db.Put("key4", []byte("value4")) // This should trigger a flush

	db.Close()

	// Reopen DB and verify data
	db2, err := Open(config)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db2.Close()

	value, _ := db2.Get("key1")
	if string(value) != "value1" {
		t.Errorf("Expected value1, got %s", string(value))
	}

	value, _ = db2.Get("key4")
	if string(value) != "value4" {
		t.Errorf("Expected value4, got %s", string(value))
	}
}
