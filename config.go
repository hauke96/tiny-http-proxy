package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

const ConfigPath = "./tiny.json"

var Configuration Config

type Config struct {
	Target string `json:"target"`
}

func LoadConfig() {
	file, err := ioutil.ReadFile(ConfigPath)

	if err != nil {
		Fatal.Println(err.Error())
		os.Exit(1)
	}

	json.Unmarshal(file, &Configuration)
}
