// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// QMPResponse represents a response from QMP
type QMPResponse struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *QMPError       `json:"error,omitempty"`
	Event  *QMPEvent       `json:"event,omitempty"`
}

// QMPError represents an error response from QMP
type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// QMPEvent represents an event from QMP
type QMPEvent struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Time  *QMPTimestamp          `json:"timestamp,omitempty"`
}

// QMPTimestamp represents a timestamp in QMP events
type QMPTimestamp struct {
	Seconds      int64 `json:"seconds"`
	Microseconds int64 `json:"microseconds"`
}

// QMPClient represents a QMP client connection
type QMPClient struct {
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	mu         sync.Mutex
	events     []QMPEvent
	eventsMu   sync.RWMutex
	logger     Logger
}

// Logger interface for dependency injection and testing
type Logger interface {
	Debug(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Exception(err error, msg string, args ...interface{})
}

// DefaultLogger implements Logger with no-op operations
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, args ...interface{})                {}
func (l *DefaultLogger) Error(msg string, args ...interface{})                {}
func (l *DefaultLogger) Exception(err error, msg string, args ...interface{}) {}

// NewQMPClient creates a new QMP client
func NewQMPClient(socketPath string) *QMPClient {
	return &QMPClient{
		socketPath: socketPath,
		logger:     &DefaultLogger{},
	}
}

// NewQMPClientWithLogger creates a new QMP client with a custom logger
func NewQMPClientWithLogger(socketPath string, logger Logger) *QMPClient {
	return &QMPClient{
		socketPath: socketPath,
		logger:     logger,
	}
}

// Connected returns true if the client is connected
func (q *QMPClient) Connected() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.conn != nil
}

// Connect establishes a connection to the QMP socket
func (q *QMPClient) Connect(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.conn != nil {
		return nil
	}

	// Check if socket file exists
	if _, err := os.Stat(q.socketPath); os.IsNotExist(err) {
		return fmt.Errorf("QMP socket at %s not found, is QEMU running?", q.socketPath)
	}

	// Connect to Unix socket
	conn, err := net.Dial("unix", q.socketPath)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("you lack permissions to talk over socket %s", q.socketPath)
		}
		return fmt.Errorf("failed to connect to QMP socket: %w", err)
	}

	q.conn = conn
	q.reader = bufio.NewReader(conn)
	q.writer = bufio.NewWriter(conn)

	// Read QMP greeting
	if err := q.readGreeting(); err != nil {
		q.closeConnection()
		return fmt.Errorf("failed to read QMP greeting: %w", err)
	}

	// Send qmp_capabilities command
	_, err = q.sendCommandInternal(ctx, map[string]interface{}{
		"execute": "qmp_capabilities",
	})
	if err != nil {
		q.closeConnection()
		return fmt.Errorf("failed to send qmp_capabilities: %w", err)
	}

	return nil
}

// Close closes the QMP connection
func (q *QMPClient) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closeConnection()
}

func (q *QMPClient) closeConnection() error {
	if q.conn == nil {
		return nil
	}

	err := q.conn.Close()
	q.conn = nil
	q.reader = nil
	q.writer = nil
	return err
}

// readGreeting reads the initial QMP greeting
func (q *QMPClient) readGreeting() error {
	line, err := q.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read greeting: %w", err)
	}

	// Parse greeting (we don't need to validate it, just consume it)
	var greeting map[string]interface{}
	if err := json.Unmarshal([]byte(line), &greeting); err != nil {
		return fmt.Errorf("failed to parse greeting: %w", err)
	}

	q.logger.Debug("QMP greeting received: %s", strings.TrimSpace(line))
	return nil
}

// getResponse reads a response from the QMP server
func (q *QMPClient) getResponse(ctx context.Context) (*QMPResponse, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line, err := q.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("connection closed by server")
			}
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		var response QMPResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			q.logger.Exception(err, "QMP ERR> error reading response")
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Handle events
		if response.Event != nil {
			q.logger.Debug("QMP EVENT:\n%s", formatJSON(response))
			q.eventsMu.Lock()
			q.events = append(q.events, *response.Event)
			q.eventsMu.Unlock()
			continue
		}

		// Handle return or error
		if response.Return != nil || response.Error != nil {
			return &response, nil
		}

		// Unknown message type
		q.logger.Error("got a QMP message from server which I do not understand:\n%s", formatJSON(response))
		return nil, fmt.Errorf("unknown QMP message type")
	}
}

