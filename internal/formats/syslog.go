package formats

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// reSyslog matches RFC 3164 syslog lines:
// Jan  1 00:00:00 hostname process[pid]: message
// Also handles RFC 5424 and systemd journal export formats.
var reSyslog3164 = regexp.MustCompile(
	`^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s+(.+)$`,
)

// reSyslog5424 matches RFC 5424:
// <priority>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
var reSyslog5424 = regexp.MustCompile(
	`^<(\d+)>(\d)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(?:-|\[.*?\])\s+(.+)$`,
)

// priorityToLevel converts a syslog priority integer to a level string.
// Syslog severity: 0=EMERG 1=ALERT 2=CRIT 3=ERR 4=WARNING 5=NOTICE 6=INFO 7=DEBUG
func priorityToLevel(priority int) string {
	severity := priority % 8
	switch severity {
	case 0, 1:
		return "FATAL"
	case 2:
		return "FATAL"
	case 3:
		return "ERROR"
	case 4:
		return "WARN"
	case 5, 6:
		return "INFO"
	case 7:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// SyslogParser handles both RFC 3164 and RFC 5424 syslog formats.
type SyslogParser struct{}

func init() { RegisterParser(&SyslogParser{}) }

// Name returns "syslog".
func (p *SyslogParser) Name() string { return "syslog" }

// CanParse returns a high confidence when the line matches a known syslog pattern.
func (p *SyslogParser) CanParse(sample string) float64 {
	if reSyslog5424.MatchString(sample) && strings.HasPrefix(sample, "<") {
		return 0.95
	}
	if reSyslog3164.MatchString(sample) {
		return 0.85
	}
	return 0
}

// ParseLine parses a single syslog line.
func (p *SyslogParser) ParseLine(line string) Entry {
	e := Entry{Raw: line}

	// Try RFC 5424 first.
	if m := reSyslog5424.FindStringSubmatch(line); m != nil {
		priority, _ := strconv.Atoi(m[1])
		e.Level = priorityToLevel(priority)
		e.Timestamp = parseTime(m[3])
		e.Host = m[4]
		e.Service = m[5]
		e.Process = m[5]
		if pid, err := strconv.Atoi(m[6]); err == nil {
			e.PID = pid
		}
		e.Message = strings.TrimSpace(m[8])
		e.Fields = map[string]any{
			"version": m[2],
			"msgid":   m[7],
		}
		return e
	}

	// Try RFC 3164.
	if m := reSyslog3164.FindStringSubmatch(line); m != nil {
		e.Timestamp = parseTime(m[1])
		e.Host = m[2]
		e.Service = m[3]
		e.Process = m[3]
		if pid, err := strconv.Atoi(m[4]); err == nil {
			e.PID = pid
		}
		e.Message = strings.TrimSpace(m[5])
		// RFC 3164 has no inline severity — default to INFO.
		e.Level = "INFO"

		// Common kernel / systemd messages contain "error" / "warn" / "crit".
		lower := strings.ToLower(e.Message)
		switch {
		case strings.Contains(lower, "error") || strings.Contains(lower, "fail"):
			e.Level = "ERROR"
		case strings.Contains(lower, "warn"):
			e.Level = "WARN"
		case strings.Contains(lower, "crit") || strings.Contains(lower, "emerg"):
			e.Level = "FATAL"
		case strings.Contains(lower, "debug"):
			e.Level = "DEBUG"
		}
		return e
	}

	e.ParseError = fmt.Errorf("line does not match any syslog format")
	return e
}
