package parser_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/parser"
)

func TestPipeline_Run(t *testing.T) {
	cfg := config.ParseConfig{
		Workers:   2,
		BatchSize: 10,
		Format:    "auto",
	}
	filterCfg := config.FilterConfig{}

	pipeline, err := parser.New(cfg, filterCfg, nil)
	if err != nil {
		t.Fatalf("unexpected error creating pipeline: %v", err)
	}

	input := `{"level":"INFO","msg":"Test 1"}
{"level":"ERROR","msg":"Test 2"}
invalid line
{"level":"DEBUG","msg":"Test 3"}`

	ctx := context.Background()
	result, err := pipeline.Run(ctx, strings.NewReader(input), "test_source")
	if err != nil {
		t.Fatalf("unexpected error running pipeline: %v", err)
	}

	if result.Stats.TotalLines != 4 {
		t.Errorf("expected 4 total lines, got %d", result.Stats.TotalLines)
	}
	if result.Stats.ValidEntries != 3 {
		t.Errorf("expected 3 valid entries, got %d", result.Stats.ValidEntries)
	}
	if result.Stats.InvalidEntries != 1 {
		t.Errorf("expected 1 invalid entry, got %d", result.Stats.InvalidEntries)
	}
	if len(result.Entries) != 4 {
		t.Errorf("expected 4 filtered entries, got %d", len(result.Entries))
	}
}
