// Package parser implements the concurrent worker-pool parsing pipeline.
//
// Pipeline:
//
//	Scanner lines ──► [batch distributor] ──► N parser workers ──► results channel
//
// The number of workers is configurable. Each worker applies a format parser,
// feeds the entry through any registered filters, records statistics, and
// sends accepted entries downstream.
package parser

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/filters"
	"github.com/arthurgray2k/goLogParser/internal/formats"
	"github.com/arthurgray2k/goLogParser/internal/scanner"
	"github.com/arthurgray2k/goLogParser/internal/statistics"
)

const defaultBatchSize = 256

// Result holds the outcome of a complete parse run.
type Result struct {
	// Entries contains all log entries that passed the filter predicate.
	Entries []formats.Entry
	// Stats holds aggregate metrics for the entire run.
	Stats statistics.Stats
	// Errors is a list of non-fatal parse errors encountered.
	Errors []error
}

// Pipeline orchestrates concurrent log parsing.
type Pipeline struct {
	cfg    config.ParseConfig
	filter filters.Filter
	stats  *statistics.Collector
	logger *slog.Logger
}

// New creates a new Pipeline with the given configuration.
// filterCfg may be zero-valued to accept all entries.
func New(cfg config.ParseConfig, filterCfg config.FilterConfig, logger *slog.Logger) (*Pipeline, error) {
	f, err := filters.FromConfig(filterCfg)
	if err != nil {
		return nil, fmt.Errorf("building filter: %w", err)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Pipeline{
		cfg:    cfg,
		filter: f,
		stats:  statistics.New(),
		logger: logger,
	}, nil
}

// Run reads lines from r (named sourceName), parses them concurrently, and
// returns the aggregated Result. It respects ctx cancellation.
func (p *Pipeline) Run(ctx context.Context, r io.Reader, sourceName string) (Result, error) {
	start := time.Now()

	workers := p.cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	batchSize := p.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	// ── Determine the format parser ──────────────────────────────────────────
	var fmtParser formats.Parser
	if p.cfg.Format == config.FormatAuto || p.cfg.Format == "" {
		// Auto-detect from the first non-empty line.
		fmtParser = nil // resolved lazily
	} else {
		var err error
		fmtParser, err = formats.ForFormat(p.cfg.Format)
		if err != nil {
			return Result{}, err
		}
	}

	// ── Channel plumbing ──────────────────────────────────────────────────────
	lineCh := scanner.Scan(ctx, r, sourceName)

	// resultCh carries entries from all workers to the aggregator.
	resultCh := make(chan formats.Entry, workers*batchSize)
	// errCh carries non-fatal errors.
	errCh := make(chan error, 1024)

	var wg sync.WaitGroup

	// ── Parser workers ────────────────────────────────────────────────────────
	// We start a single coordinator goroutine that distributes lines in batches
	// to a fixed worker pool.
	batchCh := make(chan []scanner.Line, workers*2)

	// Launch workers.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.workerLoop(ctx, batchCh, fmtParser, resultCh, errCh)
		}()
	}

	// Launch batch distributor.
	go func() {
		defer close(batchCh)
		batch := make([]scanner.Line, 0, batchSize)

		for line := range lineCh {
			batch = append(batch, line)
			if len(batch) >= batchSize {
				select {
				case <-ctx.Done():
					return
				case batchCh <- batch:
				}
				batch = make([]scanner.Line, 0, batchSize)
			}
		}
		if len(batch) > 0 {
			select {
			case <-ctx.Done():
			case batchCh <- batch:
			}
		}
	}()

	// ── Aggregator ────────────────────────────────────────────────────────────
	var (
		entries []formats.Entry
		errs    []error
		aggDone = make(chan struct{})
	)

	go func() {
		defer close(aggDone)
		for e := range resultCh {
			entries = append(entries, e)
		}
	}()

	go func() {
		for err := range errCh {
			errs = append(errs, err)
		}
	}()

	// Wait for all workers to finish, then close downstream channels.
	wg.Wait()
	close(resultCh)
	close(errCh)
	<-aggDone

	p.stats.SetDuration(time.Since(start))
	snap := p.stats.Snapshot()

	p.logger.Info("parse run complete",
		"source", sourceName,
		"total_lines", snap.TotalLines,
		"valid", snap.ValidEntries,
		"invalid", snap.InvalidEntries,
		"duration", snap.ParseDuration,
	)

	return Result{
		Entries: entries,
		Stats:   snap,
		Errors:  errs,
	}, nil
}

// workerLoop processes batches of lines from batchCh until it is closed or
// ctx is cancelled.
func (p *Pipeline) workerLoop(
	ctx context.Context,
	batchCh <-chan []scanner.Line,
	fmtParser formats.Parser,
	resultCh chan<- formats.Entry,
	errCh chan<- error,
) {
	// Per-worker parser — resolved on first line when auto-detecting.
	localParser := fmtParser

	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-batchCh:
			if !ok {
				return
			}
			for _, line := range batch {
				if line.Text == "" {
					continue
				}

				// Lazy auto-detection using the first non-empty line.
				if localParser == nil {
					localParser = formats.Detect(line.Text)
					p.logger.Debug("auto-detected format",
						"format", localParser.Name(),
						"source", line.Source,
					)
				}

				entry := localParser.ParseLine(line.Text)
				p.stats.Record(entry)

				if entry.ParseError != nil {
					select {
					case errCh <- fmt.Errorf("line %d in %q: %w", line.Number, line.Source, entry.ParseError):
					default:
						// Drop error when buffer is full to avoid blocking.
					}
				}

				if p.filter.Match(entry) {
					select {
					case <-ctx.Done():
						return
					case resultCh <- entry:
					}
				}
			}
		}
	}
}
