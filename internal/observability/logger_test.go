package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLoggerRejectsInvalidLevel(t *testing.T) {
	_, err := NewLogger("verbose")
	if err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestJSONLogger(t *testing.T) {
	var output bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logger.Info("test event", "service", "trip-service")

	if !strings.Contains(output.String(), `"msg":"test event"`) {
		t.Fatalf("expected JSON log message, got %s", output.String())
	}
}
