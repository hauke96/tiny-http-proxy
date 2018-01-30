package main

import (
	"encoding/json"
	"io/ioutil"
)

const ConfigPath = "./tiny.json"

type Config struct {
	Target      string `json:"target"`
	CacheFolder string `json:"cache_folder"`
}

func LoadConfig() (*Config, error) {
	file, err := ioutil.ReadFile(ConfigPath)

	if err != nil {
		return nil, err
	}

	var config Config
	json.Unmarshal(file, &config)

	return &config, nil
}
