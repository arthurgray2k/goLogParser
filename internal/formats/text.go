package formats

import (
	"regexp"
	"strings"
)

// PlainTextParser is the fallback parser for unstructured log lines.
// It attempts to extract a timestamp and log level from common plain-text
// patterns, then treats the entire line as the message.
type PlainTextParser struct{}

func init() { RegisterParser(&PlainTextParser{}) }

// Name returns "text".
func (p *PlainTextParser) Name() string { return "text" }

// CanParse always returns a low confidence so that more specific parsers win.
func (p *PlainTextParser) CanParse(_ string) float64 { return 0.1 }

// rePlainLevel captures common inline level tokens, e.g.:
//
//	[ERROR]  ERROR:  WARN -  (INFO)
var rePlainLevel = regexp.MustCompile(
	`(?i)\b(TRACE|DEBUG|INFO|NOTICE|WARN(?:ING)?|ERROR|ERR|FATAL|CRIT(?:ICAL)?|PANIC|EMERG(?:ENCY)?)\b`,
)

// rePlainTimestamp captures ISO-8601-like timestamp prefixes.
var rePlainTimestamp = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?)`,
)

// ParseLine extracts what it can from a plain-text log line.
func (p *PlainTextParser) ParseLine(line string) Entry {
	e := Entry{Raw: line}

	// Try to extract a leading timestamp.
	if m := rePlainTimestamp.FindString(line); m != "" {
		e.Timestamp = parseTime(m)
		line = strings.TrimSpace(line[len(m):])
	}

	// Try to find a level keyword.
	if m := rePlainLevel.FindStringSubmatch(line); m != nil {
		e.Level = normaliseLevel(m[1])
		// Strip the level keyword and surrounding decorators (e.g., [INFO], INFO:, INFO -)
		idx := strings.Index(strings.ToUpper(line), strings.ToUpper(m[1]))
		if idx != -1 {
			start := idx
			if start > 0 && (line[start-1] == '[' || line[start-1] == '(') {
				start--
			}
			end := idx + len(m[1])
			if end < len(line) && (line[end] == ']' || line[end] == ')') {
				end++
			}
			for end < len(line) && (line[end] == ' ' || line[end] == ':' || line[end] == '-' || line[end] == '\t') {
				end++
			}
			line = line[:start] + line[end:]
		}
	}

	if e.Level == "" {
		e.Level = "UNKNOWN"
	}

	e.Message = strings.TrimSpace(line)
	return e
}
