// This file tests structured logger level filtering.
package logger

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewFiltersLogEntriesByLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  []string
	}{
		{name: "debug", level: "debug", want: []string{"debug message", "info message", "warn message", "error message"}},
		{name: "info", level: "info", want: []string{"info message", "warn message", "error message"}},
		{name: "warn", level: "warn", want: []string{"warn message", "error message"}},
		{name: "error", level: "error", want: []string{"error message"}},
		{name: "unknown defaults to info", level: "trace", want: []string{"info message", "warn message", "error message"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loggedMessages(t, tt.level)
			if len(got) != len(tt.want) {
				t.Fatalf("messages = %#v, want %#v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("messages = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func loggedMessages(t *testing.T, level string) []string {
	t.Helper()

	var output bytes.Buffer
	log := New(&output, level)
	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil
	}

	messages := make([]string, 0, len(lines))
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("decode log entry %q: %v", string(line), err)
		}
		message, ok := entry["msg"].(string)
		if !ok {
			t.Fatalf("log entry missing string msg: %#v", entry)
		}
		messages = append(messages, message)
	}
	return messages
}
