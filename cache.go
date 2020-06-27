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
		sigolo.Debug("Cache has content for '%s' already loaded", hashValue)

		file, err := os.Open(c.folder + hashValue)
		if err != nil {
			sigolo.Error("Error reading cached file '%s': %s", hashValue, err)
			return nil, err
		}

		response = file
	}else {
		response = bytes.NewReader(content)
	}

	return &response, nil
}

func (c *Cache) put(key string, content *io.Reader) error {
	hashValue := calcHash(key)

	file, err := os.Create(c.folder + hashValue)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	bytesWritten, err := io.Copy(writer, *content)
	if err != nil {
		return err
	}

	// Make sure, that the RAM-cache only holds values we were able to write.
	// This is a decision to prevent a false impression of the cache: If the
	// write fails, the cache isn't working correctly, which should be fixed by
	// the user of this cache.
	// TODO make cache element size configurable
	if bytesWritten <= 5 * 1024 * 1024 {
		sigolo.Debug("Add %s into in-memory cache", hashValue)
		buffer := &bytes.Buffer{}
		_, err := io.Copy(buffer, *content)
		if err != nil {
			return err
		}

		c.mutex.Lock()
		c.knownValues[hashValue] = buffer.Bytes()
		c.mutex.Unlock()
	}

	sigolo.Debug("Cache wrote content into '%s'", hashValue)

	return err
}

func calcHash(data string) string {
	sha := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sha[:])
}
