// SPDX-License-Identifier: MIT
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// Transport abstracts the communication channel to an MCP server.
type Transport interface {
	// Send sends a JSON-RPC message and returns the response bytes.
	Send(ctx context.Context, msg json.RawMessage) (json.RawMessage, error)
	// Close closes the transport.
	Close() error
}

// StdioTransport communicates with an MCP server over stdin/stdout.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Scanner
	buf    chan []byte
	cancel context.CancelFunc
}

// NewStdioTransport starts an MCP server process and returns a transport.
func NewStdioTransport(ctx context.Context, command string, args ...string) (*StdioTransport, error) {
	ctx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("mcp stdio stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		return nil, fmt.Errorf("mcp stdio stdout pipe: %w", err)
	}
	cmd.Stderr = nil // discard stderr

	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdin.Close()
		return nil, fmt.Errorf("mcp stdio start %q: %w", command, err)
	}

	t := &StdioTransport{
		cmd:    cmd,
		stdin:  stdin,
		reader: bufio.NewScanner(stdout),
		buf:    make(chan []byte, 16),
		cancel: cancel,
	}

	go t.readLines()

	return t, nil
}

func (t *StdioTransport) readLines() {
	defer close(t.buf)
	for t.reader.Scan() {
		line := make([]byte, len(t.reader.Bytes()))
		copy(line, t.reader.Bytes())
		t.buf <- line
	}
}

// Send writes a JSON-RPC message to stdin and reads the response from stdout.
func (t *StdioTransport) Send(ctx context.Context, msg json.RawMessage) (json.RawMessage, error) {
	msg = append(msg, '\n')
	if _, err := t.stdin.Write(msg); err != nil {
		return nil, fmt.Errorf("mcp stdio write: %w", err)
	}

	select {
	case line, ok := <-t.buf:
		if !ok {
			return nil, fmt.Errorf("mcp stdio: connection closed")
		}
		return line, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close terminates the server process.
func (t *StdioTransport) Close() error {
	t.cancel()
	_ = t.stdin.Close()
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
	}
	return nil
}
