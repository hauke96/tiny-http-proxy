package main

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	Target           string `json:"target"`
	CacheFolder      string `json:"cache_folder"`
	Port             string `json:"port"`
	DebugLogging     bool   `json:"debug_logging"`
	MaxCacheItemSize int64  `json:"max_cache_item_size"` // in MB
}

func LoadConfig(path string) (*Config, error) {
	file, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var config Config
	json.Unmarshal(file, &config)

	return &config, nil
}
