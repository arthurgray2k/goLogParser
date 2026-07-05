package formats

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ─── Apache / Nginx shared ────────────────────────────────────────────────────

// Combined Log Format (CLF + Referer + User-Agent):
//   %h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"
//
// Common Log Format (CLF):
//   %h %l %u %t "%r" %>s %b

// reCLF matches: host ident user [timestamp] "request" status bytes
var reCLF = regexp.MustCompile(
	`^(\S+)\s+(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+"([^"]*?)"\s+(\d{3}|-)\s+(\d+|-)`,
)

// reCombined matches CLF + referer + user-agent
var reCombined = regexp.MustCompile(
	`^(\S+)\s+(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+"([^"]*?)"\s+(\d{3}|-)\s+(\d+|-)\s+"([^"]*)"\s+"([^"]*)"`,
)

func parseCLFLine(line string, combined bool) Entry {
	e := Entry{Raw: line}

	re := reCLF
	if combined {
		re = reCombined
	}

	m := re.FindStringSubmatch(line)
	if m == nil {
		e.ParseError = fmt.Errorf("line does not match CLF format")
		return e
	}

	e.Host = m[1]
	// m[2] = ident (usually "-")
	// m[3] = user (usually "-")
	e.Timestamp = parseTime(m[4])
	request := m[5]
	statusCode := m[6]
	bytesStr := m[7]

	fields := map[string]any{
		"ident":   m[2],
		"user":    m[3],
		"request": request,
		"status":  statusCode,
		"bytes":   bytesStr,
	}

	if combined && len(m) >= 10 {
		fields["referer"] = m[8]
		fields["user_agent"] = m[9]
	}

	// Derive level from HTTP status code.
	if code, err := strconv.Atoi(statusCode); err == nil {
		switch {
		case code >= 500:
			e.Level = "ERROR"
		case code >= 400:
			e.Level = "WARN"
		default:
			e.Level = "INFO"
		}
		fields["status_code"] = code
	}

	// Request → method + path + protocol
	parts := strings.Fields(request)
	if len(parts) >= 2 {
		fields["method"] = parts[0]
		fields["path"] = parts[1]
	}
	if len(parts) == 3 {
		fields["protocol"] = parts[2]
	}

	e.Message = request
	e.Fields = fields
	return e
}

// ─── Common Log Format (CLF) ─────────────────────────────────────────────────

// CLFParser parses the W3C / NCSA Common Log Format.
type CLFParser struct{}

func init() { RegisterParser(&CLFParser{}) }

func (p *CLFParser) Name() string { return "clf" }

func (p *CLFParser) CanParse(sample string) float64 {
	if reCLF.MatchString(sample) {
		return 0.8
	}
	return 0
}

func (p *CLFParser) ParseLine(line string) Entry { return parseCLFLine(line, false) }

// ─── Combined Log Format ──────────────────────────────────────────────────────

// CombinedParser parses the Combined Log Format (CLF + Referer + User-Agent).
type CombinedParser struct{}

func init() { RegisterParser(&CombinedParser{}) }

func (p *CombinedParser) Name() string { return "combined" }

func (p *CombinedParser) CanParse(sample string) float64 {
	if reCombined.MatchString(sample) {
		return 0.9
	}
	return 0
}

func (p *CombinedParser) ParseLine(line string) Entry { return parseCLFLine(line, true) }

// ─── Apache Access ────────────────────────────────────────────────────────────

// ApacheAccessParser is an alias for Combined Log Format as Apache defaults to it.
type ApacheAccessParser struct {
	combined CombinedParser
	clf      CLFParser
}

func init() { RegisterParser(&ApacheAccessParser{}) }

// Name returns "apache-access".
func (p *ApacheAccessParser) Name() string { return "apache-access" }

func (p *ApacheAccessParser) CanParse(sample string) float64 {
	score := p.combined.CanParse(sample)
	if score > 0 {
		return score
	}
	return p.clf.CanParse(sample) // try CLF
}

func (p *ApacheAccessParser) ParseLine(line string) Entry {
	e := p.combined.ParseLine(line)
	if e.ParseError != nil {
		e = p.clf.ParseLine(line)
	}
	e.Service = "apache"
	return e
}

