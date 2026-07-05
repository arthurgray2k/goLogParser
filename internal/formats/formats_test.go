package formats_test

import (
	"testing"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/formats"
)

func TestJSONParser(t *testing.T) {
	p := &formats.JSONParser{}
	if p.Name() != "json" {
		t.Errorf("expected json parser name, got %s", p.Name())
	}

	line := `{"timestamp": "2026-07-05T01:15:22Z", "level": "error", "message": "something failed", "host": "localhost"}`
	if p.CanParse(line) < 0.8 {
		t.Errorf("expected JSON parser to be confident about JSON line")
	}

	entry := p.ParseLine(line)
	if entry.ParseError != nil {
		t.Fatalf("expected no parse error, got: %v", entry.ParseError)
	}

	if entry.Level != "ERROR" {
		t.Errorf("expected level ERROR, got %s", entry.Level)
	}
	if entry.Message != "something failed" {
		t.Errorf("expected message, got %s", entry.Message)
	}
	if entry.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", entry.Host)
	}
	expectedTime := time.Date(2026, 7, 5, 1, 15, 22, 0, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}

func TestNDJSONParser(t *testing.T) {
	p, err := formats.ForFormat(config.FormatNDJSON)
	if err != nil {
		t.Fatalf("failed to get ndjson parser: %v", err)
	}
	if p.Name() != "ndjson" {
		t.Errorf("expected ndjson parser name, got %s", p.Name())
	}

	line := `{"level":"info","msg":"service started"}`
	entry := p.ParseLine(line)
	if entry.ParseError != nil {
		t.Fatalf("expected no parse error, got: %v", entry.ParseError)
	}
	if entry.Level != "INFO" {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
}

func TestWebParsers(t *testing.T) {
	t.Run("CLF", func(t *testing.T) {
		p, err := formats.ForFormat(config.FormatCLF)
		if err != nil {
			t.Fatalf("failed to get clf parser: %v", err)
		}
		line := `127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326`
		if p.CanParse(line) < 0.8 {
			t.Errorf("expected CanParse to match CLF line")
		}
		entry := p.ParseLine(line)
		if entry.ParseError != nil {
			t.Fatalf("expected no parse error: %v", entry.ParseError)
		}
		if entry.Host != "127.0.0.1" {
			t.Errorf("expected host 127.0.0.1, got %s", entry.Host)
		}
		if entry.Message != "GET /apache_pb.gif HTTP/1.0" {
			t.Errorf("expected message, got %s", entry.Message)
		}
	})

	t.Run("Combined", func(t *testing.T) {
		p, err := formats.ForFormat(config.FormatCombined)
		if err != nil {
			t.Fatalf("failed to get combined parser: %v", err)
		}
		line := `127.0.0.1 - frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326 "http://www.example.com/start.html" "Mozilla/4.08 [en] (Win98; I ;Nav)"`
		if p.CanParse(line) < 0.8 {
			t.Errorf("expected CanParse to match Combined line")
		}
		entry := p.ParseLine(line)
		if entry.ParseError != nil {
			t.Fatalf("expected no parse error: %v", entry.ParseError)
		}
		if entry.Fields["referer"] != "http://www.example.com/start.html" {
			t.Errorf("expected referer, got %v", entry.Fields["referer"])
		}
	})

	t.Run("Apache Error", func(t *testing.T) {
		p, err := formats.ForFormat(config.FormatApacheError)
		if err != nil {
			t.Fatalf("failed to get apache error parser: %v", err)
		}
		line := `[Wed Oct 11 14:32:52.123456 2000] [error] [client 1.2.3.4] Directory index forbidden by buffered index file`
		if p.CanParse(line) < 0.8 {
			t.Errorf("expected CanParse to match apache error")
		}
		entry := p.ParseLine(line)
		if entry.ParseError != nil {
			t.Fatalf("expected no parse error: %v", entry.ParseError)
		}
		if entry.Level != "ERROR" {
			t.Errorf("expected level ERROR, got %s", entry.Level)
		}
		if entry.Fields["client"] != "1.2.3.4" {
			t.Errorf("expected client field: %v", entry.Fields["client"])
		}
	})

	t.Run("Nginx Error", func(t *testing.T) {
		p, err := formats.ForFormat(config.FormatNginxError)
		if err != nil {
			t.Fatalf("failed to get nginx error parser: %v", err)
		}
		line := `2024/01/15 12:34:56 [error] 1234#0: *5 open() "/usr/share/nginx/html/favicon.ico" failed`
		if p.CanParse(line) < 0.8 {
			t.Errorf("expected CanParse to match nginx error")
		}
		entry := p.ParseLine(line)
		if entry.ParseError != nil {
			t.Fatalf("expected no parse error: %v", entry.ParseError)
		}
		if entry.Level != "ERROR" {
			t.Errorf("expected level ERROR, got %s", entry.Level)
		}
		if entry.PID != 1234 {
			t.Errorf("expected PID 1234, got %d", entry.PID)
		}
	})
}

func TestSyslogParser(t *testing.T) {
	p := &formats.SyslogParser{}

	t.Run("RFC3164", func(t *testing.T) {
		line := `Oct 11 22:14:15 myhost myproc[123]: failed connection`
		if p.CanParse(line) < 0.8 {
			t.Errorf("expected CanParse to match syslog 3164")
		}
		entry := p.ParseLine(line)
		if entry.ParseError != nil {
			t.Fatalf("expected no parse error: %v", entry.ParseError)
		}
		if entry.Host != "myhost" {
			t.Errorf("expected host, got %s", entry.Host)
		}
		if entry.PID != 123 {
			t.Errorf("expected PID 123, got %d", entry.PID)
		}
		if entry.Level != "ERROR" { // derived from keyword "failed"
			t.Errorf("expected level ERROR, got %s", entry.Level)
		}
	})

	t.Run("RFC5424", func(t *testing.T) {
		line := `<34>1 2003-10-11T22:14:15.003Z myhost.example.com su 770 mymsgid - 'su root' failed for lonvick`
		if p.CanParse(line) < 0.8 {
			t.Errorf("expected CanParse to match syslog 5424")
		}
		entry := p.ParseLine(line)
		if entry.ParseError != nil {
			t.Fatalf("expected no parse error: %v", entry.ParseError)
		}
		if entry.Level != "FATAL" { // 34%8 = 2 (CRIT -> FATAL)
			t.Errorf("expected level FATAL, got %s", entry.Level)
		}
		if entry.Host != "myhost.example.com" {
			t.Errorf("expected host, got %s", entry.Host)
		}
	})
}

func TestPlainTextParser(t *testing.T) {
	p := &formats.PlainTextParser{}
	line := `2026-07-05 01:15:22 [WARN] hello standard format`
	entry := p.ParseLine(line)
	if entry.ParseError != nil {
		t.Fatalf("expected no parse error: %v", entry.ParseError)
	}
	if entry.Level != "WARN" {
		t.Errorf("expected WARN, got %s", entry.Level)
	}
}
