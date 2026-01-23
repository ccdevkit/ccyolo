package hostexec

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"ccyolo/internal/constants"
	"ccyolo/internal/protocol"
)

// DebugFunc is a function for debug logging
type DebugFunc func(format string, args ...any)

// Server handles TCP connections for host-side command execution
type Server struct {
	listener net.Listener
	port     int
	homeDir  string
	cwd      string
	debug    DebugFunc
	wg       sync.WaitGroup
	done     chan struct{}
}

// Start creates and starts a TCP server on a random available port
func Start(homeDir, cwd string, debug DebugFunc) (*Server, error) {
	// Listen on localhost with a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	s := &Server{
		listener: listener,
		port:     port,
		homeDir:  homeDir,
		cwd:      cwd,
		debug:    debug,
		done:     make(chan struct{}),
	}

	s.wg.Add(1)
	go s.acceptLoop()

	debug("TCP server started on port %d", port)
	return s, nil
}

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.port
}

// Stop gracefully shuts down the server
func (s *Server) Stop() {
	close(s.done)
	s.listener.Close()
	s.wg.Wait()
	s.debug("TCP server stopped")
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				s.debug("Accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Use a buffered reader to peek at the first line
	reader := bufio.NewReader(conn)

	// Read the first line to determine the request type
	firstLine, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		s.debug("Read error: %v", err)
		return
	}

	// Try to parse as JSON request
	trimmedLine := bytes.TrimSpace(firstLine)

	// Check for exec request
	var execReq protocol.ExecRequest
	if err := json.Unmarshal(trimmedLine, &execReq); err == nil && execReq.Type == "exec" {
		s.debug("Handling exec request: %s", execReq.Command)
		s.handleExec(conn, reader, &execReq)
		return
	}

	// Check for log request
	var logReq protocol.LogRequest
	if err := json.Unmarshal(trimmedLine, &logReq); err == nil && logReq.Type == "log" {
		s.handleLog(&logReq)
		return
	}

	// Not a JSON request - treat as raw statusline data
	// Prepend the first line we already read and read the rest
	restData, _ := io.ReadAll(reader)
	stdinData := append(firstLine, restData...)
	s.debug("Received %d bytes of stdin data for statusline", len(stdinData))
	s.handleStatusline(conn, stdinData)
}

// shellCommand creates an exec.Cmd that runs the given command through the
// platform's shell (sh -c on Unix, cmd /C on Windows)
func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

func (s *Server) handleExec(conn net.Conn, reader *bufio.Reader, req *protocol.ExecRequest) {
	s.debug("[host] Executing on host: %s (cwd: %s)", req.Command, req.Cwd)

	// Execute the command on the host
	cmd := shellCommand(req.Command)

	// Use the provided cwd, or fall back to server's cwd
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	} else {
		cmd.Dir = s.cwd
	}

	// Don't pipe stdin - exec commands are non-interactive
	// If stdin were needed, CombinedOutput would block waiting for EOF

	// Capture combined output
	output, err := cmd.CombinedOutput()

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			s.debug("[host] Exec command error: %v", err)
			exitCode = 1
		}
	}

	s.debug("[host] Command finished with exit code %d, response %d bytes", exitCode, len(output))

	// Write exit code on first line, then output
	fmt.Fprintf(conn, "%d\n", exitCode)
	conn.Write(output)
}

func (s *Server) handleLog(req *protocol.LogRequest) {
	// Forward the log message to the debug function
	// The [container] prefix distinguishes container logs from host logs
	s.debug("[container] %s", req.Message)
}

// rewriteContainerPaths replaces /home/claude with the actual host home directory
// in the statusline JSON data. This is needed because paths like transcriptPath
// are generated inside the container but need to be valid on the host.
func (s *Server) rewriteContainerPaths(data []byte) []byte {
	// Parse as generic JSON to find and replace paths
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		// Not valid JSON, return as-is
		return data
	}

	// Recursively rewrite paths in the object
	s.rewritePathsInObject(obj)

	// Re-encode
	result, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return result
}

// rewritePathsInObject recursively walks a JSON object and rewrites container paths
func (s *Server) rewritePathsInObject(obj map[string]any) {
	for key, value := range obj {
		switch v := value.(type) {
		case string:
			// Check if this string starts with the container home path
			if strings.HasPrefix(v, constants.ContainerHome) {
				obj[key] = s.homeDir + strings.TrimPrefix(v, constants.ContainerHome)
			}
		case map[string]any:
			// Recurse into nested objects
			s.rewritePathsInObject(v)
		case []any:
			// Handle arrays
			for i, item := range v {
				if str, ok := item.(string); ok && strings.HasPrefix(str, constants.ContainerHome) {
					v[i] = s.homeDir + strings.TrimPrefix(str, constants.ContainerHome)
				} else if nested, ok := item.(map[string]any); ok {
					s.rewritePathsInObject(nested)
				}
			}
		}
	}
}

func (s *Server) handleStatusline(conn net.Conn, stdinData []byte) {
	settings, err := GetMergedSettings(s.homeDir, s.cwd)
	if err != nil {
		s.debug("Failed to get settings: %v", err)
		return
	}

	if settings.StatusLine == nil || settings.StatusLine.Command == "" {
		s.debug("No statusline command configured")
		return
	}

	// Rewrite container paths to host paths
	rewrittenData := s.rewriteContainerPaths(stdinData)
	s.debug("Statusline input (rewritten): %s", strings.TrimSpace(string(rewrittenData)))

	cmd := shellCommand(settings.StatusLine.Command)
	cmd.Dir = s.cwd
	cmd.Stdin = bytes.NewReader(rewrittenData)

	output, err := cmd.Output()
	if err != nil {
		s.debug("Statusline command failed: %v", err)
		return
	}

	s.debug("Statusline output: %s", strings.TrimSpace(string(output)))
	conn.Write(output)
}
