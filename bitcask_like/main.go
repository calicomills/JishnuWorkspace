package main

import (
	"bufio"
	"container/list"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var DATA_FILE = "data"
var DATA_REF_FILE = "reference"

//var keyRefMap, _ = loadRefToMemory(DATA_REF_FILE)

var cache = NewLRUCache(100)

// LRUCache represents the Least Recently Used cache.
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	list     *list.List
}

// entry represents a key-value pair in the cache.
type entry struct {
	key   string
	value int64
}

// NewLRUCache creates a new LRUCache instance with the given capacity.
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		list:     list.New(),
	}
}

// Get retrieves the value associated with the given key from the cache.
func (lru *LRUCache) Get(key string) int64 {
	if elem, ok := lru.cache[key]; ok {
		lru.list.MoveToFront(elem)
		return elem.Value.(*entry).value
	}
	return -1
}

// Put inserts a new key-value pair into the cache.
func (lru *LRUCache) Put(key string, value int64) {
	// Check if the key already exists
	if elem, ok := lru.cache[key]; ok {
		// Update the value and move the element to the front of the list
		elem.Value.(*entry).value = value
		lru.list.MoveToFront(elem)
	} else {
		fmt.Printf("%s written successfully.\n", key)
		// Create a new entry
		newEntry := &entry{key, value}
		// Add the new entry to the cache
		elem := lru.list.PushFront(newEntry)
		lru.cache[key] = elem
		// Check if the cache has reached its capacity
		if len(lru.cache) > lru.capacity {
			// Remove the least recently used entry
			last := lru.list.Back()
			delete(lru.cache, last.Value.(*entry).key)
			lru.list.Remove(last)
			fmt.Printf("%s removed successfully.\n", last.Value.(*entry).key)
		}
	}
}

func main() {

	// Initialise Database
	initDB()

	loadRefToLRU(DATA_REF_FILE)

	// Define your routes and handlers
	http.HandleFunc("/get", handleGet)

	http.HandleFunc("/put", handlePut)

	// Start the server
	fmt.Println("Server listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/get" {
		http.NotFound(w, r)
		return
	}

	fmt.Println("get written successfully.")
	// Extract the key from the URL query parameter
	key := r.URL.Query().Get("key")

	// Check if the key is empty
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	// Check if the key exists in the keyRefMap
	// value, ok := keyRefMap[key]
	value := cache.Get(key)

	if value == -1 {
		// Read the record from the database using the provided key
		value, err := readKeyPos(DATA_REF_FILE, key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading record: %v", err), http.StatusInternalServerError)
			return
		}
		value2, err := readRecord(DATA_FILE, value)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading record: %v", err), http.StatusInternalServerError)
			return
		}
		parsedValues := parseData(value2)
		// Write the value to the response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": key, "value": parsedValues[1]})
		return

	}
	fmt.Println("Cache hit!.")
	value2, err := readRecord(DATA_FILE, value)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading record: %v", err), http.StatusInternalServerError)
		return
	}
	parsedValues := parseData(value2)
	// Write the value to the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"key": key, "value": parsedValues[1]})
}

func handlePut(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/put" {
		http.NotFound(w, r)
		return
	}
	fmt.Println("put written successfully.")
	// Decode the request body into a map[string]string
	key := r.URL.Query().Get("key")
	value := r.URL.Query().Get("value")

	// Check if key or value is empty
	if key == "" || value == "" {
		http.Error(w, "Key and value are required", http.StatusBadRequest)
		return
	}

	// If key is already present update it, else add new
	exists, _ := readKeyPos(DATA_REF_FILE, key)

	if exists != -1 {
		// Acquire a lock to ensure atomicity
		lock, err := acquireLock()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error acquiring lock: %v", err), http.StatusInternalServerError)
			return
		}
		defer lock.Close() // Release the lock when the function exits

		// Write the key-value pair to the database
		pos, err := writeRecord(key, value, DATA_FILE)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error writing record: %v", err), http.StatusInternalServerError)
			return
		}
		// keyRefMap[key] = pos
		cache.Put(key, pos)

		// update the key reference to the reference file
		err = updateReference(key, pos, DATA_REF_FILE)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error writing reference: %v", err), http.StatusInternalServerError)

			// If updating reference fails, rollback by deleting the written record
			rollbackErr := rollbackWriteRecord(DATA_FILE)
			if rollbackErr != nil {
				http.Error(w, fmt.Sprintf("Error rolling back record: %v", rollbackErr), http.StatusInternalServerError)
				return
			}

			http.Error(w, fmt.Sprintf("Error writing reference: %v", err), http.StatusInternalServerError)
			return
		}

	} else {
		lock, err := acquireLock()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error acquiring lock: %v", err), http.StatusInternalServerError)
			return
		}
		defer lock.Close() // Release the lock when the function exits
		// Write the key-value pair to the database
		pos, err := writeRecord(key, value, DATA_FILE)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error writing record: %v", err), http.StatusInternalServerError)
			return
		}
		// keyRefMap[key] = pos
		cache.Put(key, pos)

		// Write the key reference to the reference file
		_, err = writeReference(key, pos, DATA_REF_FILE)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error writing reference: %v", err), http.StatusInternalServerError)
			return
		}

	}

	// Write a success response
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Record written successfully.")
}

