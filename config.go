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

func LoadConfig() error {
	file, err := ioutil.ReadFile(ConfigPath)

	if err != nil {
		return err
	}

	json.Unmarshal(file, &Configuration)

	return nil
}
