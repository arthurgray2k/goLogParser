// Package formats defines the LogEntry type and the Parser interface that
// every log-format implementation must satisfy.
package formats

import (
	"time"
)

// Entry represents a single, normalised log entry produced by any parser.
// Parsers populate only the fields they can extract; zero-values signal
// absence of data.
type Entry struct {
	// Raw is the original, unmodified log line.
	Raw string

	// Timestamp is the parsed log timestamp in UTC. Zero value means unknown.
	Timestamp time.Time

	// Level is the normalised severity level (e.g. "ERROR", "INFO").
	Level string

	// Host is the originating hostname.
	Host string

	// Service is the service or application name.
	Service string

	// Process is the process name or identifier.
	Process string

	// PID is the numeric process ID when available.
	PID int

	// Thread is the thread or goroutine identifier.
	Thread string

	// Message is the human-readable log message body.
	Message string

	// Fields holds any additional structured key/value data extracted by
	// the parser (e.g. JSON log fields, HTTP status codes, …).
	Fields map[string]any

	// ParseError is non-nil when the line could not be fully parsed. The
	// entry is still included in results with Raw populated.
	ParseError error
}

// Parser is the interface that every log-format implementation must satisfy.
// Implementations must be safe for concurrent use.
type Parser interface {
	// Name returns a human-readable identifier for this format.
	Name() string

	// CanParse returns a confidence score in [0, 1] indicating how likely
	// this parser can handle the provided sample line. Detectors call this
	// during auto-detection. A score ≥ 0.8 is treated as a confident match.
	CanParse(sample string) float64

	// ParseLine parses a single log line and returns the corresponding Entry.
	// It must never panic. If the line cannot be parsed, it returns the line
	// in Entry.Raw with a non-nil Entry.ParseError.
	ParseLine(line string) Entry
}

// Registry holds all registered parsers. It is populated by init() functions
// in each format sub-package and by callers of RegisterParser.
var registry []Parser

// RegisterParser adds p to the global parser registry.
// It is safe to call from init() functions in format packages.
func RegisterParser(p Parser) {
	registry = append(registry, p)
}

// Registered returns a copy of the currently registered parsers.
func Registered() []Parser {
	out := make([]Parser, len(registry))
	copy(out, registry)
	return out
}
