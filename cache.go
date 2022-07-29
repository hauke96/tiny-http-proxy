package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
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

func CreateCache() (*Cache, error) {
	cacheFolder, err := h.CheckDirAndCreate(config.CacheFolder, "CreateCache HTTP")
	if err != nil {
		olo.Fatal("Cache.CreateCache(): Error: %s", err.Error())
		return nil, err
	}
	CacheFolderHTTPS, err := h.CheckDirAndCreate(config.CacheFolderHTTPS, "CreateCache HTTPS")
	if err != nil {
		olo.Fatal("Cache.CreateCache(): Error: %s", err.Error())
		return nil, err
	}

	mutex := &sync.Mutex{}
	busy := make(map[string]*sync.Mutex)
	memory := make(map[string]CacheMemoryItem)

	// Go through every file an save its name in the map. The content of the file
	// is loaded when needed. This makes sure that we don't have to read
	// the directory content each time the user wants data that's not yet loaded.
	prefillCache := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		olo.Debug("prefill cache with path: %s", path)
		urlScheme := "http://"
		cfTrim := cacheFolder
		if strings.HasPrefix(path, CacheFolderHTTPS) {
			urlScheme = "https://"
			cfTrim = CacheFolderHTTPS
		}
		cachedItem := strings.TrimPrefix(path, cfTrim)
		cachedItem, err = url.QueryUnescape(cachedItem)
		if err != nil {
			olo.Fatal("While url decode file from cache %s Error: %s", path, err.Error())
			return err
		}
		cachedItem = urlScheme + cachedItem

		if config.PrefillCacheOnStartup {
			file, err := os.Open(path)
			if err != nil {
				olo.Fatal("Error reading cached file '%s': %s", path, err)
				return err
			}
			defer file.Close()

			fi, err := file.Stat()
			if err != nil {
				olo.Fatal("Error stating cached file '%s': %s", path, err)
				return err
			}
			if fi.Size() <= config.MaxCacheItemSize*1024*1024 {
				buffer := &bytes.Buffer{}
				size, err := io.Copy(buffer, file)
				if err != nil {
					return err
				}
				mutex.Lock()
				memory[cachedItem] = CacheMemoryItem{content: buffer.Bytes(), loadedAt: time.Now()}
				mutex.Unlock()

				olo.Debug("prefillCache: Added %s for %s back into in-memory cache with size of %d", path, cachedItem, size)
			} else {
				olo.Debug("prefillCache: Added %s for %s back into known cache items, but will only read it from files as size is %d", path, cachedItem, fi.Size())
			}
		}

		return nil
	}

	channel := make(chan error)
	olo.Debug("filepath.Walk'ing directory " + cacheFolder)
	go func() { channel <- filepath.WalkDir(cacheFolder, prefillCache) }()
	olo.Debug("filepath.Walk'ing directory " + CacheFolderHTTPS)
	go func() { channel <- filepath.WalkDir(CacheFolderHTTPS, prefillCache) }()
	<-channel // Walk done

	if !config.PrefillCacheOnStartup {
		olo.Info("prefillCache: Not prefilling in-memory cache with content, because prefill_cache_on_startup in config is set to false")
	}
	// for _, info := range fileInfos {
	// if !info.IsDir() {
	// memory[info.Name()] = KnownValues{}
	// }
	// }

	cache := &Cache{
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
	olo.Debug("checking cache for requestedURL %s", requestedURL)

	// If the resource is busy, wait for it to be free. This is the case if
	// the resource is currently being cached as a result of another request.
	// Also, release the lock on the cache to allow other readers while waiting
	if lock, busy := c.busyItems[requestedURL]; busy {
		c.mutex.Unlock()
		lock.Lock()
		lock.Unlock()
		c.mutex.Lock()
	}

	// fmt.Printf("%+v\n", c.cacheMemoryItems)

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

func (c *Cache) get(requestedURL string, defaultCacheTTL time.Duration, invalidateCache bool) (CacheResponse, error) {
	cacheURL, err := removeSchemeFromURL(requestedURL)
	if err != nil {
		return CacheResponse{}, err
	}
	// Try to get content. Error if not found.
	c.mutex.Lock()
	cacheMemoryItem, ok := c.cacheMemoryItems[requestedURL]
	c.mutex.Unlock()

	if !ok && len(cacheMemoryItem.content) > 0 {
		olo.Debug("Cache item not found: '%s'", cacheURL)
		return CacheResponse{}, fmt.Errorf("cache item '%s' is not known", cacheURL)
	}
	cacheFolder := config.CacheFolder
	if strings.HasPrefix(requestedURL, "https://") {
		cacheFolder = config.CacheFolderHTTPS
	}
	urlParts := strings.SplitN(requestedURL, "/", 4)

	fileCacheDir := filepath.Join(cacheFolder, urlParts[2])
	uriEncoded := url.QueryEscape(urlParts[3])
	cacheFile := filepath.Join(fileCacheDir, uriEncoded)

	// check if Cache is too old based on mtime, if so call getRemote() and renew cache
	err = checkCacheTTL(cacheFile, requestedURL, defaultCacheTTL, invalidateCache)
	if err != nil {
		return CacheResponse{}, err
	}

	// Key is known, but not found in-memory, read from file
	if cacheMemoryItem.content == nil {
		olo.Debug("Cache item '%s' is known but is not stored in memory. Reading from file: %s", cacheURL, cacheFile)

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
		if fi.Size() <= config.MaxCacheItemSize*1024*1024 {
			// read content of file back into memory
			err = c.fillInMemoryCacheWithFileContent(cacheFile, requestedURL)
			if err != nil {
				olo.Error("Error while reading cached file content back into memory '%s': %s", cacheFile, err)
				return CacheResponse{}, err
			}
		}
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

func (c *Cache) put(requestedURL string, content *io.Reader, contentLength int64) error {
	// make sure cache directories exist
	cacheFolder := config.CacheFolder
	if strings.HasPrefix(requestedURL, "https://") {
		cacheFolder = config.CacheFolderHTTPS
	}
	urlParts := strings.SplitN(requestedURL, "/", 4)
	olo.Debug("adding to cache folder %s the url part 2 %s", cacheFolder, urlParts[2])
	fileCacheDir := filepath.Join(cacheFolder, urlParts[2])
	_, err := h.CheckDirAndCreate(fileCacheDir, "Cache.put")
	if err != nil {
		olo.Fatal("Cache.put(): while trying to serve cacheURL %s : Error: %s", requestedURL, err.Error())
		return err
	}

	uriEncoded := url.QueryEscape(urlParts[3])
	cacheFile := filepath.Join(fileCacheDir, uriEncoded)

	// always write to file
	file, err := os.Create(cacheFile)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)
	written, err := io.Copy(writer, *content)
	if err != nil {
		return err
	}
	olo.Debug("Wrote content size %d of entry %s into file %s", written, requestedURL, cacheFile)

	if contentLength <= config.MaxCacheItemSize*1024*1024 {
		// write to in-memory
		olo.Debug("Content size of %s is %d not larger than max_cache_item_size_in_mb %d so we write it also into a memory buffer", requestedURL, contentLength, config.MaxCacheItemSize*1024*1024)
		// Small enough to put it into the in-memory cache
		err = c.fillInMemoryCacheWithFileContent(cacheFile, requestedURL)
		if err != nil {
			return err
		}
	} else {
		defer c.release(requestedURL, nil, time.Now())
	}

	return nil
}

func checkCacheTTL(filePath string, requestedURL string, defaultCacheTTL time.Duration, invalidateCache bool) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		promCounters["CACHE_ITEM_MISSING"].Inc()
		_, err := GetRemote(requestedURL)
		if err != nil {
			return err
		}
		olo.Error("found cache item while starting service, but it was removed afterwards, trying to get it again: '%s'", requestedURL)
		olo.Fatal("found cache item while starting service, but it was removed afterwards, trying to get it again: '%s'", requestedURL)
		err = checkCacheTTL(filePath, requestedURL, defaultCacheTTL, invalidateCache)
		if err != nil {
			return err
		}
		return nil
	}
	mtime := fi.ModTime()

	ttl := defaultCacheTTL
	for name, cr := range config.CacheRules {
		r := regexp.MustCompile(cr.Regex)
		// olo.Debug("comparing regex rule: '%s' with regex '%s' with cacheURL: '%s'", name, cr.Regex, cacheURL)
		if r.MatchString(requestedURL) {
			olo.Debug("found matching regex rule: '%s' with regex '%s' and ttl '%s' for requestedURL: '%s'", name, cr.Regex, cr.TTL, requestedURL)
			ttl = cr.TTL
			// olo.Debug("setting ttl to '%s' for file '%s'", ttl, cacheURL)
			break
		}
	}

	olo.Debug("using cache TTL '%s' for file: '%s'", ttl, requestedURL)
	validUntil := mtime.Add(ttl)

	//valid := time.Now().AddDate(1, 0, 0)
	//fmt.Println(validUntil)
	// olo.Info("cacheURL:", cacheURL)
	// olo.Info("requestedURL:", requestedURL)
	if time.Now().After(validUntil) || invalidateCache {
		if invalidateCache {
			olo.Info("CACHE_INVALIDATE for requested URL '%s'", requestedURL)
			promCounters["CACHE_INVALIDATE"].Inc()
		} else {
			olo.Info("CACHE_TOO_OLD for requested URL '%s'", requestedURL)
			promCounters["CACHE_TOO_OLD"].Inc()
		}
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
	olo.Info("CACHE_OK until '%s'/'%s' for requested URL '%s'", time.Until(validUntil), validUntil.Format("2006-01-02 15:04:05"), requestedURL)
	promCounters["CACHE_OK"].Inc()
	return nil
}

func (c *Cache) fillInMemoryCacheWithFileContent(file string, requestedURL string) error {
	source, err := os.Open(file)
	if err != nil {
		return err
	}
	defer source.Close()
	buffer := &bytes.Buffer{}
	size, err := io.Copy(buffer, source)
	if err != nil {
		return err
	}

	olo.Debug("Added %s for %s back into in-memory cache with size of %d", file, requestedURL, size)
	defer c.release(requestedURL, buffer.Bytes(), time.Now())
	return nil
}
