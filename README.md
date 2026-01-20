# PebbleDB

PebbleDB is a minimalistic implementation of RocksDB in Golang

## Implementation goals and algorithm

- Implement a WAL file to append new writes to a log file
- Add a `memtable` for in memory entries . 
  - This is a balanced tree (RB tree or AVL) which keeps a sorted in-memory representation of the latest segments
- Once the memtable reaches a certain limit, move it to disk. Create a new log file and memtable for upcoming writes
- Periodically perform a merge and compaction on disk

## Implementation Details

### Components

1. **WAL (Write-Ahead Log)**: Ensures durability by logging all writes before applying them to the memtable
2. **Memtable**: In-memory sorted storage for recent writes (implemented with Go maps + sorting)
3. **SSTable**: Sorted String Table format for persistent on-disk storage
4. **Compaction**: Merges multiple SSTables and removes deleted entries

### Architecture

```
Write Flow:
  Put/Delete → WAL → Memtable → (if full) → Flush to SSTable

Read Flow:
  Get → Check Memtable → Check SSTables (newest to oldest)

Background:
  Periodic Compaction → Merge SSTables → Remove tombstones
```

## Usage

### Basic Example

```go
package main

import (
    "fmt"
    "log"
    "pebbledb"
)

func main() {
    // Open database
    config := &pebbledb.Config{
        DataDir:          "./data",
        MemtableSize:     1000,
        CompactionPeriod: 60, // seconds
    }
    
    db, err := pebbledb.Open(config)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Put data
    db.Put("user:1", []byte("Alice"))
    
    // Get data
    value, _ := db.Get("user:1")
    fmt.Println(string(value)) // Output: Alice
    
    // Delete data
    db.Delete("user:1")
}
```

### Running Tests

```bash
cd pebbledb
go test -v
```

### Running Example

```bash
cd pebbledb/example
go run main.go
```

## Features

- ✅ WAL for durability
- ✅ In-memory memtable with automatic flushing
- ✅ SSTable format for disk storage
- ✅ Automatic compaction
- ✅ Tombstone-based deletion
- ✅ Persistence across restarts
- ✅ Thread-safe operations

## Limitations

This is a minimalistic educational implementation. Production databases would include:
- Bloom filters for faster lookups
- Level-based compaction strategy
- Block cache for SSTable reads
- Compression
- Recovery from WAL on startup
- Better error handling and monitoring