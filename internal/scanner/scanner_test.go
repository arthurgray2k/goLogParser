package scanner_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthurgray2k/goLogParser/internal/scanner"
)

func TestScan(t *testing.T) {
	input := "line1\nline2\r\nline3\n"
	r := strings.NewReader(input)
	ctx := context.Background()

	ch := scanner.Scan(ctx, r, "testfile")

	var lines []scanner.Line
	for l := range ch {
		lines = append(lines, l)
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	if lines[0].Text != "line1" || lines[0].Number != 1 || lines[0].Source != "testfile" {
		t.Errorf("line 1 mismatch: %+v", lines[0])
	}
	if lines[1].Text != "line2" || lines[1].Number != 2 {
		t.Errorf("line 2 mismatch: %+v", lines[1])
	}
	if lines[2].Text != "line3" || lines[2].Number != 3 {
		t.Errorf("line 3 mismatch: %+v", lines[2])
	}
}

func TestScanCancellation(t *testing.T) {
	input := "line1\nline2\nline3\n"
	r := strings.NewReader(input)
	ctx, cancel := context.WithCancel(context.Background())

	ch := scanner.Scan(ctx, r, "testfile")

	// Read one line and cancel
	l, ok := <-ch
	if !ok {
		t.Fatal("expected first line")
	}
	if l.Text != "line1" {
		t.Errorf("expected line1, got %s", l.Text)
	}

	cancel()

	// Drain remaining. We just verify that the channel is closed and we do not hang.
	for range ch {
	}
}
