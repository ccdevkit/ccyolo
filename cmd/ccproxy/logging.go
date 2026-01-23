package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"ccyolo/internal/protocol"
)

// Logger handles debug logging for ccproxy
type Logger struct {
	hostAddress string
	verbose     bool
	mu          sync.Mutex
}

var globalLogger *Logger

// InitLogger initializes the global logger with the given config
func InitLogger(hostAddress string, verbose bool) {
	globalLogger = &Logger{
		hostAddress: hostAddress,
		verbose:     verbose,
	}
}

// Debug logs a debug message if verbose mode is enabled
func Debug(format string, args ...any) {
	if globalLogger == nil || !globalLogger.verbose {
		return
	}

	message := fmt.Sprintf(format, args...)
	globalLogger.sendLog(message)
}

func (l *Logger) sendLog(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Connect to host
	conn, err := net.DialTimeout("tcp", l.hostAddress, 2*time.Second)
	if err != nil {
		// Can't log if we can't connect - fail silently
		return
	}
	defer conn.Close()

	// Send log request
	req := protocol.LogRequest{
		Type:    "log",
		Message: message,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return
	}

	conn.Write(append(reqBytes, '\n'))
}
