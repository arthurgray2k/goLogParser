package formats

import (
	"encoding/json"
	"strings"
)

// JSONParser parses structured JSON log lines (one JSON object per line).
// It handles both single-line JSON objects and pretty-printed single-line
// variants produced by common frameworks.
type JSONParser struct{}

func init() { RegisterParser(&JSONParser{}) }

// Name returns "json".
func (p *JSONParser) Name() string { return "json" }

// CanParse returns a high confidence score when the sample looks like a JSON
// object that contains at least one of the common log-level keys.
func (p *JSONParser) CanParse(sample string) float64 {
	s := strings.TrimSpace(sample)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return 0
	}
	var probe map[string]any
	if json.Unmarshal([]byte(s), &probe) != nil {
		return 0
	}
	for _, key := range []string{"level", "severity", "lvl", "log_level"} {
		if _, ok := probe[key]; ok {
			return 1.0
		}
	}
	// Looks like JSON but no level key — still plausible.
	return 0.6
}

// ParseLine parses a single JSON log line into an Entry.
func (p *JSONParser) ParseLine(line string) Entry {
	e := Entry{Raw: line}
	s := strings.TrimSpace(line)

	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		e.ParseError = err
		return e
	}

	e.Timestamp = extractTime(m, "timestamp", "time", "ts", "@timestamp", "date")
	e.Level = strings.ToUpper(extractString(m, "level", "severity", "lvl", "log_level"))
	e.Message = extractString(m, "message", "msg", "log", "body", "text")
	e.Host = extractString(m, "host", "hostname", "node")
	e.Service = extractString(m, "service", "app", "application", "name", "logger")
	e.Process = extractString(m, "process", "proc")
	e.Thread = extractString(m, "thread", "goroutine")
	e.Fields = m

	return e
}
