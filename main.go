package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const configPath = "./tiny.json"

var config *Config
var cache *Cache

var client *http.Client

func main() {
	prepare()

	Info.Println("Ready to serve")

	server := &http.Server{
		Addr:         ":" + config.Port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	err := server.ListenAndServe()
	if err != nil {
		Error.Fatal(err.Error())
	}
}

func prepare() {
	var err error

	Info.Println("Load config")
	config, err = LoadConfig(configPath)
	if err != nil {
		Error.Fatalf("Could not read config: '%s'", err.Error())
	}

	Info.Println("Init cache")
	cache, err = CreateCache(config.CacheFolder)

	if err != nil {
		Error.Fatalf("Could not init cache: '%s'", err.Error())
	}

	client = &http.Client{
		Timeout: time.Second * 30,
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	fullUrl := r.URL.Path + "?" + r.URL.RawQuery

	Info.Printf("Requested '%s'\n", fullUrl)

	// Only pass request to target host when cache does not has an entry for the
	// given URL.
	if cache.has(fullUrl) {
		content, err := cache.get(fullUrl)

		if err != nil {
			handleError(err, w)
		} else {
			w.Write(content)
		}
	} else {
		response, err := client.Get(config.Target + fullUrl)
		if err != nil {
			handleError(err, w)
			return
		}

		body, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			handleError(err, w)
			return
		}

		err = cache.put(fullUrl, body)

		// Do not fail. Even if the put failed, the end user would be sad if he
		// gets an error, even if the proxy alone works.
		if err != nil {
			Error.Printf("Could not write into cache: %s", err)
		}

		w.Write(body)
	}
}

func handleError(err error, w http.ResponseWriter) {
	Error.Println(err.Error())
	w.WriteHeader(500)
	fmt.Fprintf(w, err.Error())
}
