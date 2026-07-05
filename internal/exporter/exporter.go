// Package exporter writes parsed log entries and statistics to an io.Writer
// in the format requested by the caller (JSON, NDJSON, CSV, console).
package exporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/formats"
	"github.com/arthurgray2k/goLogParser/internal/statistics"
)

// Exporter writes entries to a writer in a specific format.
type Exporter interface {
	// WriteEntry writes a single log entry.
	WriteEntry(e formats.Entry) error
	// Flush flushes any buffered data and writes a format-appropriate footer.
	Flush() error
}

// New returns an Exporter for the given ExportConfig writing to w.
func New(w io.Writer, cfg config.ExportConfig) Exporter {
	switch cfg.Format {
	case config.ExportNDJSON:
		return &ndjsonExporter{w: w}
	case config.ExportCSV:
		return newCSVExporter(w)
	case config.ExportConsole:
		return &consoleExporter{w: w, noColor: cfg.NoColor}
	default: // config.ExportJSON
		return &jsonExporter{w: w, pretty: cfg.Pretty, first: true}
	}
}

// ─── JSON ─────────────────────────────────────────────────────────────────────

type jsonExporter struct {
	w      io.Writer
	pretty bool
	first  bool
	opened bool
}

func (e *jsonExporter) WriteEntry(entry formats.Entry) error {
	if !e.opened {
		if _, err := io.WriteString(e.w, "[\n"); err != nil {
			return err
		}
		e.opened = true
	}
	if !e.first {
		if _, err := io.WriteString(e.w, ",\n"); err != nil {
			return err
		}
	}
	e.first = false

	var b []byte
	var err error
	if e.pretty {
		b, err = json.MarshalIndent(entryToMap(entry), "  ", "  ")
	} else {
		b, err = json.Marshal(entryToMap(entry))
	}
	if err != nil {
		return err
	}
	if e.pretty {
		_, err = fmt.Fprintf(e.w, "  %s", b)
	} else {
		_, err = e.w.Write(b)
	}
	return err
}

func (e *jsonExporter) Flush() error {
	if !e.opened {
		_, err := io.WriteString(e.w, "[]\n")
		return err
	}
	_, err := io.WriteString(e.w, "\n]\n")
	return err
}

// ─── NDJSON ───────────────────────────────────────────────────────────────────

type ndjsonExporter struct{ w io.Writer }

func (e *ndjsonExporter) WriteEntry(entry formats.Entry) error {
	b, err := json.Marshal(entryToMap(entry))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(e.w, "%s\n", b)
	return err
}

func (e *ndjsonExporter) Flush() error { return nil }

// ─── CSV ──────────────────────────────────────────────────────────────────────

var csvHeaders = []string{
	"timestamp", "level", "host", "service", "process", "pid", "thread", "message", "raw",
}

type csvExporter struct {
	cw      *csv.Writer
	written bool
}

func newCSVExporter(w io.Writer) *csvExporter {
	return &csvExporter{cw: csv.NewWriter(w)}
}

func (e *csvExporter) WriteEntry(entry formats.Entry) error {
	if !e.written {
		if err := e.cw.Write(csvHeaders); err != nil {
			return err
		}
		e.written = true
	}
	row := []string{
		fmtTime(entry.Timestamp),
		entry.Level,
		entry.Host,
		entry.Service,
		entry.Process,
		fmt.Sprint(entry.PID),
		entry.Thread,
		entry.Message,
		entry.Raw,
	}
	return e.cw.Write(row)
}

func (e *csvExporter) Flush() error {
	e.cw.Flush()
	return e.cw.Error()
}

// ─── Console ──────────────────────────────────────────────────────────────────

// ANSI colour codes.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
	ansiWhite  = "\033[97m"
)

type consoleExporter struct {
	w       io.Writer
	noColor bool
}

func (e *consoleExporter) WriteEntry(entry formats.Entry) error {
	ts := fmtTime(entry.Timestamp)
	level := padRight(entry.Level, 7)
	msg := entry.Message
	if msg == "" {
		msg = entry.Raw
	}

	if e.noColor {
		_, err := fmt.Fprintf(e.w, "%s  %s  %s\n", ts, level, msg)
		return err
	}

	color := levelColor(entry.Level)
	_, err := fmt.Fprintf(e.w, "%s%s%s  %s%s%s  %s\n",
		ansiGray, ts, ansiReset,
		color, level, ansiReset,
		msg,
	)
	return err
}

func (e *consoleExporter) Flush() error { return nil }

func levelColor(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR", "FATAL", "PANIC":
		return ansiRed
	case "WARN", "WARNING":
		return ansiYellow
	case "DEBUG", "TRACE":
		return ansiGray
	case "INFO":
		return ansiCyan
	default:
		return ansiWhite
	}
}

// ─── Stats exporter ───────────────────────────────────────────────────────────

// WriteStats writes a human-readable statistics summary to w.
func WriteStats(w io.Writer, s statistics.Stats, noColor bool) error {
	sep := strings.Repeat("─", 50)

	lines := []string{
		sep,
		fmt.Sprintf("  Total lines    : %d", s.TotalLines),
		fmt.Sprintf("  Valid entries  : %d", s.ValidEntries),
		fmt.Sprintf("  Invalid entries: %d", s.InvalidEntries),
		fmt.Sprintf("  Parse duration : %s", s.ParseDuration),
	}

	if !s.EarliestTimestamp.IsZero() {
		lines = append(lines,
			fmt.Sprintf("  Time range     : %s → %s",
				fmtTime(s.EarliestTimestamp), fmtTime(s.LatestTimestamp)),
		)
	}

	lines = append(lines, "", "  Log Levels:")
	for level, count := range s.Levels {
		lines = append(lines, fmt.Sprintf("    %-10s %d", level, count))
	}

	if len(s.Services) > 0 {
		lines = append(lines, "", fmt.Sprintf("  Unique services: %d", len(s.Services)))
	}
	if len(s.Hosts) > 0 {
		lines = append(lines, fmt.Sprintf("  Unique hosts   : %d", len(s.Hosts)))
	}
	lines = append(lines, sep)

	for _, l := range lines {
		if _, err := fmt.Fprintln(w, l); err != nil {
			return err
		}
	}
	return nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func entryToMap(e formats.Entry) map[string]any {
	m := map[string]any{
		"raw":     e.Raw,
		"level":   e.Level,
		"message": e.Message,
	}
	if !e.Timestamp.IsZero() {
		m["timestamp"] = e.Timestamp.Format(time.RFC3339Nano)
	}
	if e.Host != "" {
		m["host"] = e.Host
	}
	if e.Service != "" {
		m["service"] = e.Service
	}
	if e.Process != "" {
		m["process"] = e.Process
	}
	if e.PID != 0 {
		m["pid"] = e.PID
	}
	if e.Thread != "" {
		m["thread"] = e.Thread
	}
	for k, v := range e.Fields {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	if e.ParseError != nil {
		m["parse_error"] = e.ParseError.Error()
	}
	return m
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
