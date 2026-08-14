package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewLevelParsing(t *testing.T) {
	cases := []struct {
		in    string
		level slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"garbage", slog.LevelInfo},
	}

	for _, c := range cases {
		logger := New(c.in)
		if !logger.Handler().Enabled(context.Background(), c.level) {
			t.Errorf("New(%q): expected level %v enabled, got disabled", c.in, c.level)
		}
	}
}
