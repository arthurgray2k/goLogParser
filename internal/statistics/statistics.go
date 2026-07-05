// Package statistics computes aggregate metrics over a stream of parsed log
// entries. All public methods are safe for concurrent use via an internal mutex.
package statistics

import (
	"sync"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/formats"
)

// Stats holds aggregate metrics collected during a parse run.
type Stats struct {
	// TotalLines is the total number of lines read (including empty/blank).
	TotalLines int64
	// ValidEntries is the number of lines successfully parsed.
	ValidEntries int64
	// InvalidEntries is the number of lines that could not be parsed.
	InvalidEntries int64

	// Levels maps normalised level strings to occurrence counts.
	Levels map[string]int64
	// Services maps service names to occurrence counts.
	Services map[string]int64
	// Hosts maps hostnames to occurrence counts.
	Hosts map[string]int64

	// EarliestTimestamp is the smallest non-zero Timestamp seen.
	EarliestTimestamp time.Time
	// LatestTimestamp is the largest Timestamp seen.
	LatestTimestamp time.Time

	// ParseDuration is the wall-clock time taken by the parsing pipeline.
	ParseDuration time.Duration
}

// Collector wraps Stats and protects it with a mutex for concurrent collection.
type Collector struct {
	mu    sync.Mutex
	stats Stats
}

// New returns an initialised Stats collector.
func New() *Collector {
	return &Collector{
		stats: Stats{
			Levels:   make(map[string]int64),
			Services: make(map[string]int64),
			Hosts:    make(map[string]int64),
		},
	}
}

// Record updates the statistics with data from a single Entry.
// It is safe to call concurrently from multiple goroutines.
func (c *Collector) Record(e formats.Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stats.TotalLines++

	if e.ParseError != nil {
		c.stats.InvalidEntries++
	} else {
		c.stats.ValidEntries++
	}

	level := e.Level
	if level == "" {
		level = "UNKNOWN"
	}
	c.stats.Levels[level]++

	if e.Service != "" {
		c.stats.Services[e.Service]++
	}
	if e.Host != "" {
		c.stats.Hosts[e.Host]++
	}

	if !e.Timestamp.IsZero() {
		if c.stats.EarliestTimestamp.IsZero() || e.Timestamp.Before(c.stats.EarliestTimestamp) {
			c.stats.EarliestTimestamp = e.Timestamp
		}
		if e.Timestamp.After(c.stats.LatestTimestamp) {
			c.stats.LatestTimestamp = e.Timestamp
		}
	}
}

// SetDuration sets the parse duration on the stats.
func (c *Collector) SetDuration(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.ParseDuration = d
}

// Snapshot returns a point-in-time copy of the current statistics.
func (c *Collector) Snapshot() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := Stats{
		TotalLines:        c.stats.TotalLines,
		ValidEntries:      c.stats.ValidEntries,
		InvalidEntries:    c.stats.InvalidEntries,
		EarliestTimestamp: c.stats.EarliestTimestamp,
		LatestTimestamp:   c.stats.LatestTimestamp,
		ParseDuration:     c.stats.ParseDuration,
		Levels:            make(map[string]int64, len(c.stats.Levels)),
		Services:          make(map[string]int64, len(c.stats.Services)),
		Hosts:             make(map[string]int64, len(c.stats.Hosts)),
	}
	for k, v := range c.stats.Levels {
		snap.Levels[k] = v
	}
	for k, v := range c.stats.Services {
		snap.Services[k] = v
	}
	for k, v := range c.stats.Hosts {
		snap.Hosts[k] = v
	}
	return snap
}

// UniqueServices returns the number of distinct service names seen.
func (c *Collector) UniqueServices() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.stats.Services)
}

// UniqueHosts returns the number of distinct hostnames seen.
func (c *Collector) UniqueHosts() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.stats.Hosts)
}
