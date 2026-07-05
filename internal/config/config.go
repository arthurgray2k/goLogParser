// Package config defines configuration types used throughout goLogParser.
// All layers (library, CLI, internal) share these types to avoid duplication.
package config

import "time"

// Format identifies a supported log format.
type Format string

const (
	FormatAuto          Format = "auto"
	FormatPlainText     Format = "text"
	FormatJSON          Format = "json"
	FormatNDJSON        Format = "ndjson"
	FormatApacheAccess  Format = "apache-access"
	FormatApacheError   Format = "apache-error"
	FormatNginxAccess   Format = "nginx-access"
	FormatNginxError    Format = "nginx-error"
	FormatCLF           Format = "clf"
	FormatCombined      Format = "combined"
	FormatSyslog        Format = "syslog"
)

// ExportFormat identifies the output format for exported results.
type ExportFormat string

const (
	ExportJSON    ExportFormat = "json"
	ExportNDJSON  ExportFormat = "ndjson"
	ExportCSV     ExportFormat = "csv"
	ExportConsole ExportFormat = "console"
)

// Level represents a log severity level.
type Level string

const (
	LevelTrace   Level = "TRACE"
	LevelDebug   Level = "DEBUG"
	LevelInfo    Level = "INFO"
	LevelWarn    Level = "WARN"
	LevelWarning Level = "WARNING"
	LevelError   Level = "ERROR"
	LevelFatal   Level = "FATAL"
	LevelPanic   Level = "PANIC"
	LevelUnknown Level = "UNKNOWN"
)

// ParseConfig controls the behaviour of the parsing pipeline.
type ParseConfig struct {
	// Format is the expected log format. FormatAuto enables auto-detection.
	Format Format

	// Workers is the number of concurrent parser goroutines.
	// Defaults to runtime.NumCPU() when zero.
	Workers int

	// BatchSize is the number of lines sent to each worker in one batch.
	// Defaults to 256 when zero.
	BatchSize int

	// MaxErrors is the maximum number of parse errors tolerated before
	// aborting. Zero means unlimited.
	MaxErrors int

	// Recursive enables recursive directory traversal.
	Recursive bool

	// FilePattern is a glob pattern used to match files during directory
	// traversal. Defaults to "*" when empty.
	FilePattern string

	// FollowSymlinks enables following symbolic links during directory walk.
	FollowSymlinks bool
}

// FilterConfig holds all filtering criteria applied after parsing.
type FilterConfig struct {
	// Levels is a list of log levels to include. Empty means all levels.
	Levels []Level

	// Contains is a list of substrings that the log message must contain.
	Contains []string

	// NotContains is a list of substrings that the log message must not contain.
	NotContains []string

	// Regex is a regular expression the log message must match.
	Regex string

	// Services filters entries by service name.
	Services []string

	// Hosts filters entries by hostname.
	Hosts []string

	// After filters entries at or after this timestamp (inclusive).
	After *time.Time

	// Before filters entries before this timestamp (exclusive).
	Before *time.Time

	// StartsWith filters entries whose message starts with this prefix.
	StartsWith string

	// EndsWith filters entries whose message ends with this suffix.
	EndsWith string

	// Fields is an arbitrary set of key=value pairs matched against
	// structured log fields.
	Fields map[string]string
}

// TailConfig controls the behaviour of the live-follow (tail) mode.
type TailConfig struct {
	// ParseConfig embeds the general parsing options.
	ParseConfig

	// FilterConfig embeds the filter options.
	FilterConfig

	// PollInterval is the delay between file-stat checks when inotify is
	// unavailable.
	PollInterval time.Duration
}

// ExportConfig controls how results are written to the output.
type ExportConfig struct {
	// Format is the desired export encoding.
	Format ExportFormat

	// Pretty enables human-readable (indented) JSON output.
	Pretty bool

	// NoColor disables ANSI colour codes in console output.
	NoColor bool
}

// AppConfig is the top-level configuration read from file / environment.
type AppConfig struct {
	Parse  ParseConfig
	Filter FilterConfig
	Export ExportConfig
}
