// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestLogger implements Logger for testing
type TestLogger struct {
	t      *testing.T
	debugs []string
	errors []string
}

func (l *TestLogger) Debug(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.debugs = append(l.debugs, formatted)
	l.t.Logf("DEBUG: %s", formatted)
}

func (l *TestLogger) Error(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.errors = append(l.errors, formatted)
	l.t.Logf("ERROR: %s", formatted)
}

func (l *TestLogger) Exception(err error, msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.t.Logf("EXCEPTION: %s - %v", formatted, err)
}

// MockQEMUServer simulates a QEMU QMP server for testing
type MockQEMUServer struct {
	listener  net.Listener
	conn      net.Conn
	mu        sync.Mutex
	responses []string
	commands  []string
	closed    bool
}

// NewMockQEMUServer creates a new mock QEMU server
func NewMockQEMUServer(t *testing.T) (*MockQEMUServer, string, error) {
	// Create temporary directory for socket
	tmpDir, err := os.MkdirTemp("", "qmp-test-*")
	if err != nil {
		return nil, "", err
	}

	socketPath := filepath.Join(tmpDir, "qmp.sock")

	// Remove existing socket if it exists
	os.Remove(socketPath)

	// Create Unix domain socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, "", err
	}

	server := &MockQEMUServer{
		listener:  listener,
		responses: make([]string, 0),
		commands:  make([]string, 0),
	}

	// Start accepting connections in background
	go server.acceptConnections(t)

	return server, socketPath, nil
}

func (s *MockQEMUServer) acceptConnections(t *testing.T) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.closed {
				t.Errorf("Failed to accept connection: %v", err)
			}
			return
		}

		s.mu.Lock()
		s.conn = conn
		s.mu.Unlock()

		// Send QMP greeting
		greeting := `{"QMP":{"version":{"qemu":{"micro":0,"minor":8,"major":6}},"capabilities":["oob"]}}` + "\n"
		conn.Write([]byte(greeting))

		// Handle QMP protocol in a separate goroutine
		go s.handleQMPProtocol(t, conn)
	}
}

func (s *MockQEMUServer) handleQMPProtocol(t *testing.T, conn net.Conn) {
	defer conn.Close()

	// Read commands and send responses
	for {
		buffer := make([]byte, 1024)
		n, err := conn.Read(buffer)
		if err != nil {
			break
		}

		command := string(buffer[:n])
		s.mu.Lock()
		s.commands = append(s.commands, strings.TrimSpace(command))
		s.mu.Unlock()

		// Parse command
		var cmd map[string]interface{}
		if err := json.Unmarshal([]byte(command), &cmd); err != nil {
			t.Errorf("Failed to parse command: %v", err)
			continue
		}

		// Generate response based on command
		response := s.generateResponse(cmd)
		conn.Write([]byte(response + "\n"))
	}
}

func (s *MockQEMUServer) generateResponse(cmd map[string]interface{}) string {
	execute, ok := cmd["execute"].(string)
	if !ok {
		return `{"error":{"class":"GenericError","desc":"Invalid command format"}}`
	}

	switch execute {
	case "qmp_capabilities":
		return `{"return":{}}`
	case "query-commands":
		return `{"return":[{"name":"query-commands","ret-type":"CommandInfoList"},{"name":"query-status","ret-type":"StatusInfo"}]}`
	case "query-status":
		return `{"return":{"running":true,"singlestep":false,"status":"running"}}`
	case "system_powerdown":
		return `{"return":{}}`
	case "quit":
		// Simulate VM shutdown by closing connection
		s.mu.Lock()
		if s.conn != nil {
			s.conn.Close()
		}
		s.mu.Unlock()
		return `{"return":{}}`
	default:
		return `{"error":{"class":"CommandNotFound","desc":"Command not found"}}`
	}
}

func (s *MockQEMUServer) Close() {
	s.mu.Lock()
	s.closed = true
	if s.conn != nil {
		s.conn.Close()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()
}

func (s *MockQEMUServer) GetCommands() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	commands := make([]string, len(s.commands))
	copy(commands, s.commands)
	return commands
}

// TestQMPClientConnection tests basic connection functionality
func TestQMPClientConnection(t *testing.T) {
	server, socketPath, err := NewMockQEMUServer(t)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Close()
	defer os.RemoveAll(filepath.Dir(socketPath))

	logger := &TestLogger{t: t}
	client := NewQMPClientWithLogger(socketPath, logger)

	// Test connection
	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !client.Connected() {
		t.Error("Client should be connected")
	}

	// Test close
	err = client.Close()
	if err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}

	if client.Connected() {
		t.Error("Client should not be connected after close")
	}
}

