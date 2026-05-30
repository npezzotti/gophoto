package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tcases := []struct {
		name  string
		debug bool
	}{
		{
			name:  "Debug mode enabled",
			debug: true,
		},
		{
			name:  "Debug mode disabled",
			debug: false,
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			logger := NewLogger(&bytes.Buffer{}, tc.debug)
			if logger == nil {
				t.Errorf("Expected logger to be non-nil")
			}
			if logger.debug != tc.debug {
				t.Errorf("Expected debug mode to be %v, got %v", tc.debug, logger.debug)
			}
		})
	}
}

func TestLoggerLevelMethodsWriteExpectedPrefixes(t *testing.T) {
	tests := []struct {
		name        string
		logCall     func(*Logger)
		expectLevel string
		expectMsg   string
	}{
		{
			name: "Info writes info level",
			logCall: func(l *Logger) {
				l.Info("hello %s", "world")
			},
			expectLevel: "[INFO]",
			expectMsg:   "hello world",
		},
		{
			name: "Warn writes warn level",
			logCall: func(l *Logger) {
				l.Warn("disk at %d%%", 95)
			},
			expectLevel: "[WARN]",
			expectMsg:   "disk at 95%",
		},
		{
			name: "Error writes error level",
			logCall: func(l *Logger) {
				l.Error("operation failed: %s", "timeout")
			},
			expectLevel: "[ERROR]",
			expectMsg:   "operation failed: timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := NewLogger(buf, false)

			tc.logCall(logger)
			output := buf.String()

			if !strings.Contains(output, tc.expectLevel) {
				t.Fatalf("Expected output to contain level %q, got %q", tc.expectLevel, output)
			}
			if !strings.Contains(output, tc.expectMsg) {
				t.Fatalf("Expected output to contain message %q, got %q", tc.expectMsg, output)
			}
		})
	}
}

func TestLoggerDebugRespectsDebugFlag(t *testing.T) {
	t.Run("no write when debug is disabled", func(t *testing.T) {
		buf := &bytes.Buffer{}
		logger := NewLogger(buf, false)

		logger.Debug("should not appear")

		if buf.Len() != 0 {
			t.Fatalf("Expected no output when debug is disabled, got %q", buf.String())
		}
	})

	t.Run("debug enabled writes debug level", func(t *testing.T) {
		buf := &bytes.Buffer{}
		logger := NewLogger(buf, true)

		logger.Debug("test debug log %d", 1)
		output := buf.String()

		if !strings.Contains(output, "[DEBUG]") {
			t.Fatalf("Expected output to contain debug level, got %q", output)
		}
		if !strings.Contains(output, "test debug log 1") {
			t.Fatalf("Expected output to contain debug message, got %q", output)
		}
	})
}

func TestLoggerMultipleWritesAppend(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger(buf, false)

	logger.Info("first")
	logger.Warn("second")

	output := buf.String()
	firstIdx := strings.Index(output, "first")
	secondIdx := strings.Index(output, "second")

	if firstIdx == -1 {
		t.Fatalf("Expected first message in output, got %q", output)
	}
	if secondIdx == -1 {
		t.Fatalf("Expected second message in output, got %q", output)
	}
	if firstIdx > secondIdx {
		t.Fatalf("Expected messages in write order, got %q", output)
	}
}
