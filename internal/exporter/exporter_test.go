package exporter_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/exporter"
	"github.com/arthurgray2k/goLogParser/internal/formats"
	"github.com/arthurgray2k/goLogParser/internal/statistics"
)

func TestJSONExporter(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := config.ExportConfig{
		Format: config.ExportJSON,
		Pretty: false,
	}

	exp := exporter.New(buf, cfg)
	
	e1 := formats.Entry{
		Level:     "INFO",
		Message:   "message one",
		Timestamp: time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC),
	}
	e2 := formats.Entry{
		Level:     "ERROR",
		Message:   "message two",
		Timestamp: time.Date(2026, 7, 5, 2, 0, 0, 0, time.UTC),
	}

	if err := exp.WriteEntry(e1); err != nil {
		t.Fatalf("WriteEntry: %v", err)
	}
	if err := exp.WriteEntry(e2); err != nil {
		t.Fatalf("WriteEntry: %v", err)
	}
	if err := exp.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var data []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &data); err != nil {
		t.Fatalf("failed to unmarshal JSON export: %v\nOutput: %s", err, buf.String())
	}

	if len(data) != 2 {
		t.Fatalf("expected 2 elements in exported JSON, got %d", len(data))
	}

	if data[0]["message"] != "message one" || data[1]["message"] != "message two" {
		t.Errorf("JSON content mismatch: %+v", data)
	}
}

func TestNDJSONExporter(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := config.ExportConfig{Format: config.ExportNDJSON}

	exp := exporter.New(buf, cfg)
	e := formats.Entry{Level: "INFO", Message: "ndjson msg"}

	exp.WriteEntry(e)
	exp.Flush()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("failed to parse NDJSON line: %v", err)
	}
	if parsed["message"] != "ndjson msg" {
		t.Errorf("message mismatch: %v", parsed["message"])
	}
}

func TestCSVExporter(t *testing.T) {
	buf := new(bytes.Buffer)
	cfg := config.ExportConfig{Format: config.ExportCSV}

	exp := exporter.New(buf, cfg)
	e := formats.Entry{
		Level:     "INFO",
		Message:   "csv message",
		Host:      "localhost",
		Timestamp: time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC),
	}

	exp.WriteEntry(e)
	exp.Flush()

	out := buf.String()
	if !strings.Contains(out, "timestamp,level,host") {
		t.Errorf("expected header row in CSV: %s", out)
	}
	if !strings.Contains(out, "csv message") {
		t.Errorf("expected content row in CSV: %s", out)
	}
}

func TestWriteStats(t *testing.T) {
	buf := new(bytes.Buffer)
	s := statistics.Stats{
		TotalLines:     10,
		ValidEntries:   8,
		InvalidEntries: 2,
		ParseDuration:  5 * time.Millisecond,
		Levels:         map[string]int64{"INFO": 6, "ERROR": 2},
	}

	err := exporter.WriteStats(buf, s, true)
	if err != nil {
		t.Fatalf("WriteStats failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Total lines    : 10") {
		t.Errorf("stats output missing total lines: %s", out)
	}
	if !strings.Contains(out, "Valid entries  : 8") {
		t.Errorf("stats output missing valid entries: %s", out)
	}
}
