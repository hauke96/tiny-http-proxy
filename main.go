package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

var TARGET_HOST string = "http://hauke-steler.de"

func main() {
	http.HandleFunc("/", handleGet)
	http.ListenAndServe(":8080", nil)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%s", r.URL.Path)

	response, err := http.Get(TARGET_HOST + r.URL.Path)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		log.Fatal(err)
	}

	w.Write(body)
}
