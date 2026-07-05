// Package filters provides composable, context-free predicates for filtering
// parsed log entries. All Filter implementations are safe for concurrent use.
package filters

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/formats"
)

// Filter is a predicate that returns true when an Entry should be included.
type Filter interface {
	Match(e formats.Entry) bool
}

// FilterFunc is a function type that implements Filter.
type FilterFunc func(e formats.Entry) bool

func (f FilterFunc) Match(e formats.Entry) bool { return f(e) }

// ─── Composite ────────────────────────────────────────────────────────────────

// And returns a Filter that matches when all supplied filters match.
func And(filters ...Filter) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		for _, f := range filters {
			if !f.Match(e) {
				return false
			}
		}
		return true
	})
}

// Or returns a Filter that matches when any supplied filter matches.
func Or(filters ...Filter) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		for _, f := range filters {
			if f.Match(e) {
				return true
			}
		}
		return false
	})
}

// Not returns a Filter that inverts f.
func Not(f Filter) Filter {
	return FilterFunc(func(e formats.Entry) bool { return !f.Match(e) })
}

// ─── Level ────────────────────────────────────────────────────────────────────

// Level returns a Filter that matches entries whose Level field equals one of
// the supplied levels (case-insensitive).
func Level(levels ...config.Level) Filter {
	set := make(map[string]struct{}, len(levels))
	for _, l := range levels {
		set[strings.ToUpper(string(l))] = struct{}{}
	}
	return FilterFunc(func(e formats.Entry) bool {
		_, ok := set[strings.ToUpper(e.Level)]
		return ok
	})
}

// ─── Text matching ────────────────────────────────────────────────────────────

// Contains returns a Filter that matches entries whose Message contains all of
// the given substrings (case-sensitive).
func Contains(substrings ...string) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		for _, s := range substrings {
			if !strings.Contains(e.Message, s) {
				return false
			}
		}
		return true
	})
}

// ContainsAny returns a Filter that matches when the Message contains at least
// one of the given substrings.
func ContainsAny(substrings ...string) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		for _, s := range substrings {
			if strings.Contains(e.Message, s) {
				return true
			}
		}
		return false
	})
}

// NotContains returns a Filter that excludes entries whose Message contains any
// of the given substrings.
func NotContains(substrings ...string) Filter {
	return Not(ContainsAny(substrings...))
}

// StartsWith returns a Filter that matches entries whose Message starts with
// the given prefix.
func StartsWith(prefix string) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		return strings.HasPrefix(e.Message, prefix)
	})
}

// EndsWith returns a Filter that matches entries whose Message ends with the
// given suffix.
func EndsWith(suffix string) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		return strings.HasSuffix(e.Message, suffix)
	})
}

// Regex returns a Filter that matches entries whose Message satisfies the
// compiled regular expression.
func Regex(pattern string) (Filter, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return FilterFunc(func(e formats.Entry) bool {
		return re.MatchString(e.Message)
	}), nil
}

// MustRegex is like Regex but panics on invalid patterns. Intended for
// compile-time constants only.
func MustRegex(pattern string) Filter {
	f, err := Regex(pattern)
	if err != nil {
		panic(err)
	}
	return f
}

// ─── Metadata ─────────────────────────────────────────────────────────────────

// Services returns a Filter that matches entries whose Service field equals
// one of the supplied names.
func Services(names ...string) Filter {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	return FilterFunc(func(e formats.Entry) bool {
		_, ok := set[e.Service]
		return ok
	})
}

// Hosts returns a Filter that matches entries whose Host field equals one of
// the supplied hostnames.
func Hosts(names ...string) Filter {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		set[n] = struct{}{}
	}
	return FilterFunc(func(e formats.Entry) bool {
		_, ok := set[e.Host]
		return ok
	})
}

// ─── Time range ───────────────────────────────────────────────────────────────


// ─── Field matching ───────────────────────────────────────────────────────────

// Field returns a Filter that matches entries where Fields[key] equals value.
func Field(key, value string) Filter {
	return FilterFunc(func(e formats.Entry) bool {
		v, ok := e.Fields[key]
		if !ok {
			return false
		}
		return strings.EqualFold(fmt.Sprintf("%v", v), value)
	})
}

// ─── Config-driven builder ────────────────────────────────────────────────────

// FromConfig constructs a single composite Filter from a FilterConfig.
// An entry must satisfy every non-empty criterion.
func FromConfig(cfg config.FilterConfig) (Filter, error) {
	var parts []Filter

	if len(cfg.Levels) > 0 {
		parts = append(parts, Level(cfg.Levels...))
	}
	if len(cfg.Contains) > 0 {
		parts = append(parts, Contains(cfg.Contains...))
	}
	if len(cfg.NotContains) > 0 {
		parts = append(parts, NotContains(cfg.NotContains...))
	}
	if cfg.Regex != "" {
		rf, err := Regex(cfg.Regex)
		if err != nil {
			return nil, err
		}
		parts = append(parts, rf)
	}
	if len(cfg.Services) > 0 {
		parts = append(parts, Services(cfg.Services...))
	}
	if len(cfg.Hosts) > 0 {
		parts = append(parts, Hosts(cfg.Hosts...))
	}
	if cfg.After != nil {
		after := *cfg.After
		parts = append(parts, FilterFunc(func(e formats.Entry) bool {
			return !e.Timestamp.IsZero() && !e.Timestamp.Before(after)
		}))
	}
	if cfg.Before != nil {
		before := *cfg.Before
		parts = append(parts, FilterFunc(func(e formats.Entry) bool {
			return !e.Timestamp.IsZero() && e.Timestamp.Before(before)
		}))
	}
	if cfg.StartsWith != "" {
		parts = append(parts, StartsWith(cfg.StartsWith))
	}
	if cfg.EndsWith != "" {
		parts = append(parts, EndsWith(cfg.EndsWith))
	}
	for k, v := range cfg.Fields {
		parts = append(parts, Field(k, v))
	}

	if len(parts) == 0 {
		// No criteria → accept everything.
		return FilterFunc(func(_ formats.Entry) bool { return true }), nil
	}
	return And(parts...), nil
}