func acquireLock() (io.Closer, error) {
	// Open or create a lock file
	lockFile, err := os.OpenFile("lockfile.lock", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, err
	}

	// Try to obtain an exclusive lock
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		lockFile.Close()
		return nil, err
	}

	// Return a closer that unlocks the file when closed
	return lockFile, nil
}

func rollbackWriteRecord(fileName string) error {
	// Open the file for reading and writing
	file, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("error opening file for rollback: %w", err)
	}
	defer file.Close()

	// Move the file pointer to the end of the file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("error seeking to end of file: %w", err)
	}

	// Find the position of the last newline character
	pos, err := file.Seek(-1, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("error seeking to find last newline: %w", err)
	}

	// Move the file pointer to the position before the last newline character
	_, err = file.Seek(pos, io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking to position before last newline: %w", err)
	}

	// Truncate the file at this position
	err = file.Truncate(pos)
	if err != nil {
		return fmt.Errorf("error truncating file: %w", err)
	}

	fmt.Println("Rollback successful.")
	return nil
}

func initDB() {
	// open data record file and build ref file or load to memory
	filename := DATA_FILE
	refFilename := DATA_REF_FILE

	// Check if the file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// File does not exist, create it
		fmt.Println("Creating data record:", filename)
		if err := os.WriteFile(filename, []byte(""), 0644); err != nil {
			fmt.Println("Error creating file:", err)
			return
		}
		fmt.Println("File created successfully.")
	} else {
		fmt.Println("File already exists.")
		// Built the reference file.
		// Read the content of the file
		fmt.Println("Reading file:", filename)
		inputFile, err := os.Open(filename)
		if err != nil {
			fmt.Println("Error opening input file:", err)
			return
		}
		defer inputFile.Close()

		// Create the output file
		outputFile, err := os.Create(refFilename)
		if err != nil {
			fmt.Println("Error creating output file:", err)
			return
		}
		defer outputFile.Close()

		// Create a scanner to read the input file line by line
		scanner := bufio.NewScanner(inputFile)

		// Write each line to the output file along with its position
		var position int64 = 0
		for scanner.Scan() {
			line := scanner.Text()
			parsedData := parseData(line)

			err = updateReference(parsedData[0], position, DATA_REF_FILE)
			if err != nil {
				fmt.Println("Error writing to reference file:", err)
				return
			}
			// Write the line to the output file
			// _, err := outputFile.WriteString(fmt.Sprintf("%s=%d\n", parsedData[0], position))
			// if err != nil {
			// 	fmt.Println("Error writing to output file:", err)
			// 	return
			// }

			// Update the position
			position += int64(len(line) + 1) // Add 1 for the newline character
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading input file:", err)
			return
		}

		fmt.Println("Data written to output file successfully.")
	}
}

func readKeyPos(fileName string, key string) (int64, error) {
	// Open the file
	file, err := os.Open(fileName)
	if err != nil {
		return -1, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Initialize position counter
	var position int64 = 0

	// Loop through each line in the file
	for scanner.Scan() {
		line := scanner.Text()
		// Compare line with the key
		if strings.HasPrefix(line, key) {
			// If there's a match, extract the value and return it along with the position
			parts := strings.SplitN(line, "=", 2)
			num, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				fmt.Println("Error:", err)
				return -1, fmt.Errorf("error opening file: %w", err)
			}
			if len(parts) == 2 {
				return num, nil
			}
		}
		// Update the position for the next line
		position += int64(len(line) + 1) // Add 1 for the newline character
	}

	// Check for any scanner errors
	if err := scanner.Err(); err != nil {
		return -1, fmt.Errorf("error scanning file: %w", err)
	}

	// If the key is not found, return an error
	return -1, fmt.Errorf("key not found")
}

