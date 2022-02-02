package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	h "github.com/xorpaul/gohelper"
	olo "github.com/xorpaul/sigolo"
)

type Cache struct {
	folder           string
	busyItems        map[string]*sync.Mutex
	cacheMemoryItems map[string]CacheMemoryItem
	mutex            *sync.Mutex
}

type CacheMemoryItem struct {
	loadedAt time.Time
	content  []byte
}

type CacheResponse struct {
	loadedAt time.Time
	content  io.ReadSeeker
}

func CreateCache(cacheFolder string) (*Cache, error) {
	cacheFolder, err := h.CheckDirAndCreate(cacheFolder, "CreateCache")
	if err != nil {
		olo.Fatal("Cache.CreateCache(): Error: %s", err.Error())
		return nil, err
	}

	busy := make(map[string]*sync.Mutex)
	memory := make(map[string]CacheMemoryItem)

	// Go through every file an save its name in the map. The content of the file
	// is loaded when needed. This makes sure that we don't have to read
	// the directory content each time the user wants data that's not yet loaded.
	prefillCache := func(path string, info os.FileInfo, err error) error {
		if h.IsDir(path) || path == cacheFolder {
			return nil
		}
		// removing cache dir from path
		olo.Debug("path: %s", path)
		cachedItem := strings.TrimPrefix(path, cacheFolder)
		cachedItem, err = url.QueryUnescape(cachedItem)
		if err != nil {
			olo.Fatal("CreateCache(): while url decode file from cache %s Error: %s", path, err.Error())
			return err
		}
		olo.Debug("adding to cache: %s", cachedItem)
		memory[cachedItem] = CacheMemoryItem{}

		return nil
	}

	c := make(chan error)
	olo.Debug("filepath.Walk'ing directory " + cacheFolder)
	go func() { c <- filepath.Walk(cacheFolder, prefillCache) }()
	<-c // Walk done

	// for _, info := range fileInfos {
	// if !info.IsDir() {
	// memory[info.Name()] = KnownValues{}
	// }
	// }

	mutex := &sync.Mutex{}

	cache := &Cache{
		folder:           cacheFolder,
		busyItems:        busy,
		mutex:            mutex,
		cacheMemoryItems: memory,
	}

	return cache, nil
}

// Returns true if the resource is found, and false otherwise. If the
// resource is busy, this method will hang until the resource is free. If
// the resource is not found, a lock indicating that the resource is busy will
// be returned. Once the resource has been put into cache the busy lock *must*
// be unlocked to allow others to access the newly cached resource
func (c *Cache) has(requestedURL string) (*sync.Mutex, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// If the resource is busy, wait for it to be free. This is the case if
	// the resource is currently being cached as a result of another request.
	// Also, release the lock on the cache to allow other readers while waiting
	if lock, busy := c.busyItems[requestedURL]; busy {
		c.mutex.Unlock()
		lock.Lock()
		lock.Unlock()
		c.mutex.Lock()
	}

	// If a resource is in the shared cache, it can't be reserved. One can simply
	// access it directly from the cache
	if _, found := c.cacheMemoryItems[requestedURL]; found {
		return nil, true
	}

	// The resource is not in the cache, mark the resource as busy until it has
	// been cached successfully. Unlocking lock is required!
	lock := new(sync.Mutex)
	lock.Lock()
	c.busyItems[requestedURL] = lock
	return lock, false
}

func (c *Cache) get(requestedURL string) (CacheResponse, error) {
	cacheURL, err := removeSchemeFromURL(requestedURL)
	if err != nil {
		return CacheResponse{}, err
	}
	// Try to get content. Error if not found.
	c.mutex.Lock()
	cacheMemoryItem, ok := c.cacheMemoryItems[cacheURL]
	c.mutex.Unlock()

	if !ok && len(cacheMemoryItem.content) > 0 {
		olo.Debug("Cache item not found: '%s'", cacheURL)
		return CacheResponse{}, fmt.Errorf("cache item '%s' is not known", cacheURL)
	}

	urlParts := strings.SplitN(cacheURL, "/", 2)
	fileCacheDir := filepath.Join(c.folder, urlParts[0])
	uriEncoded := url.QueryEscape(urlParts[1])
	cacheFile := filepath.Join(fileCacheDir, uriEncoded)

	// check if Cache is too old based on mtime, if so call getRemote() and renew cache
	err = checkCacheTTL(cacheFile, cacheURL, requestedURL)
	if err != nil {
		return CacheResponse{}, err
	}

	// Key is known, but not found in-memory, read from file
	if cacheMemoryItem.content == nil {
		olo.Debug("Cache item '%s' known but is not stored in memory. Reading from file: %s", cacheURL, cacheFile)

		file, err := os.Open(cacheFile)
		if err != nil {
			olo.Error("Error reading cached file '%s': %s", cacheFile, err)
			return CacheResponse{}, err
		}

		fi, err := file.Stat()
		if err != nil {
			olo.Error("Error stating cached file '%s': %s", cacheFile, err)
			return CacheResponse{}, err
		}
		// TODO: neede by http.ServeContent otherwise:
		// seeker can't seek
		// file already closed
		// defer file.Close()

		promSummaries["CACHE_READ_FILE"].Observe(float64(fi.Size()))
		return CacheResponse{content: file, loadedAt: cacheMemoryItem.loadedAt}, nil

	}

	// Key is known and data is already loaded to RAM
	promSummaries["CACHE_READ_MEMORY"].Observe(float64(len(cacheMemoryItem.content)))
	content := bytes.NewReader(cacheMemoryItem.content)
	return CacheResponse{content: content, loadedAt: cacheMemoryItem.loadedAt}, nil

}

