// Package formats — helper utilities shared by all parsers in this package.
package formats

import (
	"fmt"
	"strings"
	"time"
)

// commonTimestampLayouts is the ordered list of timestamp formats tried by
// extractTime. More specific / longer formats are tried first.
var commonTimestampLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05.999",
	"2006-01-02 15:04:05",
	"02/Jan/2006:15:04:05 -0700", // Apache / CLF
	"Jan  2 15:04:05",            // Syslog (no year)
	"Jan 02 15:04:05",            // Syslog
	"2006/01/02 15:04:05",
	time.ANSIC,
	time.UnixDate,
	time.RubyDate,
	time.RFC822,
	time.RFC822Z,
	time.RFC850,
	time.RFC1123,
	time.RFC1123Z,
}

// parseTime attempts to parse a string value into time.Time using multiple
// common layouts. It returns the zero time.Time when no layout matches.
func parseTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range commonTimestampLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			// Syslog timestamps have no year; assume the current year.
			if t.Year() == 0 {
				t = t.AddDate(time.Now().Year(), 0, 0)
			}
			return t.UTC()
		}
	}
	return time.Time{}
}

// extractTime looks up the first present key in m whose value is a parseable
// timestamp string or a float64 Unix epoch.
func extractTime(m map[string]any, keys ...string) time.Time {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch tv := v.(type) {
		case string:
			if t := parseTime(tv); !t.IsZero() {
				delete(m, k)
				return t
			}
		case float64:
			// Unix timestamp (seconds, possibly fractional).
			sec := int64(tv)
			nsec := int64((tv - float64(sec)) * 1e9)
			t := time.Unix(sec, nsec).UTC()
			delete(m, k)
			return t
		}
	}
	return time.Time{}
}

// extractString returns the first non-empty string found at any of the given
// keys in m. The matched key is deleted from m to avoid double-reporting in
// Fields.
func extractString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		s := fmt.Sprintf("%v", v)
		if s != "" {
			delete(m, k)
			return s
		}
	}
	return ""
}

// normaliseLevel maps common level strings to the canonical uppercase variants
// used throughout goLogParser.
func normaliseLevel(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "TRACE", "TRC":
		return "TRACE"
	case "DEBUG", "DBG", "D":
		return "DEBUG"
	case "INFO", "INF", "I", "INFORMATION":
		return "INFO"
	case "WARN", "WARNING", "WRN", "W":
		return "WARN"
	case "ERROR", "ERR", "E":
		return "ERROR"
	case "FATAL", "CRIT", "CRITICAL", "F":
		return "FATAL"
	case "PANIC":
		return "PANIC"
	default:
		if raw == "" {
			return "UNKNOWN"
		}
		return strings.ToUpper(raw)
	}
}
