package hostexec

import (
	"bytes"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
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

	// Read all stdin data from the connection
	stdinData, err := io.ReadAll(conn)
	if err != nil {
		s.debug("Read error: %v", err)
		return
	}

	s.debug("Received %d bytes of stdin data", len(stdinData))

	// Execute the statusline command with the received stdin
	s.handleStatusline(conn, stdinData)
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

	cmd := exec.Command("sh", "-c", settings.StatusLine.Command)
	cmd.Dir = s.cwd
	cmd.Stdin = bytes.NewReader(stdinData)

	output, err := cmd.Output()
	if err != nil {
		s.debug("Statusline command failed: %v", err)
		return
	}

	s.debug("Statusline output: %s", strings.TrimSpace(string(output)))
	conn.Write(output)
}
