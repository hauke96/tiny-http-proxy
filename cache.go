package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"sync"

	"github.com/hauke96/sigolo"
)

type Cache struct {
	folder      string
	hash        hash.Hash
	knownValues map[string][]byte
	mutex       *sync.Mutex
}

func CreateCache(path string) (*Cache, error) {
	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		sigolo.Error("Cannot open cache folder '%s': %s", path, err)
		sigolo.Info("Create cache folder '%s'", path)
		os.Mkdir(path, os.ModePerm)
	}

	values := make(map[string][]byte, 0)

	// Go through every file an save its name in the map. The content of the file
	// is loaded when needed. This makes sure that we don't have to read
	// the directory content each time the user wants data that's not yet loaded.
	for _, info := range fileInfos {
		if !info.IsDir() {
			values[info.Name()] = nil
		}
	}

	hash := sha256.New()

	mutex := &sync.Mutex{}

	cache := &Cache{
		folder:      path,
		hash:        hash,
		knownValues: values,
		mutex:       mutex,
	}

	return cache, nil
}

func (c *Cache) has(key string) bool {
	hashValue := calcHash(key)

	c.mutex.Lock()
	_, ok := c.knownValues[hashValue]
	c.mutex.Unlock()

	return ok
}

func (c *Cache) get(key string) (*io.Reader, error) {
	var response io.Reader
	hashValue := calcHash(key)

	// Try to get content. Error if not found.
	c.mutex.Lock()
	content, ok := c.knownValues[hashValue]
	c.mutex.Unlock()
	if !ok && len(content) > 0 {
		sigolo.Debug("Cache doesn't know key '%s'", hashValue)
		return nil, errors.New(fmt.Sprintf("Key '%s' is not known to cache", hashValue))
	}

	sigolo.Debug("Cache has key '%s'", hashValue)

	// Key is known, but not loaded into RAM
	if content == nil {
		sigolo.Debug("Cache item '%s' known but is not stored in memory. Using file.", hashValue)

		file, err := os.Open(c.folder + hashValue)
		if err != nil {
			sigolo.Error("Error reading cached file '%s': %s", hashValue, err)
			return nil, err
		}

		response = file

		sigolo.Debug("Create reader from file %s", hashValue)
	} else { // Key is known and data is already loaded to RAM
		response = bytes.NewReader(content)
		sigolo.Debug("Create reader from %d byte large cache content", len(content))
	}

	return &response, nil
}

func (c *Cache) put(key string, content *io.Reader, contentLength int64) error {
	hashValue := calcHash(key)

	// Small enough to put it into the in-memory cache
	if contentLength <= config.MaxCacheItemSize*1024*1024 {
		buffer := &bytes.Buffer{}
		_, err := io.Copy(buffer, *content)
		if err != nil {
			return err
		}

		c.mutex.Lock()
		c.knownValues[hashValue] = buffer.Bytes()
		c.mutex.Unlock()
		sigolo.Debug("Added %s into in-memory cache", hashValue)

		err = ioutil.WriteFile(c.folder+hashValue, buffer.Bytes(), 0644)
		if err != nil {
			return err
		}
		sigolo.Debug("Wrote content of entry %s into file", hashValue)
	} else { // Too large for in-memory cache, just write to file
		c.mutex.Lock()
		c.knownValues[hashValue] = nil
		c.mutex.Unlock()
		sigolo.Debug("Added nil-entry for %s into in-memory cache", hashValue)

		file, err := os.Create(c.folder + hashValue)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(file)
		_, err = io.Copy(writer, *content)
		if err != nil {
			return err
		}
		sigolo.Debug("Wrote content of entry %s into file", hashValue)
	}

	sigolo.Debug("Cache wrote content into '%s'", hashValue)

	return nil
}

func calcHash(data string) string {
	sha := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sha[:])
}
