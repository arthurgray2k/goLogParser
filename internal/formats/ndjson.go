package formats

import (
	"encoding/json"
	"strings"
)

// NDJSONParser handles Newline-Delimited JSON (NDJSON / JSON Lines).
// Each line is an independent JSON object; the overall stream is not a JSON
// array. It intentionally reuses JSONParser's per-line logic.
type NDJSONParser struct {
	inner JSONParser
}

func init() { RegisterParser(&NDJSONParser{}) }

// Name returns "ndjson".
func (p *NDJSONParser) Name() string { return "ndjson" }

// CanParse returns a confidence score similar to JSONParser but slightly
// lower so that pure JSON files are preferred for single-object detection.
func (p *NDJSONParser) CanParse(sample string) float64 {
	s := strings.TrimSpace(sample)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return 0
	}
	var probe map[string]any
	if json.Unmarshal([]byte(s), &probe) != nil {
		return 0
	}
	// NDJSON and JSON look identical at the line level; prefer JSON when it
	// contains a strong log-level indicator.
	for _, key := range []string{"level", "severity", "lvl", "log_level"} {
		if _, ok := probe[key]; ok {
			return 0.95
		}
	}
	return 0.55
}

// ParseLine delegates to the JSON parser — the per-line format is identical.
func (p *NDJSONParser) ParseLine(line string) Entry {
	return p.inner.ParseLine(line)
}