func readRecord(fileName string, pos int64) (string, error) {
	// Open the file
	file, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close() // Close the file when done

	// Seek to the specified position in the file
	_, err = file.Seek(pos, 0)
	if err != nil {
		return "", fmt.Errorf("error seeking in file: %w", err)
	}

	// Create a new buffered reader
	reader := bufio.NewReader(file)

	// Read until the end of the line
	line, err := reader.ReadBytes('\n')
	if err != nil && err.Error() != "EOF" {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	return string(line), nil
}

func parseData(data string) []string {
	data = strings.ReplaceAll(data, "\n", "")
	parts := strings.Split(data, "=")
	if len(parts) < 2 {
		// Handle case where data does not contain comma-separated key-value pair
		return nil
	}
	key := parts[0]
	value := parts[1]
	return []string{key, value}
}

func writeRecord(key string, value string, fileName string) (int64, error) {
	// Open the file with append mode, create it if it doesn't exist, and give write permissions
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close() // Close the file when done

	// Get the current offset in the file
	pos, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("error getting current position in file: %w", err)
	}

	// Write key-value pair to the file
	_, err = fmt.Fprintf(file, "%s=%s\n", key, value)
	if err != nil {
		return 0, fmt.Errorf("error writing to file: %w", err)
	}

	fmt.Println("Data written successfully.")
	return pos, nil
}

func writeReference(key string, offset int64, fileName string) (int64, error) {
	// Open the file with append mode, create it if it doesn't exist, and give write permissions
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return 0, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close() // Close the file when done

	// Write key-value pair to the file
	_, err = fmt.Fprintf(file, "%s=%s\n", key, strconv.FormatInt(offset, 10))
	if err != nil {
		return 0, fmt.Errorf("error writing to file: %w", err)
	}

	fmt.Println("Data written successfully.")
	return 0, err
}

func updateReference(key string, newValue int64, fileName string) error {
	// Open the file for reading and writing
	file, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Create a temporary file to store the modified content
	tmpFile, err := os.CreateTemp("", "temp")
	if err != nil {
		return fmt.Errorf("error creating temporary file: %w", err)
	}
	defer tmpFile.Close()

	keyUpdated := false

	// Iterate over each line in the file
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip lines without '=' delimiter
		}
		// Check if the key matches the line
		if parts[0] == key {
			// Write the updated line with the new value
			_, err := fmt.Fprintf(tmpFile, "%s=%s\n", key, strconv.FormatInt(newValue, 10))
			if err != nil {
				return fmt.Errorf("error writing to temporary file: %w", err)
			}
			keyUpdated = true
		} else {
			// Write the original line
			_, err := fmt.Fprintln(tmpFile, line)
			if err != nil {
				return fmt.Errorf("error writing to temporary file: %w", err)
			}
		}
	}

	// Check for any errors encountered during scanning
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	// If the key was not found and updated, append it to the file with the new value
	if !keyUpdated {
		_, err := fmt.Fprintf(tmpFile, "%s=%s\n", key, strconv.FormatInt(newValue, 10))
		if err != nil {
			return fmt.Errorf("error writing to temporary file: %w", err)
		}
	}

	// Close the original file
	if err := file.Close(); err != nil {
		return fmt.Errorf("error closing original file: %w", err)
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("error closing temporary file: %w", err)
	}

	// Rename the temporary file to overwrite the original file
	if err := os.Rename(tmpFile.Name(), fileName); err != nil {
		return fmt.Errorf("error renaming temporary file: %w", err)
	}

	fmt.Println("Reference updated successfully.")
	return nil
}

func loadRefToMemory(fileName string) (map[string]int64, error) {
	// Open the file
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Create a map to store key-value pairs
	refMap := make(map[string]int64)

	// Read the first 100 lines from the file
	lineCount := 0
	for scanner.Scan() && lineCount < 100 {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip lines without '=' delimiter
		}
		key := parts[0]
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing value: %w", err)
		}
		refMap[key] = value
		lineCount++
	}

	// Check for any errors encountered during scanning
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file: %w", err)
	}

	return refMap, nil
}

func loadRefToLRU(fileName string) (map[string]int64, error) {
	// Open the file
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Create a map to store key-value pairs
	refMap := make(map[string]int64)

	// Read the first 100 lines from the file
	lineCount := 0
	for scanner.Scan() && lineCount < 100 {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Skip lines without '=' delimiter
		}
		key := parts[0]
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing value: %w", err)
		}
		cache.Put(key, value)
		lineCount++
	}

	// Check for any errors encountered during scanning
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file: %w", err)
	}

	return refMap, nil
}
