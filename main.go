package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

var cache *Cache

func main() {
	LoadConfig()

	var err error
	cache, err = CreateCache(Configuration.CacheFolder)

	if err != nil {
		Error.Fatalf("Could not init cache: '%s'", err.Error())
	}

	http.HandleFunc("/", handleGet)

	Info.Println("Ready to serve")
	http.ListenAndServe(":8080", nil)
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
		response, err := http.Get(Configuration.Target + fullUrl)
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
