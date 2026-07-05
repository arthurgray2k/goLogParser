package statistics_test

import (
	"errors"
	"testing"
	"time"

	"github.com/arthurgray2k/goLogParser/internal/formats"
	"github.com/arthurgray2k/goLogParser/internal/statistics"
)

func TestStatistics(t *testing.T) {
	s := statistics.New()

	t1 := time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 5, 2, 0, 0, 0, time.UTC)

	s.Record(formats.Entry{
		Level:     "INFO",
		Timestamp: t2,
		Service:   "api",
		Host:      "host-1",
	})

	s.Record(formats.Entry{
		Level:     "ERROR",
		Timestamp: t1,
		Service:   "db",
		Host:      "host-2",
	})

	s.Record(formats.Entry{
		Level:      "UNKNOWN",
		ParseError: errors.New("corrupt line"),
	})

	snap := s.Snapshot()

	if snap.TotalLines != 3 {
		t.Errorf("expected 3 total lines, got %d", snap.TotalLines)
	}
	if snap.ValidEntries != 2 {
		t.Errorf("expected 2 valid entries, got %d", snap.ValidEntries)
	}
	if snap.InvalidEntries != 1 {
		t.Errorf("expected 1 invalid entry, got %d", snap.InvalidEntries)
	}

	if snap.Levels["INFO"] != 1 || snap.Levels["ERROR"] != 1 || snap.Levels["UNKNOWN"] != 1 {
		t.Errorf("levels counts mismatch: %+v", snap.Levels)
	}

	if snap.Services["api"] != 1 || snap.Services["db"] != 1 {
		t.Errorf("services counts mismatch: %+v", snap.Services)
	}

	if s.UniqueServices() != 2 {
		t.Errorf("expected 2 unique services, got %d", s.UniqueServices())
	}

	if s.UniqueHosts() != 2 {
		t.Errorf("expected 2 unique hosts, got %d", s.UniqueHosts())
	}

	if !snap.EarliestTimestamp.Equal(t1) {
		t.Errorf("expected earliest timestamp %v, got %v", t1, snap.EarliestTimestamp)
	}

	if !snap.LatestTimestamp.Equal(t2) {
		t.Errorf("expected latest timestamp %v, got %v", t2, snap.LatestTimestamp)
	}
}
