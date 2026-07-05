package formats

import (
	"fmt"

	"github.com/arthurgray2k/goLogParser/internal/config"
)

// Detect returns the best-matching Parser for the given sample line.
// It iterates over all registered parsers, scores each one, and returns the
// parser with the highest confidence. A PlainTextParser is returned as the
// unconditional fallback.
func Detect(sample string) Parser {
	var best Parser
	var bestScore float64

	for _, p := range registry {
		score := p.CanParse(sample)
		if score > bestScore {
			bestScore = score
			best = p
		}
	}

	// Anything ≥ 0.5 is a confident-enough match.
	if bestScore >= 0.5 {
		return best
	}

	// Fallback: plain text.
	return &PlainTextParser{}
}

// ForFormat returns the registered Parser that matches the given Format name.
// It returns an error when the format is not recognised.
func ForFormat(f config.Format) (Parser, error) {
	if f == config.FormatAuto || f == "" {
		// Caller must use Detect() with a sample line.
		return nil, fmt.Errorf("auto-detection requires a sample line; use Detect()")
	}

	for _, p := range registry {
		if p.Name() == string(f) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("unknown log format %q", f)
}
