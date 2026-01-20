# PebbleDB Architecture

## Overview

PebbleDB is a minimalistic LSM tree-based key-value store implemented in Go. It follows the Log-Structured Merge-tree architecture pattern, similar to LevelDB and RocksDB.

## Core Components

### 1. Entry (`types.go`)

The fundamental data structure representing a key-value pair:

```go
type Entry struct {
    Key       string
    Value     []byte
    Timestamp int64
    Deleted   bool  // Tombstone marker
}
```

### 2. Write-Ahead Log (WAL) (`wal.go`)

**Purpose**: Ensures durability by persisting writes before applying them to memory.

**File Format**:
```
[keyLen(4)] [key] [valueLen(4)] [value] [timestamp(8)] [deleted(1)]
```

**Operations**:
- `Write(entry)`: Appends an entry to the log
- `Close()`: Flushes and closes the WAL
- `Delete()`: Removes the WAL file after successful flush

**Lifecycle**:
- Created when DB opens or after memtable flush
- Deleted after memtable successfully flushed to SSTable

### 3. Memtable (`memtable.go`)

**Purpose**: In-memory sorted structure for fast recent writes.

**Implementation**: 
- Uses Go's `map[string]*Entry` for O(1) lookups
- Sorts keys when flushing to maintain SSTable ordering

**Operations**:
- `Put(entry)`: Adds/updates entry
- `Get(key)`: Retrieves entry
- `IsFull()`: Checks if size threshold reached
- `GetSortedEntries()`: Returns entries sorted by key (for flushing)

**Flush Trigger**: When size reaches `config.MemtableSize`

### 4. SSTable (`sstable.go`)

**Purpose**: Persistent on-disk sorted storage.

**File Format**: Same as WAL, but entries are sorted by key.

**In-Memory Index**:
```go
map[string]int64  // key -> file offset
```

**Operations**:
- `Get(key)`: O(1) index lookup + single disk seek
- `GetAll()`: Sequential scan of entire table
- `Delete()`: Removes SSTable file

**Creation**: When memtable is flushed to disk.

### 5. Database (`db.go`)

**Purpose**: Coordinates all components and provides main API.

**State**:
```go
type DB struct {
    memtable  *Memtable
    sstables  []*SSTable   // Ordered: oldest to newest
    wal       *WAL
    config    *Config
}
```

**Operations**:
- `Put(key, value)`: Write path
- `Get(key)`: Read path
- `Delete(key)`: Tombstone-based deletion
- `Close()`: Graceful shutdown with final flush

### 6. Compaction (`compaction.go`)

**Purpose**: Merges SSTables and removes deleted entries.

**Strategy**:
- Triggered periodically (configurable interval)
- Can be manually triggered via `CompactNow()`

**Process**:
1. Read all entries from all SSTables
2. Merge by key, keeping most recent timestamp
3. Filter out tombstones (deleted entries)
4. Create new compacted SSTable
5. Delete old SSTables

## Data Flow

### Write Path (Put/Delete)

```
1. Create Entry with timestamp
2. Write to WAL (durability)
3. Add to Memtable
4. If Memtable full:
   a. Get sorted entries
   b. Create SSTable on disk
   c. Clear Memtable
   d. Delete old WAL
   e. Create new WAL
```

### Read Path (Get)

```
1. Check Memtable
   └─ Found? Return (most recent)
2. Check SSTables (newest to oldest)
   └─ Found? Return
3. Return nil (not found)
```

**Note**: Tombstones are checked at each level. If found, return nil immediately.

### Compaction Flow

```
Periodic Timer → compact()
    ↓
Collect all entries from all SSTables
    ↓
Merge by key (keep latest timestamp)
    ↓
Filter deleted entries
    ↓
Create new SSTable
    ↓
Delete old SSTables
```

## Thread Safety

- `Memtable`: Uses `sync.RWMutex` for concurrent access
- `DB`: Uses `sync.RWMutex` to protect:
  - Memtable operations
  - SSTable list modifications
  - WAL operations

## File Layout

```
data/
├── wal-0.log          # Current WAL
├── sstable-0.db       # Oldest SSTable
├── sstable-1.db
└── sstable-2.db       # Newest SSTable
```

## Configuration

```go
type Config struct {
    DataDir          string  // Where to store data
    MemtableSize     int     // Max entries before flush
    CompactionPeriod int     // Seconds between compactions
}
```

## Trade-offs & Design Decisions

### Simplifications (vs production DBs)

1. **No Bloom Filters**: Every SSTable lookup requires disk I/O
2. **No Level-Based Compaction**: All SSTables at same level
3. **No Block Cache**: Repeated reads always hit disk
4. **No Compression**: Data stored uncompressed
5. **Simple Index**: Full key index in memory (can be large)
6. **No WAL Recovery**: On restart, only SSTables loaded

### Why This Design Works

- **Small-scale use cases**: Good for educational purposes and small datasets
- **Simple to understand**: Clear LSM tree principles without complexity
- **Functional**: Implements core durability and performance features

## Performance Characteristics

| Operation | Complexity | Notes |
|-----------|-----------|-------|
| Put       | O(1) amortized | WAL append + memtable insert |
| Get       | O(1) memtable + O(N) SSTables | N = number of SSTables |
| Delete    | O(1) amortized | Tombstone write |
| Flush     | O(M log M) | M = memtable size, sorting |
| Compact   | O(K) | K = total entries across SSTables |

## Future Enhancements

1. **Bloom Filters**: Reduce unnecessary SSTable lookups
2. **Leveled Compaction**: Organize SSTables into levels
3. **Block Cache**: LRU cache for frequently accessed blocks
4. **Compression**: Snappy/LZ4 compression
5. **WAL Recovery**: Replay WAL on startup
6. **Range Queries**: Scan support with iterators
7. **Snapshots**: Point-in-time consistent views
8. **Batch Writes**: Atomic multi-key operations
