package pebbledb

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WAL represents a Write-Ahead Log
type WAL struct {
	file   *os.File
	writer *bufio.Writer
	path   string
}

// NewWAL creates a new WAL file
func NewWAL(dir string, id int) (*WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	path := filepath.Join(dir, fmt.Sprintf("wal-%d.log", id))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	return &WAL{
		file:   file,
		writer: bufio.NewWriter(file),
		path:   path,
	}, nil
}

// Write appends an entry to the WAL
func (w *WAL) Write(entry *Entry) error {
	// Format: keyLen(4) + key + valueLen(4) + value + timestamp(8) + deleted(1)
	keyLen := uint32(len(entry.Key))
	valueLen := uint32(len(entry.Value))

	// Write key length
	if err := binary.Write(w.writer, binary.LittleEndian, keyLen); err != nil {
		return err
	}

	// Write key
	if _, err := w.writer.Write([]byte(entry.Key)); err != nil {
		return err
	}

	// Write value length
	if err := binary.Write(w.writer, binary.LittleEndian, valueLen); err != nil {
		return err
	}

	// Write value
	if _, err := w.writer.Write(entry.Value); err != nil {
		return err
	}

	// Write timestamp
	if err := binary.Write(w.writer, binary.LittleEndian, entry.Timestamp); err != nil {
		return err
	}

	// Write deleted flag
	deleted := byte(0)
	if entry.Deleted {
		deleted = 1
	}
	if err := w.writer.WriteByte(deleted); err != nil {
		return err
	}

	return w.writer.Flush()
}

// Close closes the WAL file
func (w *WAL) Close() error {
	if err := w.writer.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// Delete removes the WAL file
func (w *WAL) Delete() error {
	w.Close()
	return os.Remove(w.path)
}

// ReadWAL reads all entries from a WAL file
func ReadWAL(path string) ([]*Entry, error) {
	file, err := os.Open(path)
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

func readEntry(reader *bufio.Reader) (*Entry, error) {
	// Read key length
	var keyLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &keyLen); err != nil {
		return nil, err
	}

	// Read key
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}

	// Read value length
	var valueLen uint32
	if err := binary.Read(reader, binary.LittleEndian, &valueLen); err != nil {
		return nil, err
	}

	// Read value
	value := make([]byte, valueLen)
	if _, err := io.ReadFull(reader, value); err != nil {
		return nil, err
	}

	// Read timestamp
	var timestamp int64
	if err := binary.Read(reader, binary.LittleEndian, &timestamp); err != nil {
		return nil, err
	}

	// Read deleted flag
	deletedByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	return &Entry{
		Key:       string(key),
		Value:     value,
		Timestamp: timestamp,
		Deleted:   deletedByte == 1,
	}, nil
}
