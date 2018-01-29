package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	LoadConfig()

	http.HandleFunc("/", handleGet)

	Info.Println("Ready to serve")
	http.ListenAndServe(":8080", nil)
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	fullUrl := r.URL.Path + "?" + r.URL.RawQuery

	Info.Printf("Requested '%s'\n", fullUrl)

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

	w.Write(body)
}

func handleError(err error, w http.ResponseWriter) {
	Error.Println(err.Error())
	w.WriteHeader(500)
	fmt.Fprintf(w, err.Error())
}
