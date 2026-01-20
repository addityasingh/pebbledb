package pebbledb

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// SSTable represents a Sorted String Table on disk
type SSTable struct {
	path  string
	id    int
	index map[string]int64 // In-memory index: key -> file offset
}

// NewSSTable creates a new SSTable from sorted entries
func NewSSTable(dir string, id int, entries []*Entry) (*SSTable, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("sstable-%d.db", id))
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSTable file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	index := make(map[string]int64)
	offset := int64(0)

	// Write entries to file and build index
	for _, entry := range entries {
		index[entry.Key] = offset

		// Format: keyLen(4) + key + valueLen(4) + value + timestamp(8) + deleted(1)
		keyLen := uint32(len(entry.Key))
		valueLen := uint32(len(entry.Value))

		// Write key length
		if err := binary.Write(writer, binary.LittleEndian, keyLen); err != nil {
			return nil, err
		}
		offset += 4

		// Write key
		if _, err := writer.Write([]byte(entry.Key)); err != nil {
			return nil, err
		}
		offset += int64(keyLen)

		// Write value length
		if err := binary.Write(writer, binary.LittleEndian, valueLen); err != nil {
			return nil, err
		}
		offset += 4

		// Write value
		if _, err := writer.Write(entry.Value); err != nil {
			return nil, err
		}
		offset += int64(valueLen)

		// Write timestamp
		if err := binary.Write(writer, binary.LittleEndian, entry.Timestamp); err != nil {
			return nil, err
		}
		offset += 8

		// Write deleted flag
		deleted := byte(0)
		if entry.Deleted {
			deleted = 1
		}
		if err := writer.WriteByte(deleted); err != nil {
			return nil, err
		}
		offset += 1
	}

	if err := writer.Flush(); err != nil {
		return nil, err
	}

	return &SSTable{
		path:  path,
		id:    id,
		index: index,
	}, nil
}

// LoadSSTable loads an existing SSTable from disk
func LoadSSTable(path string, id int) (*SSTable, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	index := make(map[string]int64)
	offset := int64(0)

	// Build index by reading through the file
	for {
		// Read key length
		var keyLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &keyLen); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Read key
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(reader, key); err != nil {
			return nil, err
		}

		index[string(key)] = offset

		// Skip value length, value, timestamp, and deleted flag
		var valueLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &valueLen); err != nil {
			return nil, err
		}

		// Skip value
		if _, err := reader.Discard(int(valueLen)); err != nil {
			return nil, err
		}

		// Skip timestamp (8 bytes) and deleted flag (1 byte)
		if _, err := reader.Discard(9); err != nil {
			return nil, err
		}

		offset += int64(4 + keyLen + 4 + valueLen + 8 + 1)
	}

	return &SSTable{
		path:  path,
		id:    id,
		index: index,
	}, nil
}

// Get retrieves an entry from the SSTable
func (s *SSTable) Get(key string) (*Entry, error) {
	offset, ok := s.index[key]
	if !ok {
		return nil, nil // Key not found
	}

	file, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Seek to the offset
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	return readEntry(reader)
}

// GetAll returns all entries in the SSTable
func (s *SSTable) GetAll() ([]*Entry, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	entries := []*Entry{}

	for {
		entry, err := readEntry(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetKeys returns all keys in sorted order
func (s *SSTable) GetKeys() []string {
	keys := make([]string, 0, len(s.index))
	for k := range s.index {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Delete removes the SSTable file from disk
func (s *SSTable) Delete() error {
	return os.Remove(s.path)
}

// Path returns the file path of the SSTable
func (s *SSTable) Path() string {
	return s.path
}
