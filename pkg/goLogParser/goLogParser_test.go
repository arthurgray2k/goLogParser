package goLogParser_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	glp "github.com/arthurgray2k/goLogParser/pkg/goLogParser"
)

func TestPublicAPIParseFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "glp-api-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "app.log")
	logContent := `{"timestamp": "2026-07-05T01:00:00Z", "level": "info", "message": "starting service", "service": "auth"}
{"timestamp": "2026-07-05T01:05:00Z", "level": "error", "message": "connection failed", "service": "db"}
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("writing temp log: %v", err)
	}

	p := glp.New()
	ctx := context.Background()

	t.Run("ParseFile auto-detect", func(t *testing.T) {
		res, err := p.ParseFile(ctx, logPath, glp.ParseConfig{Format: glp.FormatAuto}, glp.FilterConfig{})
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		if res.Stats.TotalLines != 2 {
			t.Errorf("expected 2 total lines, got %d", res.Stats.TotalLines)
		}
		if len(res.Entries) != 2 {
			t.Errorf("expected 2 parsed entries, got %d", len(res.Entries))
		}

		if res.Entries[0].Level != "INFO" || res.Entries[1].Level != "ERROR" {
			t.Errorf("levels mismatch: %s, %s", res.Entries[0].Level, res.Entries[1].Level)
		}
	})

	t.Run("ParseFile with filter", func(t *testing.T) {
		filterCfg := glp.FilterConfig{
			Levels: []glp.Level{glp.LevelError},
		}
		res, err := p.ParseFile(ctx, logPath, glp.ParseConfig{Format: glp.FormatAuto}, filterCfg)
		if err != nil {
			t.Fatalf("ParseFile failed: %v", err)
		}

		if len(res.Entries) != 1 {
			t.Errorf("expected 1 entry after filtering, got %d", len(res.Entries))
		}
		if res.Entries[0].Level != "ERROR" {
			t.Errorf("expected level ERROR, got %s", res.Entries[0].Level)
		}
	})
}

type customTestParser struct{}

func (p *customTestParser) Name() string { return "custom-test" }
func (p *customTestParser) CanParse(sample string) float64 {
	if strings.Contains(sample, "custom") {
		return 0.9
	}
	return 0
}
func (p *customTestParser) ParseLine(line string) glp.Entry {
	return glp.Entry{Raw: line, Level: "INFO", Message: "custom: " + line}
}

func TestCustomParserRegistration(t *testing.T) {
	glp.RegisterParser(&customTestParser{})

	p := glp.New()
	ctx := context.Background()
	
	logContent := "test custom line"
	r := filepath.Join(t.TempDir(), "custom.log")
	os.WriteFile(r, []byte(logContent), 0644)

	res, err := p.ParseFile(ctx, r, glp.ParseConfig{Format: "custom-test"}, glp.FilterConfig{})
	if err != nil {
		t.Fatalf("failed to parse with custom-test: %v", err)
	}

	if len(res.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(res.Entries))
	}
	if res.Entries[0].Message != "custom: test custom line" {
		t.Errorf("expected custom message prefix, got %q", res.Entries[0].Message)
	}
}

func TestFilterEntriesHelper(t *testing.T) {
	res := glp.Result{
		Entries: []glp.Entry{
			{Level: "INFO", Message: "hello info"},
			{Level: "ERROR", Message: "hello error"},
		},
	}

	filtered := glp.FilterEntries(res, glp.LevelFilter(glp.LevelError))
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered entry, got %d", len(filtered))
	}
	if filtered[0].Level != "ERROR" {
		t.Errorf("expected ERROR, got %s", filtered[0].Level)
	}
}

func TestLiveTail(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "glp-tail-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "app.log")
	if err := os.WriteFile(logPath, []byte(""), 0644); err != nil {
		t.Fatalf("creating empty log: %v", err)
	}

	p := glp.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := p.Tail(ctx, logPath, glp.TailConfig{
		ParseConfig: glp.ParseConfig{Format: glp.FormatAuto},
		PollInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Tail failed: %v", err)
	}

	// Write entry
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("opening log to append: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString("2026-07-05 01:00:00 [INFO] line check\n"); err != nil {
		t.Fatalf("writing log content: %v", err)
	}

	// Read entry
	select {
	case line, ok := <-ch:
		if !ok {
			t.Fatal("tail channel closed unexpectedly")
		}
		if line.Entry.Level != "INFO" {
			t.Errorf("expected level INFO, got %s", line.Entry.Level)
		}
		if line.Entry.Message != "line check" {
			t.Errorf("expected message 'line check', got %q", line.Entry.Message)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for tail entry")
	}
}
