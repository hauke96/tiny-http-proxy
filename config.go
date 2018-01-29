package main

import (
	"encoding/json"
	"io/ioutil"
)

const ConfigPath = "./tiny.json"

var Configuration Config

type Config struct {
	Target string `json:"target"`
}

func LoadConfig() {
	file, err := ioutil.ReadFile(ConfigPath)

	if err != nil {
		Error.Fatalln(err.Error())
	}

	json.Unmarshal(file, &Configuration)
}
