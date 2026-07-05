package tail_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/tail"
)

func TestFollow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")

	if err := os.WriteFile(path, []byte("initial line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.TailConfig{
		PollInterval: 50 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := tail.Follow(ctx, path, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := f.WriteString("{\"level\":\"INFO\",\"msg\":\"new line\"}\n"); err != nil {
		t.Fatal(err)
	}

	select {
	case line := <-ch:
		if line.Entry.Level != "INFO" {
			t.Errorf("expected INFO, got %s", line.Entry.Level)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for new line")
	}
}
