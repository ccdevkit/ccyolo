package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"ccyolo/internal/constants"
)

// LogRequest is the JSON request for logging to the host
type LogRequest struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func main() {
	// Read config to get host address and verbose flag
	data, err := os.ReadFile(constants.ContainerProxyConfigPath())
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

	// Check for --prefix flag for piped input
	prefix := ""
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "--prefix" {
		prefix = args[1]
		args = args[2:]
	}

	// Determine the source label (for log prefix)
	source := "entrypoint"
	if prefix != "" {
		source = prefix
	}

	// If we have arguments, log them as a single message
	if len(args) > 0 {
		message := strings.Join(args, " ")
		sendLog(config.HostAddress, source, message)
		return
	}

	// No arguments - read from stdin (piped input)
	// Check if stdin has data (is a pipe, not a terminal)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		// stdin is a terminal, not a pipe - nothing to do
		os.Exit(0)
	}

	// Read and forward each line from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			sendLog(config.HostAddress, source, line)
		}
	}
}

func sendLog(hostAddress, source, message string) {
	conn, err := net.DialTimeout("tcp", hostAddress, 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()

	req := LogRequest{
		Type:    "log",
		Message: fmt.Sprintf("[%s] %s", source, message),
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return
	}

	conn.Write(append(reqBytes, '\n'))
}
