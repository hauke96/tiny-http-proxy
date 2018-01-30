package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/ioutil"
)

type Cache struct {
	folder      string
	hash        hash.Hash
	knownValues map[string][]byte
}

func CreateCache(path string) (*Cache, error) {
	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		Error.Printf("Error opening cache folder '%s':\n", path)
		return nil, err
	}

	values := make(map[string][]byte, 0)

	for _, info := range fileInfos {
		if !info.IsDir() {
			values[info.Name()] = nil
		}
	}

	hash := sha256.New()

	cache := &Cache{
		folder:      path,
		hash:        hash,
		knownValues: values,
	}

	return cache, nil
}

func (c Cache) has(key string) bool {
	hashValue := hex.EncodeToString(c.hash.Sum([]byte(key)))

	_, ok := c.knownValues[hashValue]

	return ok
}

func (c Cache) get(key string) ([]byte, error) {
	hashValue := hex.EncodeToString(c.hash.Sum([]byte(key)))

	content, ok := c.knownValues[hashValue]

	// Key is not known
	if !ok {
		return nil, errors.New(fmt.Sprintf("Key '%s' is not known to cache", hashValue))
	}

	// Key is known, but not loaded into RAM
	if content == nil {
		content, err := ioutil.ReadFile(c.folder + hashValue)
		if err != nil {
			Error.Printf("Error reading cached file '%s'", hashValue)
			return nil, err
		}

		c.knownValues[hashValue] = content
	}

	return content, nil
}

func (c Cache) put(key string, content []byte) error {
	hashValue := hex.EncodeToString(c.hash.Sum([]byte(key)))

	c.knownValues[hashValue] = content

	return ioutil.WriteFile(c.folder+hashValue, content, 0644)
}
