package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"ccyolo/internal/protocol"
)

// ProxyToHost connects to the host server and executes a command
// Returns the exit code from the host
func ProxyToHost(hostAddress, command string) int {
	Debug("Connecting to host at %s", hostAddress)
	conn, err := net.Dial("tcp", hostAddress)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: failed to connect to host: %v\n", err)
		return 1
	}
	defer conn.Close()

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: failed to get working directory: %v\n", err)
		return 1
	}

	Debug("Sending exec request: %s (cwd: %s)", command, cwd)

	// Send the exec request as JSON on the first line
	req := protocol.ExecRequest{
		Type:    "exec",
		Command: command,
		Cwd:     cwd,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: failed to marshal request: %v\n", err)
		return 1
	}

	// Write the request line
	if _, err := conn.Write(append(reqBytes, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: failed to send request: %v\n", err)
		return 1
	}

	// Close the write side to signal we're done sending
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	// Read the response: first line is exit code, rest is output
	reader := bufio.NewReader(conn)

	// Read exit code line
	exitCodeLine, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: failed to read exit code: %v\n", err)
		return 1
	}

	var exitCode int
	if _, err := fmt.Sscanf(exitCodeLine, "%d", &exitCode); err != nil {
		fmt.Fprintf(os.Stderr, "ccproxy: invalid exit code: %v\n", err)
		return 1
	}

	Debug("Command completed with exit code %d", exitCode)

	// Print note about host execution, then stream the rest of the output
	fmt.Fprintln(os.Stdout, "[NOTE: This command was run on the host machine]")
	io.Copy(os.Stdout, reader)

	return exitCode
}