// TestQMPClientCommands tests command sending functionality
func TestQMPClientCommands(t *testing.T) {
	server, socketPath, err := NewMockQEMUServer(t)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Close()
	defer os.RemoveAll(filepath.Dir(socketPath))

	logger := &TestLogger{t: t}
	client := NewQMPClientWithLogger(socketPath, logger)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Test query commands
	commands, err := client.QueryCommands(ctx)
	if err != nil {
		t.Fatalf("Failed to query commands: %v", err)
	}

	if len(commands) == 0 {
		t.Error("Expected commands to be returned")
	}

	// Test check status
	status, err := client.CheckStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to check status: %v", err)
	}

	if status == nil {
		t.Error("Expected status to be returned")
	}

	// Test is running
	running := client.IsRunning(ctx)
	if !running {
		t.Error("Expected VM to be running")
	}
}

// TestQMPClientShutdown tests shutdown functionality
func TestQMPClientShutdown(t *testing.T) {
	server, socketPath, err := NewMockQEMUServer(t)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Close()
	defer os.RemoveAll(filepath.Dir(socketPath))

	logger := &TestLogger{t: t}
	client := NewQMPClientWithLogger(socketPath, logger)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Test shutdown
	success, err := client.Shutdown(ctx, 100*time.Millisecond, 1*time.Second, true)
	if err != nil {
		t.Fatalf("Failed to shutdown: %v", err)
	}

	if !success {
		t.Error("Expected shutdown to succeed")
	}

	// Verify commands were sent
	commands := server.GetCommands()
	expectedCommands := []string{"qmp_capabilities", "system_powerdown", "quit"}

	if len(commands) < len(expectedCommands) {
		t.Errorf("Expected at least %d commands, got %d", len(expectedCommands), len(commands))
	}
}

// TestQMPClientErrors tests error handling
func TestQMPClientErrors(t *testing.T) {
	// Test connection to non-existent socket
	client := NewQMPClient("/non/existent/socket")

	ctx := context.Background()
	err := client.Connect(ctx)
	if err == nil {
		t.Error("Expected error when connecting to non-existent socket")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestQMPClientContextCancellation tests context cancellation
func TestQMPClientContextCancellation(t *testing.T) {
	server, socketPath, err := NewMockQEMUServer(t)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Close()
	defer os.RemoveAll(filepath.Dir(socketPath))

	logger := &TestLogger{t: t}
	client := NewQMPClientWithLogger(socketPath, logger)

	ctx, cancel := context.WithCancel(context.Background())
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Cancel context and test that operations fail
	cancel()

	_, err = client.SendCommand(ctx, map[string]interface{}{
		"execute": "query-status",
	})
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}
}

// TestQMPClientReconnection tests reconnection behavior
func TestQMPClientReconnection(t *testing.T) {
	server, socketPath, err := NewMockQEMUServer(t)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Close()
	defer os.RemoveAll(filepath.Dir(socketPath))

	logger := &TestLogger{t: t}
	client := NewQMPClientWithLogger(socketPath, logger)

	ctx := context.Background()

	// Connect first time
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Close connection
	client.Close()

	// Connect again
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to reconnect: %v", err)
	}

	if !client.Connected() {
		t.Error("Client should be connected after reconnection")
	}

	client.Close()
}

// TestQMPClientEvents tests event handling
func TestQMPClientEvents(t *testing.T) {
	// This test would require a more sophisticated mock server that can send events
	// For now, we'll test the event buffer functionality
	client := NewQMPClient("/tmp/test")

	// Test that events buffer starts empty
	events := client.GetEvents()
	if len(events) != 0 {
		t.Error("Expected empty events buffer")
	}
}

// TestQMPClientJSONFormatting tests JSON formatting utility
func TestQMPClientJSONFormatting(t *testing.T) {
	testData := map[string]interface{}{
		"test":   "value",
		"number": 42,
	}

	formatted := formatJSON(testData)
	if !strings.Contains(formatted, `"test": "value"`) {
		t.Error("Expected formatted JSON to contain test value")
	}

	if !strings.Contains(formatted, `"number": 42`) {
		t.Error("Expected formatted JSON to contain number value")
	}
}

// TestQMPClientConcurrency tests concurrent access
func TestQMPClientConcurrency(t *testing.T) {
	server, socketPath, err := NewMockQEMUServer(t)
	if err != nil {
		t.Fatalf("Failed to create mock server: %v", err)
	}
	defer server.Close()
	defer os.RemoveAll(filepath.Dir(socketPath))

	logger := &TestLogger{t: t}
	client := NewQMPClientWithLogger(socketPath, logger)

	ctx := context.Background()
	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Test concurrent command sending
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.SendCommand(ctx, map[string]interface{}{
				"execute": "query-status",
			})
			if err != nil {
				t.Errorf("Failed to send command: %v", err)
			}
		}()
	}

	wg.Wait()
}
