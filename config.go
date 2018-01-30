package main

import (
	"encoding/json"
	"io/ioutil"
)

const ConfigPath = "./tiny.json"

var Configuration Config

type Config struct {
	Target      string `json:"target"`
	CacheFolder string `json:"cache_folder"`
}

func LoadConfig() {
	Debug.Println("Try to load config")

	file, err := ioutil.ReadFile(ConfigPath)

	if err != nil {
		Error.Fatalln(err.Error())
	}

	json.Unmarshal(file, &Configuration)
	Debug.Println("Loading config succeeded")
}
