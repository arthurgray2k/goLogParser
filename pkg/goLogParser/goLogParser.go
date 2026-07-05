// Package goLogParser is the public API of the goLogParser log parsing
// framework. Import this package to embed log parsing capabilities in your
// own Go application.
//
// Quick start:
//
//	p := goLogParser.New()
//
//	result, err := p.ParseFile(ctx, "application.log")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	errors := goLogParser.Filter(result, goLogParser.Level("ERROR"))
//	stats  := result.Stats
//
// All operations stream data and do not load the entire file into memory.
package goLogParser

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/filters"
	"github.com/arthurgray2k/goLogParser/internal/formats"
	"github.com/arthurgray2k/goLogParser/internal/parser"
	"github.com/arthurgray2k/goLogParser/internal/reader"
	"github.com/arthurgray2k/goLogParser/internal/statistics"
	"github.com/arthurgray2k/goLogParser/internal/tail"
)

// Re-export core types so callers need only import this package.

// Entry is a parsed log entry. See internal/formats for field documentation.
type Entry = formats.Entry

// Stats holds aggregate metrics from a parse run.
type Stats = statistics.Stats

// Filter is a predicate over a log entry.
type Filter = filters.Filter

// Format identifies a supported log format.
type Format = config.Format

// ExportFormat identifies a supported export format.
type ExportFormat = config.ExportFormat

// Level is a log severity level string.
type Level = config.Level

// Result holds the outcome of a parse operation.
type Result = parser.Result

// TailLine is a parsed entry emitted during a live-tail session.
type TailLine = tail.Line

// ParseConfig controls parsing behaviour.
type ParseConfig = config.ParseConfig

// FilterConfig controls which entries are accepted.
type FilterConfig = config.FilterConfig

// ExportConfig controls result serialisation.
type ExportConfig = config.ExportConfig

// TailConfig controls the live-tail behaviour.
type TailConfig = config.TailConfig

// Format constants.
const (
	FormatAuto         = config.FormatAuto
	FormatPlainText    = config.FormatPlainText
	FormatJSON         = config.FormatJSON
	FormatNDJSON       = config.FormatNDJSON
	FormatApacheAccess = config.FormatApacheAccess
	FormatApacheError  = config.FormatApacheError
	FormatNginxAccess  = config.FormatNginxAccess
	FormatNginxError   = config.FormatNginxError
	FormatCLF          = config.FormatCLF
	FormatCombined     = config.FormatCombined
	FormatSyslog       = config.FormatSyslog
)

// ExportFormat constants.
const (
	ExportJSON    = config.ExportJSON
	ExportNDJSON  = config.ExportNDJSON
	ExportCSV     = config.ExportCSV
	ExportConsole = config.ExportConsole
)

// Level constants.
const (
	LevelTrace   = config.LevelTrace
	LevelDebug   = config.LevelDebug
	LevelInfo    = config.LevelInfo
	LevelWarn    = config.LevelWarn
	LevelWarning = config.LevelWarning
	LevelError   = config.LevelError
	LevelFatal   = config.LevelFatal
	LevelPanic   = config.LevelPanic
	LevelUnknown = config.LevelUnknown
)

// ─── Parser ───────────────────────────────────────────────────────────────────

// Parser is the high-level entry point for the goLogParser library.
type Parser struct {
	logger *slog.Logger
}

// New creates a Parser using the default structured logger.
func New() *Parser {
	return &Parser{logger: slog.Default()}
}

// NewWithLogger creates a Parser that uses the supplied *slog.Logger.
func NewWithLogger(l *slog.Logger) *Parser {
	return &Parser{logger: l}
}

// ParseFile parses the file at path using the supplied configs and returns a
// Result. Both ParseConfig and FilterConfig may be zero-valued to use defaults.
func (p *Parser) ParseFile(ctx context.Context, path string, cfg ParseConfig, filter FilterConfig) (Result, error) {
	srcs, err := reader.OpenFile(path)
	if err != nil {
		return Result{}, err
	}

	return p.parseSources(ctx, srcs, cfg, filter)
}

// ParseDir parses all matching files under dir.
func (p *Parser) ParseDir(ctx context.Context, dir string, cfg ParseConfig, filter FilterConfig, opts reader.Options) (Result, error) {
	srcs, err := reader.OpenDir(ctx, dir, opts)
	if err != nil {
		return Result{}, err
	}
	return p.parseSources(ctx, srcs, cfg, filter)
}