// ─── Apache Error ─────────────────────────────────────────────────────────────

// reApacheError matches Apache 2.4 error log format:
// [Wed Oct 11 14:32:52.123456 2000] [error] [client 1.2.3.4] message
var reApacheError = regexp.MustCompile(
	`^\[([^\]]+)\]\s+\[([^\]]+)\]\s+(?:\[([^\]]+)\]\s+)?(.+)$`,
)

// ApacheErrorParser parses Apache HTTP server error log lines.
type ApacheErrorParser struct{}

func init() { RegisterParser(&ApacheErrorParser{}) }

func (p *ApacheErrorParser) Name() string { return "apache-error" }

func (p *ApacheErrorParser) CanParse(sample string) float64 {
	if reApacheError.MatchString(sample) && strings.HasPrefix(sample, "[") {
		if strings.Contains(sample, "] [") {
			return 0.85
		}
	}
	return 0
}

func (p *ApacheErrorParser) ParseLine(line string) Entry {
	e := Entry{Raw: line, Service: "apache"}
	m := reApacheError.FindStringSubmatch(line)
	if m == nil {
		e.ParseError = fmt.Errorf("line does not match apache error format")
		return e
	}

	e.Timestamp = parseTime(m[1])
	e.Level = normaliseLevel(m[2])

	fields := map[string]any{}
	if m[3] != "" {
		// Could be "client x.x.x.x" or "pid 1234"
		extra := m[3]
		if strings.HasPrefix(extra, "client ") {
			fields["client"] = strings.TrimPrefix(extra, "client ")
		} else if strings.HasPrefix(extra, "pid ") {
			if pid, err := strconv.Atoi(strings.TrimPrefix(extra, "pid ")); err == nil {
				e.PID = pid
			}
		} else {
			fields["extra"] = extra
		}
	}

	e.Message = strings.TrimSpace(m[4])
	e.Fields = fields
	return e
}

// ─── Nginx Access ─────────────────────────────────────────────────────────────

// NginxAccessParser is identical to Combined Log Format (Nginx default).
type NginxAccessParser struct{ inner CombinedParser }

func init() { RegisterParser(&NginxAccessParser{}) }

func (p *NginxAccessParser) Name() string { return "nginx-access" }

func (p *NginxAccessParser) CanParse(sample string) float64 {
	if reCombined.MatchString(sample) {
		return 0.85
	}
	return 0
}

func (p *NginxAccessParser) ParseLine(line string) Entry {
	e := p.inner.ParseLine(line)
	e.Service = "nginx"
	return e
}

// ─── Nginx Error ──────────────────────────────────────────────────────────────

// reNginxError matches Nginx error log format:
// 2024/01/15 12:34:56 [error] 1234#0: *5 message
var reNginxError = regexp.MustCompile(
	`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\s+\[(\w+)\]\s+(\d+)#(\d+):\s+(?:\*(\d+)\s+)?(.+)$`,
)

// NginxErrorParser parses Nginx error log lines.
type NginxErrorParser struct{}

func init() { RegisterParser(&NginxErrorParser{}) }

func (p *NginxErrorParser) Name() string { return "nginx-error" }

func (p *NginxErrorParser) CanParse(sample string) float64 {
	if reNginxError.MatchString(sample) {
		return 0.9
	}
	return 0
}

func (p *NginxErrorParser) ParseLine(line string) Entry {
	e := Entry{Raw: line, Service: "nginx"}
	m := reNginxError.FindStringSubmatch(line)
	if m == nil {
		e.ParseError = fmt.Errorf("line does not match nginx error format")
		return e
	}

	e.Timestamp = parseTime(m[1])
	e.Level = normaliseLevel(m[2])

	pid, _ := strconv.Atoi(m[3])
	e.PID = pid

	fields := map[string]any{
		"worker": m[4],
	}
	if m[5] != "" {
		fields["connection"] = m[5]
	}

	e.Message = strings.TrimSpace(m[6])
	e.Fields = fields
	return e
}
