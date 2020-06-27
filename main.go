package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hauke96/sigolo"
)

const configPath = "./tiny.json"

var config *Config
var cache *Cache

var client *http.Client

func main() {
	loadConfig()
	if config.DebugLogging {
		sigolo.LogLevel = sigolo.LOG_DEBUG
	}
	sigolo.Debug("Config loaded")

	prepare()
	sigolo.Debug("Cache initialized")

	server := &http.Server{
		Addr:         ":" + config.Port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	sigolo.Info("Start serving...")
	err := server.ListenAndServe()
	if err != nil {
		sigolo.Fatal(err.Error())
	}
}

func loadConfig() {
	var err error

	config, err = LoadConfig(configPath)
	if err != nil {
		sigolo.Fatal("Could not read config: '%s'", err.Error())
	}
}

func prepare() {
	var err error

	cache, err = CreateCache(config.CacheFolder)

	if err != nil {
		sigolo.Fatal("Could not init cache: '%s'", err.Error())
	}

	client = &http.Client{
		Timeout: time.Second * 30,
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	fullUrl := r.URL.Path + "?" + r.URL.RawQuery

	sigolo.Info("Requested '%s'", fullUrl)

	// Only pass request to target host when cache does not has an entry for the
	// given URL.
	if cache.has(fullUrl) {
		content, err := cache.get(fullUrl)

		if err != nil {
			handleError(err, w)
		} else {
			_, err := io.Copy(w, *content)
			if err != nil {
				sigolo.Error("Error writing response: %s", err.Error())
			}
		}
	} else { // Cache miss
		response, err := client.Get(config.Target + fullUrl)
		if err != nil {
			handleError(err, w)
			return
		}

		var reader io.Reader
		reader = response.Body

		err = cache.put(fullUrl, &reader)
		if err != nil {
			handleError(err, w)
			return
		}
		defer response.Body.Close()

		// Do not fail. Even if the put failed, the end user would be sad if he
		// gets an error, even if the proxy alone works.
		if err != nil {
			sigolo.Error("Could not write into cache")
			handleError(err, w)
			return
		}

		content, err := cache.get(fullUrl)
		_, err = io.Copy(w, *content)
		if err != nil {
			handleError(err, w)
			return
		}

	}
}

func handleError(err error, w http.ResponseWriter) {
	sigolo.Error(err.Error())
	w.WriteHeader(500)
	fmt.Fprintf(w, err.Error())
}
