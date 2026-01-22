package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// LogRequest is the JSON request for logging to the host
type LogRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func main() {
	// Read config to get host address and verbose flag
	configPath := "/tmp/ccyolo-proxy.json"
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config = no logging, exit silently
		os.Exit(0)
	}

	var config struct {
		HostAddress string `json:"hostAddress"`
		Verbose     bool   `json:"verbose"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		os.Exit(0)
	}

	// Only log if verbose mode is enabled
	if !config.Verbose {
		os.Exit(0)
	}

	// Get message from arguments
	if len(os.Args) < 2 {
		os.Exit(0)
	}
	message := strings.Join(os.Args[1:], " ")

	// Send log to host
	conn, err := net.DialTimeout("tcp", config.HostAddress, 2*time.Second)
	if err != nil {
		os.Exit(0)
	}
	defer conn.Close()

	req := LogRequest{
		Type:    "log",
		Message: fmt.Sprintf("[entrypoint] %s", message),
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		os.Exit(0)
	}

	conn.Write(append(reqBytes, '\n'))
}
