// SPDX-License-Identifier: GPL-3.0-or-later
// SPDX-FileCopyrightText: 2025 Jesper Devantier <jwd@defmacro.it>
package internal

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleQMPUsage demonstrates how to use the QMP client
func ExampleQMPUsage() {
	// Create a QMP client for a VM's socket
	socketPath := "/tmp/qemu-vm.qmp"
	client := NewQMPClient(socketPath)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to the QMP socket
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to QMP: %v", err)
	}
	defer client.Close()

	// Check if VM is running
	if client.IsRunning(ctx) {
		fmt.Println("VM is running")
	} else {
		fmt.Println("VM is not running")
	}

	// Query available commands
	commands, err := client.QueryCommands(ctx)
	if err != nil {
		log.Printf("Failed to query commands: %v", err)
	} else {
		fmt.Printf("Available commands: %d\n", len(commands))
	}

	// Get VM status
	status, err := client.CheckStatus(ctx)
	if err != nil {
		log.Printf("Failed to check status: %v", err)
	} else {
		fmt.Printf("VM status: %+v\n", status)
	}

	// Shutdown the VM gracefully
	fmt.Println("Shutting down VM...")
	success, err := client.Shutdown(ctx, 1*time.Second, 20*time.Second, true)
	if err != nil {
		log.Printf("Shutdown failed: %v", err)
	} else if success {
		fmt.Println("VM shutdown successful")
	} else {
		fmt.Println("VM shutdown failed")
	}

	// Check for any events that occurred
	events := client.GetEvents()
	if len(events) > 0 {
		fmt.Printf("Received %d events during operation\n", len(events))
		for i, event := range events {
			fmt.Printf("Event %d: %s\n", i+1, event.Event)
		}
	}
}

// ExampleQMPWithCustomLogger demonstrates using a custom logger
func ExampleQMPWithCustomLogger() {
	// Create a custom logger
	customLogger := &CustomLogger{}

	// Create QMP client with custom logger
	socketPath := "/tmp/qemu-vm.qmp"
	client := NewQMPClientWithLogger(socketPath, customLogger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the client as normal
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// The custom logger will be used for all debug/error messages
	client.IsRunning(ctx)
}

// CustomLogger implements the Logger interface
type CustomLogger struct{}

func (l *CustomLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}

func (l *CustomLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}

func (l *CustomLogger) Exception(err error, msg string, args ...interface{}) {
	fmt.Printf("[EXCEPTION] "+msg+": %v\n", append(args, err)...)
}

// ExampleQMPErrorHandling demonstrates proper error handling
func ExampleQMPErrorHandling() {
	client := NewQMPClient("/non/existent/socket")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This will fail because the socket doesn't exist
	if err := client.Connect(ctx); err != nil {
		fmt.Printf("Connection failed as expected: %v\n", err)
		// You can check for specific error types
		if err.Error() == "QMP socket at /non/existent/socket not found, is QEMU running?" {
			fmt.Println("This is the expected error for a non-existent socket")
		}
	}
}

// ExampleQMPContextCancellation demonstrates context cancellation
func ExampleQMPContextCancellation() {
	client := NewQMPClient("/tmp/qemu-vm.qmp")

	// Create a context that will be cancelled immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// This will fail due to context cancellation
	if err := client.Connect(ctx); err != nil {
		fmt.Printf("Connection cancelled as expected: %v\n", err)
	}
}