// sendCommandInternal sends a command and returns the response
func (q *QMPClient) sendCommandInternal(ctx context.Context, cmd map[string]interface{}) (*QMPResponse, error) {
	if q.conn == nil || q.reader == nil || q.writer == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Encode and send command
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		q.logger.Exception(err, "error encoding QMP message")
		return nil, fmt.Errorf("failed to encode command: %w", err)
	}

	cmdBytes = append(cmdBytes, '\n')
	if _, err := q.writer.Write(cmdBytes); err != nil {
		return nil, fmt.Errorf("failed to write command: %w", err)
	}

	if err := q.writer.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush command: %w", err)
	}

	q.logger.Debug("QMP CMD ->\n%s", formatJSON(cmd))

	// Read response
	response, err := q.getResponse(ctx)
	if err != nil {
		return nil, err
	}

	q.logger.Debug("<- QMP RSP:\n%s", formatJSON(response))
	return response, nil
}

// SendCommand sends a command to QMP and returns the response
func (q *QMPClient) SendCommand(ctx context.Context, cmd map[string]interface{}) (*QMPResponse, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sendCommandInternal(ctx, cmd)
}

// QueryCommands queries available QMP commands
func (q *QMPClient) QueryCommands(ctx context.Context) ([]map[string]interface{}, error) {
	response, err := q.SendCommand(ctx, map[string]interface{}{
		"execute": "query-commands",
	})
	if err != nil {
		return nil, fmt.Errorf("failed query-commands: %w", err)
	}

	if response.Error != nil {
		q.logger.Error("error while sending QMP command 'query-commands':\n%s", formatJSON(response))
		return nil, fmt.Errorf("error while sending QMP command 'query-commands': %s", response.Error.Desc)
	}

	var commands []map[string]interface{}
	if err := json.Unmarshal(response.Return, &commands); err != nil {
		return nil, fmt.Errorf("failed to parse commands response: %w", err)
	}

	return commands, nil
}

// CheckStatus checks if the VM is responsive by querying its status
func (q *QMPClient) CheckStatus(ctx context.Context) (map[string]interface{}, error) {
	response, err := q.SendCommand(ctx, map[string]interface{}{
		"execute": "query-status",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query VM status: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("error querying VM status: %s", response.Error.Desc)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(response.Return, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return status, nil
}

// IsRunning checks if the VM is running and responsive
func (q *QMPClient) IsRunning(ctx context.Context) bool {
	status, err := q.CheckStatus(ctx)
	if err != nil {
		return false
	}

	if running, ok := status["running"].(bool); ok {
		return running
	}
	return false
}

// shutdown attempts to shut down the VM
func (q *QMPClient) shutdown(ctx context.Context, checkInterval time.Duration, timeout time.Duration, force bool) (bool, error) {
	deadline := time.Now().Add(timeout)
	forceCmd := "quit"
	if !force {
		forceCmd = "system_powerdown"
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		_, err := q.SendCommand(ctx, map[string]interface{}{
			"execute": forceCmd,
		})

		if err != nil {
			// Check if connection is broken (VM has shut down)
			if strings.Contains(err.Error(), "connection closed") ||
				strings.Contains(err.Error(), "broken pipe") ||
				strings.Contains(err.Error(), "connection reset") {
				return true, nil
			}
		}

		// Wait before next attempt
		select {
		case <-time.After(checkInterval):
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}

	return false, nil
}

// Shutdown attempts to shut down the VM gracefully, with fallback to force quit
func (q *QMPClient) Shutdown(ctx context.Context, checkInterval time.Duration, timeout time.Duration, forceAfterTimeout bool) (bool, error) {
	// Try graceful shutdown first
	success, err := q.shutdown(ctx, checkInterval, timeout, false)
	if err != nil {
		return false, err
	}

	if success {
		return true, nil
	}

	// If graceful shutdown failed and force is enabled, try force shutdown
	if forceAfterTimeout {
		return q.shutdown(ctx, checkInterval, 5*time.Second, true)
	}

	return false, nil
}

// GetEvents returns all collected events and clears the buffer
func (q *QMPClient) GetEvents() []QMPEvent {
	q.eventsMu.Lock()
	defer q.eventsMu.Unlock()

	events := make([]QMPEvent, len(q.events))
	copy(events, q.events)
	q.events = q.events[:0]
	return events
}

// formatJSON formats a JSON object for logging
func formatJSON(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("error formatting JSON: %v", err)
	}
	return string(data)
}
