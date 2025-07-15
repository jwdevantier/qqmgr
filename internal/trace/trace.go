package trace

import (
	"log/slog"
	"os"
	"path/filepath"
)

// Tracer interface for trace logging
type Tracer interface {
	Trace(category, msg string, args ...any)
	EnabledForCategory(category string) bool
	GetPatterns() []string
	AddPattern(pattern string)
	SetPatterns(patterns []string)
	Close() error
}

// TraceLogger is a concrete implementation of Tracer
type TraceLogger struct {
	*slog.Logger
	patterns []string
	file     *os.File // Keep reference to close later
}

// NewTraceLogger creates a new trace logger that writes to stderr
func NewTraceLogger(patterns []string) Tracer {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &TraceLogger{
		Logger:   logger,
		patterns: patterns,
		file:     nil,
	}
}

// NewTraceLoggerWithFile creates a new trace logger that writes to a file
// The file is truncated if it exists, created if it doesn't
func NewTraceLoggerWithFile(patterns []string, filePath string) (Tracer, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Open file, truncating if it exists
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &TraceLogger{
		Logger:   logger,
		patterns: patterns,
		file:     file,
	}, nil
}

// Close closes the underlying file if one was opened
func (t *TraceLogger) Close() error {
	if t.file != nil {
		err := t.file.Close()
		t.file = nil // Prevent double-close
		return err
	}
	return nil
}

func (t *TraceLogger) Trace(category, msg string, args ...any) {
	if t.matchesPattern(category) {
		t.Logger.Debug(msg, append([]any{"trace", category}, args...)...)
	}
}

// EnabledForCategory checks if tracing is enabled for a specific category
func (t *TraceLogger) EnabledForCategory(category string) bool {
	return t.matchesPattern(category)
}

// GetPatterns returns the currently enabled trace patterns
func (t *TraceLogger) GetPatterns() []string {
	return append([]string{}, t.patterns...) // Return a copy
}

// AddPattern adds a new trace pattern
func (t *TraceLogger) AddPattern(pattern string) {
	t.patterns = append(t.patterns, pattern)
}

// SetPatterns replaces all trace patterns
func (t *TraceLogger) SetPatterns(patterns []string) {
	t.patterns = append([]string{}, patterns...) // Make a copy
}

// NoOpTracer is a no-operation tracer that does nothing
type NoOpTracer struct{}

func NewNoOpTracer() Tracer {
	return &NoOpTracer{}
}

func (n *NoOpTracer) Trace(category, msg string, args ...any) {
	// Do nothing
}

func (n *NoOpTracer) EnabledForCategory(category string) bool {
	return false
}

func (n *NoOpTracer) GetPatterns() []string {
	return []string{}
}

func (n *NoOpTracer) AddPattern(pattern string) {
	// Do nothing
}

func (n *NoOpTracer) SetPatterns(patterns []string) {
	// Do nothing
}

func (n *NoOpTracer) Close() error {
	return nil
}

func (t *TraceLogger) matchesPattern(category string) bool {
	if len(t.patterns) == 0 {
		return false
	}

	for _, pattern := range t.patterns {
		if matched, _ := filepath.Match(pattern, category); matched {
			return true
		}
	}

	return false
}
