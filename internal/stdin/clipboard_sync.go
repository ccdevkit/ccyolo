package stdin

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"ccyolo/internal/clipboard"
)

// TCPClipboardSyncer sends clipboard images to the container via TCP
type TCPClipboardSyncer struct {
	addr  string // e.g., "host.docker.internal:9999"
	debug DebugFunc
}

// NewTCPClipboardSyncer creates a new TCP clipboard syncer
func NewTCPClipboardSyncer(addr string, debug DebugFunc) *TCPClipboardSyncer {
	if debug == nil {
		debug = func(format string, args ...any) {} // no-op
	}
	return &TCPClipboardSyncer{addr: addr, debug: debug}
}

// Sync reads the host clipboard and sends any image to the container
func (s *TCPClipboardSyncer) Sync() error {
	s.debug("ClipboardSyncer: reading clipboard image")

	// Read image from clipboard
	pngData, err := clipboard.ReadImage()
	if err != nil {
		s.debug("ClipboardSyncer: clipboard read error: %v", err)
		return fmt.Errorf("failed to read clipboard: %w", err)
	}
	if pngData == nil {
		s.debug("ClipboardSyncer: no image in clipboard")
		// No image in clipboard, nothing to sync
		return nil
	}

	s.debug("ClipboardSyncer: found image, size=%d bytes", len(pngData))

	// Connect to container daemon
	s.debug("ClipboardSyncer: connecting to daemon at %s", s.addr)
	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		s.debug("ClipboardSyncer: connection failed: %v", err)
		return fmt.Errorf("failed to connect to clipboard daemon: %w", err)
	}
	defer conn.Close()

	s.debug("ClipboardSyncer: connected, sending %d bytes", len(pngData))

	// Send length prefix (4 bytes, big-endian)
	length := uint32(len(pngData))
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, length)
	if _, err := conn.Write(lengthBuf); err != nil {
		s.debug("ClipboardSyncer: failed to send length: %v", err)
		return fmt.Errorf("failed to send length: %w", err)
	}

	// Send PNG data
	if _, err := conn.Write(pngData); err != nil {
		s.debug("ClipboardSyncer: failed to send PNG data: %v", err)
		return fmt.Errorf("failed to send PNG data: %w", err)
	}

	// Read response (1 byte)
	s.debug("ClipboardSyncer: waiting for response")
	response := make([]byte, 1)
	if _, err := io.ReadFull(conn, response); err != nil {
		s.debug("ClipboardSyncer: failed to read response: %v", err)
		return fmt.Errorf("failed to read response: %w", err)
	}

	if response[0] != 0x00 {
		s.debug("ClipboardSyncer: daemon returned error code 0x%02x", response[0])
		return fmt.Errorf("clipboard daemon returned error")
	}

	s.debug("ClipboardSyncer: sync successful")
	return nil
}

// NoOpClipboardSyncer is a no-op implementation for testing or when clipboard sync is disabled
type NoOpClipboardSyncer struct{}

// Sync does nothing
func (s *NoOpClipboardSyncer) Sync() error {
	return nil
}
