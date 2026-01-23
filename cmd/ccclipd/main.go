package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"

	"ccyolo/internal/constants"
)

const (
	responseOK    = byte(0x00)
	responseError = byte(0x01)
)

func main() {
	port := os.Getenv(constants.EnvClipboardPort)
	if port == "" {
		port = constants.DefaultClipboardPort
	}

	// Ensure DISPLAY is set for xclip
	display := os.Getenv(constants.EnvDisplay)
	if display == "" {
		os.Setenv(constants.EnvDisplay, constants.DefaultXDisplay)
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccclipd: failed to listen on port %s: %v\n", port, err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Printf("ccclipd: listening on port %s\n", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ccclipd: accept error: %v\n", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read 4-byte length prefix (big-endian)
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBuf); err != nil {
		fmt.Fprintf(os.Stderr, "ccclipd: failed to read length: %v\n", err)
		conn.Write([]byte{responseError})
		return
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	if length == 0 || length > 50*1024*1024 { // Max 50MB
		fmt.Fprintf(os.Stderr, "ccclipd: invalid length: %d\n", length)
		conn.Write([]byte{responseError})
		return
	}

	// Read PNG data
	pngData := make([]byte, length)
	if _, err := io.ReadFull(conn, pngData); err != nil {
		fmt.Fprintf(os.Stderr, "ccclipd: failed to read PNG data: %v\n", err)
		conn.Write([]byte{responseError})
		return
	}

	// Set clipboard using xclip
	if err := setClipboard(pngData); err != nil {
		fmt.Fprintf(os.Stderr, "ccclipd: failed to set clipboard: %v\n", err)
		conn.Write([]byte{responseError})
		return
	}

	fmt.Printf("ccclipd: set clipboard image (%d bytes)\n", length)
	conn.Write([]byte{responseOK})
}

func setClipboard(pngData []byte) error {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-i")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start xclip: %w", err)
	}

	if _, err := stdin.Write(pngData); err != nil {
		stdin.Close()
		cmd.Wait()
		return fmt.Errorf("failed to write to xclip: %w", err)
	}
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("xclip failed: %w", err)
	}

	return nil
}
