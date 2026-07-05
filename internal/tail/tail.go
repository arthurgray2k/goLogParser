// Package tail implements live log following similar to `tail -f`.
// It polls for new file content using a configurable interval and emits new
// lines through a channel. It is safe to cancel via context.
package tail

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/filters"
	"github.com/arthurgray2k/goLogParser/internal/formats"
)

const defaultPollInterval = 250 * time.Millisecond

// Line carries a parsed entry emitted during a tail session.
type Line struct {
	Entry  formats.Entry
	Source string
}

// Follow opens path and streams new log entries as they are appended.
// It uses the parsers and filters defined by cfg and emits accepted entries
// on the returned channel, which is closed when ctx is cancelled.
func Follow(ctx context.Context, path string, cfg config.TailConfig, logger *slog.Logger) (<-chan Line, error) {
	if logger == nil {
		logger = slog.Default()
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Seek to the end of the file so we only see new lines.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		return nil, err
	}

	filter, err := filters.FromConfig(cfg.FilterConfig)
	if err != nil {
		f.Close()
		return nil, err
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	ch := make(chan Line, 256)

	go func() {
		defer close(ch)
		defer f.Close()

		var fmtParser formats.Parser

		br := bufio.NewReader(f)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for {
					text, err := br.ReadString('\n')
					if len(text) > 0 {
						// Trim newline.
						if len(text) > 0 && text[len(text)-1] == '\n' {
							text = text[:len(text)-1]
						}
						if len(text) > 0 && text[len(text)-1] == '\r' {
							text = text[:len(text)-1]
						}

						if fmtParser == nil {
							fmtParser = formats.Detect(text)
							logger.Debug("tail: auto-detected format",
								"format", fmtParser.Name(), "source", path)
						}

						entry := fmtParser.ParseLine(text)

						if filter.Match(entry) {
							select {
							case ch <- Line{Entry: entry, Source: path}:
							case <-ctx.Done():
								return
							}
						}
					}
					if err != nil {
						// io.EOF means we've caught up — wait for the next poll.
						break
					}
				}
			}
		}
	}()

	return ch, nil
}
