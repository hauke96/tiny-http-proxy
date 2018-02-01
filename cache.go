package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/ioutil"
	"sync"
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
		Error.Printf("Error opening cache folder '%s':\n", path)
		return nil, err
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

func (c *Cache) get(key string) ([]byte, error) {
	hashValue := calcHash(key)

	// Try to get content. Error if not found.
	c.mutex.Lock()
	content, ok := c.knownValues[hashValue]
	c.mutex.Unlock()
	if !ok {
		Debug.Printf("Cache doen't know key '%s'", hashValue)
		return nil, errors.New(fmt.Sprintf("Key '%s' is not known to cache", hashValue))
	}

	Debug.Printf("Cache has key '%s'", hashValue)

	// Key is known, but not loaded into RAM
	if content == nil {
		Debug.Printf("Cache has content for '%s' already loaded", hashValue)

		content, err := ioutil.ReadFile(c.folder + hashValue)
		if err != nil {
			Error.Printf("Error reading cached file '%s'", hashValue)
			return nil, err
		}

		c.mutex.Lock()
		c.knownValues[hashValue] = content
		c.mutex.Unlock()
	}

	return content, nil
}

func (c *Cache) put(key string, content []byte) error {
	hashValue := calcHash(key)

	err := ioutil.WriteFile(c.folder+hashValue, content, 0644)

	// Make sure, that the RAM-cache only holds values we were able to write.
	// This is a decision to prevent a false impression of the cache: If the
	// write fails, the cache isn't working correctly, which should be fixed by
	// the user of this cache.
	if err == nil {
		Debug.Printf("Cache wrote content into '%s'", hashValue)
		c.mutex.Lock()
		c.knownValues[hashValue] = content
		c.mutex.Unlock()
	}

	return err
}

func calcHash(data string) string {
	sha := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sha[:])
}