// ParseReader parses log data from r (e.g. os.Stdin).
func (p *Parser) ParseReader(ctx context.Context, r io.Reader, name string, cfg ParseConfig, filter FilterConfig) (Result, error) {
	srcs := []reader.Source{{Name: name, RC: io.NopCloser(r)}}
	return p.parseSources(ctx, srcs, cfg, filter)
}

// ParseStdin parses log data from standard input.
func (p *Parser) ParseStdin(ctx context.Context, cfg ParseConfig, filter FilterConfig) (Result, error) {
	return p.ParseReader(ctx, os.Stdin, "stdin", cfg, filter)
}

// parseSources runs the pipeline over all given Sources.
func (p *Parser) parseSources(ctx context.Context, srcs []reader.Source, cfg ParseConfig, filterCfg FilterConfig) (Result, error) {
	pl, err := parser.New(cfg, filterCfg, p.logger)
	if err != nil {
		return Result{}, err
	}

	var combined Result

	for _, src := range srcs {
		func() {
			defer src.RC.Close()
			res, runErr := pl.Run(ctx, src.RC, src.Name)
			if runErr != nil {
				combined.Errors = append(combined.Errors, runErr)
				return
			}
			combined.Entries = append(combined.Entries, res.Entries...)
			combined.Errors = append(combined.Errors, res.Errors...)
			mergeStats(&combined.Stats, res.Stats)
		}()
	}

	return combined, nil
}

// ─── Filtering ────────────────────────────────────────────────────────────────

// FilterEntries returns the subset of entries from result that satisfy every
// supplied Filter.
func FilterEntries(result Result, f ...Filter) []Entry {
	composite := filters.And(f...)
	var out []Entry
	for _, e := range result.Entries {
		if composite.Match(e) {
			out = append(out, e)
		}
	}
	return out
}

// LevelFilter returns a Filter that accepts entries at any of the given levels.
func LevelFilter(levels ...Level) Filter {
	return filters.Level(levels...)
}

// ContainsFilter returns a Filter that accepts entries whose message contains
// all of the given substrings.
func ContainsFilter(substrings ...string) Filter {
	return filters.Contains(substrings...)
}

// RegexFilter returns a Filter that accepts entries matching the given regex.
func RegexFilter(pattern string) (Filter, error) {
	return filters.Regex(pattern)
}

// ─── Custom parsers ───────────────────────────────────────────────────────────

// FormatParser mirrors the internal formats.Parser interface, exposed publicly
// so callers can register their own parsers.
type FormatParser interface {
	Name() string
	CanParse(sample string) float64
	ParseLine(line string) Entry
}

// RegisterParser adds a custom parser to the global registry.
// Call from an init() function or before the first ParseFile call.
func RegisterParser(p FormatParser) {
	formats.RegisterParser(p)
}

// ─── Live tail ────────────────────────────────────────────────────────────────

// Tail follows path and returns a channel of new log entries.
// The channel is closed when ctx is cancelled.
func (p *Parser) Tail(ctx context.Context, path string, cfg TailConfig) (<-chan TailLine, error) {
	return tail.Follow(ctx, path, cfg, p.logger)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mergeStats(dst *statistics.Stats, src statistics.Stats) {
	dst.TotalLines += src.TotalLines
	dst.ValidEntries += src.ValidEntries
	dst.InvalidEntries += src.InvalidEntries
	dst.ParseDuration += src.ParseDuration

	if dst.Levels == nil {
		dst.Levels = make(map[string]int64)
	}
	for k, v := range src.Levels {
		dst.Levels[k] += v
	}

	if dst.Services == nil {
		dst.Services = make(map[string]int64)
	}
	for k, v := range src.Services {
		dst.Services[k] += v
	}

	if dst.Hosts == nil {
		dst.Hosts = make(map[string]int64)
	}
	for k, v := range src.Hosts {
		dst.Hosts[k] += v
	}

	if !src.EarliestTimestamp.IsZero() {
		if dst.EarliestTimestamp.IsZero() || src.EarliestTimestamp.Before(dst.EarliestTimestamp) {
			dst.EarliestTimestamp = src.EarliestTimestamp
		}
	}
	if src.LatestTimestamp.After(dst.LatestTimestamp) {
		dst.LatestTimestamp = src.LatestTimestamp
	}
}
