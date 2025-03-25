// lwdb/lwdb.go
package lwdb

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// SimpleDB is a very basic key-value store. It is safe for concurrent access.
type SimpleDB struct {
	filePath string
	data     map[string]string
	mu       sync.RWMutex
}

// NewSimpleDB creates a new SimpleDB. The filePath is where the data will be persisted.
func NewSimpleDB(filePath string) *SimpleDB {
	db := &SimpleDB{
		filePath: filePath,
		data:     make(map[string]string),
	}
	db.Load() // Load existing data on startup
	return db
}

// Set sets the value for a given key.
func (db *SimpleDB) Set(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.data[key] = value
	return db.Save()
}

// Get retrieves the value for a given key.
func (db *SimpleDB) Get(key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	val, ok := db.data[key]
	return val, ok
}

// Delete removes the key-value pair for the given key.
func (db *SimpleDB) Delete(key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.data, key)
	return db.Save()
}

// Save persists the database to the file.
func (db *SimpleDB) Save() error {
	file, err := os.Create(db.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for key, val := range db.data {
		_, err := writer.WriteString(fmt.Sprintf("%s:%s\n", key, val))
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}

// Load loads the database from the file.
func (db *SimpleDB) Load() error {
	file, err := os.Open(db.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Data file not found. Starting with empty database.")
			return nil // File doesn't exist, start with empty data
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			db.data[parts[0]] = parts[1]
		} else {
			fmt.Println("Skipping invalid line:", line) // warn of invalid lines
		}
	}
	if err := scanner.Err(); err != nil { //check for errors during scanning.
		return err
	}
	return nil
}

// GetData returns a copy of the database's data.  This is safe for concurrent access.
func (db *SimpleDB) GetData() map[string]string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	// Create a new map to avoid direct access to the internal data.
	dataCopy := make(map[string]string)
	for k, v := range db.data {
		dataCopy[k] = v
	}
	return dataCopy
}
