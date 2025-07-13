# QMP Client Implementation

This directory contains a Go implementation of the QEMU Management Protocol (QMP) client for VM management operations.

## Overview

The QMP client provides a robust interface for communicating with QEMU virtual machines through their QMP socket. It supports:

- **Connection Management**: Connect to QMP sockets with proper error handling
- **Command Execution**: Send QMP commands and receive responses
- **Event Handling**: Collect and buffer QMP events
- **VM Status Queries**: Check if VMs are running and get detailed status
- **Graceful Shutdown**: Shutdown VMs with graceful fallback to force quit
- **Context Support**: Full context cancellation support for timeouts and cancellation
- **Thread Safety**: Safe for concurrent use
- **Testability**: Designed for easy testing without requiring real QEMU instances

## Key Features

### 1. Robust Error Handling
- Handles connection failures gracefully
- Distinguishes between different error types (permission, not found, etc.)
- Provides detailed error messages

### 2. Event Collection
- Automatically collects QMP events during operation
- Buffers events for later retrieval
- Thread-safe event handling

### 3. Graceful Shutdown
- Attempts graceful shutdown first (`system_powerdown`)
- Falls back to force quit (`quit`) if graceful shutdown times out
- Configurable timeouts and retry intervals

### 4. Context Support
- All operations respect context cancellation
- Supports timeouts and cancellation
- Proper cleanup on context cancellation

### 5. Testability
- Mock QEMU server for testing
- Dependency injection for logging
- Isolated unit tests without requiring real QEMU

## Usage

### Basic Usage

```go
// Create a QMP client
client := NewQMPClient("/tmp/qemu-vm.qmp")

// Connect with context
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := client.Connect(ctx); err != nil {
    log.Fatalf("Failed to connect: %v", err)
}
defer client.Close()

// Check if VM is running
if client.IsRunning(ctx) {
    fmt.Println("VM is running")
}

// Shutdown the VM
success, err := client.Shutdown(ctx, 1*time.Second, 20*time.Second, true)
if err != nil {
    log.Printf("Shutdown failed: %v", err)
}
```

### Custom Logging

```go
// Create a custom logger
customLogger := &MyLogger{}

// Create client with custom logger
client := NewQMPClientWithLogger("/tmp/qemu-vm.qmp", customLogger)
```

### Error Handling

```go
if err := client.Connect(ctx); err != nil {
    if strings.Contains(err.Error(), "not found") {
        // Handle missing socket
    } else if strings.Contains(err.Error(), "permissions") {
        // Handle permission errors
    }
}
```

## Testing Strategy

The QMP client is designed to be highly testable without requiring real QEMU instances:

### 1. Mock QEMU Server
- `MockQEMUServer` simulates a real QEMU QMP server
- Responds to QMP commands with predefined responses
- Can simulate various scenarios (shutdown, errors, etc.)

### 2. Dependency Injection
- Logger interface allows injection of test loggers
- Can capture and verify log messages during tests

### 3. Isolated Unit Tests
- Tests can run without external dependencies
- Fast execution and reliable results
- Can test error conditions easily

### 4. Integration Test Support
- Can be used with real QEMU instances for integration testing
- Same interface works with both mock and real servers

## Architecture

### Core Components

1. **QMPClient**: Main client struct with connection management
2. **QMPResponse**: Represents QMP responses (return, error, event)
3. **QMPEvent**: Represents QMP events with timestamps
4. **Logger**: Interface for dependency injection

### Thread Safety

- All public methods are thread-safe
- Internal state is protected with mutexes
- Event buffer uses read-write mutex for performance

### Connection Lifecycle

1. **Connect**: Establishes connection, reads greeting, sends capabilities
2. **Command Execution**: Sends commands and reads responses
3. **Event Handling**: Collects events during operation
4. **Close**: Cleanly closes connection

## Protocol Details

The QMP client implements the QEMU Management Protocol:

- **Line-based JSON**: Each message is a JSON object followed by newline
- **Request-Response**: Commands expect responses
- **Events**: QEMU may send events at any time
- **Greeting**: Initial handshake with QMP capabilities

### Message Types

1. **Commands**: `{"execute": "command-name", "arguments": {...}}`
2. **Responses**: `{"return": {...}}` or `{"error": {...}}`
3. **Events**: `{"event": "event-name", "data": {...}, "timestamp": {...}}`

## Future Enhancements

1. **Event Streaming**: Real-time event streaming with callbacks
2. **Connection Pooling**: Multiple connection support
3. **Retry Logic**: Automatic retry for transient failures
4. **Metrics**: Built-in metrics collection
5. **More Commands**: Additional QMP command wrappers

## Dependencies

- Standard library only (no external dependencies)
- Uses `net`, `encoding/json`, `context`, `sync`, `time`

## Performance Considerations

- Buffered I/O for efficient communication
- Minimal memory allocations
- Efficient JSON parsing
- Thread-safe without excessive locking

## Security Considerations

- Unix domain socket communication (local only)
- No authentication (relies on filesystem permissions)
- Input validation for all commands
- Safe error message handling 