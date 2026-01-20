package main

import (
	"fmt"
	"log"
	"os"

	"pebbledb"
)

func main() {
	// Clean up any existing data
	os.RemoveAll("./example_data")

	// Create a new database with custom config
	config := &pebbledb.Config{
		DataDir:          "./example_data",
		MemtableSize:     5, // Small size for demonstration
		CompactionPeriod: 10,
	}

	db, err := pebbledb.Open(config)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Println("=== PebbleDB Example ===")

	// Put some data
	fmt.Println("1. Inserting data...")
	data := map[string]string{
		"user:1": "Alice",
		"user:2": "Bob",
		"user:3": "Charlie",
		"user:4": "David",
		"user:5": "Eve",
		"user:6": "Frank",
		"user:7": "Grace",
	}

	for key, value := range data {
		if err := db.Put(key, []byte(value)); err != nil {
			log.Fatalf("Failed to put %s: %v", key, err)
		}
		fmt.Printf("  Put: %s = %s\n", key, value)
	}

	// Get some data
	fmt.Println("\n2. Reading data...")
	keysToRead := []string{"user:1", "user:5", "user:7"}
	for _, key := range keysToRead {
		value, err := db.Get(key)
		if err != nil {
			log.Fatalf("Failed to get %s: %v", key, err)
		}
		if value == nil {
			fmt.Printf("  Get: %s = <not found>\n", key)
		} else {
			fmt.Printf("  Get: %s = %s\n", key, string(value))
		}
	}

	// Update a value
	fmt.Println("\n3. Updating data...")
	if err := db.Put("user:1", []byte("Alice Smith")); err != nil {
		log.Fatalf("Failed to update user:1: %v", err)
	}
	fmt.Println("  Updated: user:1 = Alice Smith")

	value, _ := db.Get("user:1")
	fmt.Printf("  Verified: user:1 = %s\n", string(value))

	// Delete a key
	fmt.Println("\n4. Deleting data...")
	if err := db.Delete("user:3"); err != nil {
		log.Fatalf("Failed to delete user:3: %v", err)
	}
	fmt.Println("  Deleted: user:3")

	value, _ = db.Get("user:3")
	if value == nil {
		fmt.Println("  Verified: user:3 not found (deleted)")
	}

	// Trigger compaction manually
	fmt.Println("\n5. Triggering compaction...")
	if err := db.CompactNow(); err != nil {
		log.Fatalf("Failed to compact: %v", err)
	}
	fmt.Println("  Compaction complete!")

	// Verify data after compaction
	fmt.Println("\n6. Verifying data after compaction...")
	for _, key := range keysToRead {
		value, err := db.Get(key)
		if err != nil {
			log.Fatalf("Failed to get %s: %v", key, err)
		}
		if value == nil {
			fmt.Printf("  Get: %s = <not found>\n", key)
		} else {
			fmt.Printf("  Get: %s = %s\n", key, string(value))
		}
	}

	fmt.Println("\n=== Example Complete ===")
}
