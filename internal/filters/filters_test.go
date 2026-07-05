package filters_test

import (
	"testing"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/filters"
	"github.com/arthurgray2k/goLogParser/internal/formats"
)

func TestFilters(t *testing.T) {
	e := formats.Entry{
		Level:     "ERROR",
		Message:   "connection timeout occurred",
		Service:   "payment-api",
		Host:      "host-a",
		Timestamp: time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC),
		Fields:    map[string]any{"code": 500, "user": "alice"},
	}

	t.Run("Level", func(t *testing.T) {
		f := filters.Level(config.LevelError)
		if !f.Match(e) {
			t.Error("expected match for level ERROR")
		}

		f2 := filters.Level(config.LevelInfo)
		if f2.Match(e) {
			t.Error("expected no match for level INFO")
		}
	})

	t.Run("Contains", func(t *testing.T) {
		f := filters.Contains("timeout", "connection")
		if !f.Match(e) {
			t.Error("expected match for connection timeout")
		}

		f2 := filters.Contains("success")
		if f2.Match(e) {
			t.Error("expected no match for success")
		}
	})

	t.Run("NotContains", func(t *testing.T) {
		f := filters.NotContains("success", "completed")
		if !f.Match(e) {
			t.Error("expected match since message doesn't contain success or completed")
		}

		f2 := filters.NotContains("timeout")
		if f2.Match(e) {
			t.Error("expected no match since message does contain timeout")
		}
	})

	t.Run("StartsWith / EndsWith", func(t *testing.T) {
		f1 := filters.StartsWith("connection")
		f2 := filters.EndsWith("occurred")
		if !f1.Match(e) || !f2.Match(e) {
			t.Error("expected starts/ends with matches")
		}
	})

	t.Run("Regex", func(t *testing.T) {
		f, _ := filters.Regex(`conn.*timeout`)
		if !f.Match(e) {
			t.Error("expected regex match")
		}
	})

	t.Run("Metadata", func(t *testing.T) {
		f1 := filters.Services("payment-api")
		f2 := filters.Hosts("host-a")
		if !f1.Match(e) || !f2.Match(e) {
			t.Error("expected metadata matches")
		}
	})

	t.Run("Field", func(t *testing.T) {
		f1 := filters.Field("code", "500")
		f2 := filters.Field("user", "alice")
		if !f1.Match(e) || !f2.Match(e) {
			t.Error("expected field matches")
		}
	})

	t.Run("FromConfig", func(t *testing.T) {
		after := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
		before := time.Date(2026, 7, 5, 2, 0, 0, 0, time.UTC)

		cfg := config.FilterConfig{
			Levels:   []config.Level{config.LevelError},
			Contains: []string{"timeout"},
			Services: []string{"payment-api"},
			After:    &after,
			Before:   &before,
			Fields:   map[string]string{"user": "alice"},
		}

		f, err := filters.FromConfig(cfg)
		if err != nil {
			t.Fatalf("FromConfig failed: %v", err)
		}

		if !f.Match(e) {
			t.Error("expected entry to match compiled config filter")
		}
	})
}
