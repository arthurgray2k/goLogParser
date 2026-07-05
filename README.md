# goLogParser

`goLogParser` is a fast, scalable, and extensible concurrent log processing framework written in Go. It is designed to parse large log files efficiently, stream data without loading entire files into memory, and support a variety of common log formats out of the box.

This project delivers two components:
1. **A standalone CLI application** for command-line log analysis.
2. **A reusable Go package** for integration into other Go applications.

## Features

- **Concurrent Processing:** Uses a worker-pool pipeline to distribute and parse log lines in parallel.
- **Streaming I/O:** Streams logs without keeping them in memory, allowing for massive files to be processed.
- **Archive Support:** Transparently handles `.gz` and `.zip` archives.
- **Auto-detection:** Automatically detects common log formats (JSON, Apache, Nginx, Syslog, etc.).
- **Filtering & Searching:** Apply predicate filters (regex, level, time, exact match) concurrently.
- **Live Tailing:** Tail and parse logs as they are written in real-time.
- **Extensible:** Easily define and inject custom log parsers into the pipeline.

## Installation

### As a CLI Tool

To build the CLI application from source:

```bash
git clone https://github.com/yourusername/goLogParser.git
cd goLogParser
go build -o gologparser ./cmd/goLogParser
```

### As a Go Library

```bash
go get github.com/yourusername/goLogParser/pkg/goLogParser
```

*(Note: Replace `yourusername/goLogParser` with the actual module path once published)*

## CLI Usage

The CLI supports several commands to process logs.

### Parse and Export

Parse a log file and output it as structured JSON:
```bash
./gologparser parse /path/to/app.log --output json
```

Output as CSV:
```bash
./gologparser parse /path/to/app.log --output csv
```

### Filtering

Filter logs by log level and time range:
```bash
./gologparser filter /path/to/app.log --level ERROR --time-start "2023-01-01T00:00:00Z"
```

Search for a specific regular expression in the message:
```bash
./gologparser search /path/to/app.log --pattern "timeout"
```

### Statistics

Get parsing statistics (lines read, parsed, errors, duration) instead of the log records themselves:
```bash
./gologparser stats /path/to/app.log
```

### Live Tailing

Tail a log file in real-time (similar to `tail -f`):
```bash
./gologparser tail /path/to/app.log
```

## Library Usage

To use `goLogParser` in your own Go application, you can configure a pipeline with the options provided by the package:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/yourusername/goLogParser/pkg/goLogParser"
)

func main() {
    // 1. Initialize the parser
    parser := goLogParser.New(
        goLogParser.WithWorkers(4),
        goLogParser.WithFile("application.log"),
    )

    // 2. Define a callback to process parsed records
    callback := func(record goLogParser.Record) {
        if record.Level == "ERROR" {
            fmt.Printf("Found Error: %s\n", record.Message)
        }
    }

    // 3. Run the parser
    ctx := context.Background()
    stats, err := parser.Run(ctx, callback)
    if err != nil {
        log.Fatalf("Parse failed: %v", err)
    }

    fmt.Printf("Parsed %d lines in %s\n", stats.LinesRead, stats.Duration)
}
```

## Supported Log Formats (Out of the box)
- `json` & `ndjson`
- `apache_access`, `apache_error`
- `nginx_access`, `nginx_error`
- `clf` (Common Log Format)
- `combined` (Combined Log Format)
- `syslog` (RFC3164)
- `plaintext` (Fallback)

## License

MIT License.