// release is an internal method which atomically caches an item and unmarks
// the item as busy, if it was busy before. The busy lock *must* be unlocked
// elsewhere!
func (c *Cache) release(requestedURL string, content []byte, loadedAt time.Time) {
	c.mutex.Lock()
	delete(c.busyItems, requestedURL)
	c.cacheMemoryItems[requestedURL] = CacheMemoryItem{content: content, loadedAt: loadedAt}
	c.mutex.Unlock()
}

func (c *Cache) put(cacheURL string, content *io.Reader, contentLength int64) error {
	// make sure cache directories exist
	urlParts := strings.SplitN(cacheURL, "/", 2)
	olo.Debug("adding to cache folder %s the url part 0 %s\n", c.folder, urlParts[0])
	fileCacheDir := filepath.Join(c.folder, urlParts[0])
	_, err := h.CheckDirAndCreate(fileCacheDir, "Cache.put")
	if err != nil {
		olo.Fatal("Cache.put(): while trying to serve cacheURL %s : Error: %s", cacheURL, err.Error())
		return err
	}

	uriEncoded := url.QueryEscape(urlParts[1])
	cacheFile := filepath.Join(fileCacheDir, uriEncoded)

	if contentLength <= config.MaxCacheItemSize*1024*1024 {
		// Small enough to put it into the in-memory cache
		buffer := &bytes.Buffer{}
		_, err := io.Copy(buffer, *content)
		if err != nil {
			return err
		}

		defer c.release(cacheURL, buffer.Bytes(), time.Now())
		olo.Debug("Added %s into in-memory cache", cacheURL)

		err = ioutil.WriteFile(cacheFile, buffer.Bytes(), 0644)
		if err != nil {
			return err
		}
	} else {
		// Too large for in-memory cache, just write to file
		defer c.release(cacheURL, nil, time.Now())

		file, err := os.Create(cacheFile)
		if err != nil {
			return err
		}

		writer := bufio.NewWriter(file)
		_, err = io.Copy(writer, *content)
		if err != nil {
			return err
		}
	}
	olo.Debug("Wrote content of entry %s into file %s", cacheURL, cacheFile)

	return nil
}

func checkCacheTTL(filePath string, cacheURL string, requestedURL string) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		promCounters["CACHE_ITEM_MISSING"].Inc()
		_, err := GetRemote(requestedURL)
		if err != nil {
			return err
		}
		olo.Error("found cache item while starting service, but it was removed afterwards, trying to get it again: '%s'", requestedURL)
		olo.Fatal("found cache item while starting service, but it was removed afterwards, trying to get it again: '%s'", requestedURL)
		err = checkCacheTTL(filePath, cacheURL, requestedURL)
		if err != nil {
			return err
		}
		return nil
	}
	mtime := fi.ModTime()

	ttl := config.DefaultCacheTTL
	for name, cr := range config.CacheRules {
		r := regexp.MustCompile(cr.Regex)
		// olo.Debug("comparing regex rule: '%s' with regex '%s' with cacheURL: '%s'", name, cr.Regex, cacheURL)
		if r.MatchString(cacheURL) {
			olo.Debug("found matching regex rule: '%s' with regex '%s' and ttl '%s' for cacheURL: '%s'", name, cr.Regex, cr.TTL, cacheURL)
			ttl = cr.TTL
			// olo.Debug("setting ttl to '%s' for file '%s'", ttl, cacheURL)
			break
		}
	}

	olo.Debug("using cache TTL '%s' for file: '%s'", ttl, cacheURL)
	validUntil := mtime.Add(ttl)

	//valid := time.Now().AddDate(1, 0, 0)
	//fmt.Println(validUntil)
	// olo.Info("cacheURL:", cacheURL)
	// olo.Info("requestedURL:", requestedURL)
	if time.Now().After(validUntil) {
		olo.Info("CACHE_TOO_OLD for requested URL '%s'", cacheURL)
		promCounters["CACHE_TOO_OLD"].Inc()
		_, err := GetRemote(requestedURL)
		if err != nil {
			if config.ReturnCacheIfRemoteFails {
				olo.Info("checking if remote " + requestedURL + " has a different/newer version failed, so provide the cached item as a fallback")
				return nil
			} else {
				return err
			}
		}
		return nil
	}
	olo.Info("CACHE_OK until '%s'/'%s' for requested URL '%s'", time.Until(validUntil), validUntil.Format("2006-01-02 15:04:05"), cacheURL)
	promCounters["CACHE_OK"].Inc()
	return nil
}
