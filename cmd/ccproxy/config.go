package main

import (
	"encoding/json"
	"os"
)

const ConfigPath = "/tmp/ccyolo-proxy.json"

// Config represents the proxy configuration
type Config struct {
	HostAddress string   `json:"hostAddress"`
	Passthrough []string `json:"passthrough"`
	Verbose     bool     `json:"verbose"`
}

// LoadConfig reads and parses the config file
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
