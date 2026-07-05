// Package cli wires up the Cobra command tree. It is the sole consumer of the
// public goLogParser library and must contain no business logic of its own.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/arthurgray2k/goLogParser/internal/config"
	"github.com/arthurgray2k/goLogParser/internal/exporter"
	glp "github.com/arthurgray2k/goLogParser/pkg/goLogParser"
)

// Execute builds and runs the root command. It returns an exit code.
func Execute() int {
	root := buildRoot()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// ─── Root ─────────────────────────────────────────────────────────────────────

func buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "goLogParser",
		Short: "A fast, concurrent log parsing framework",
		Long: `goLogParser — high-performance concurrent log parsing framework.

Supports plain text, JSON, NDJSON, Apache, Nginx, syslog and more.
Stream large log files without loading them into memory.`,
		SilenceUsage: true,
	}

	// Persistent flags shared by all sub-commands.
	pf := root.PersistentFlags()
	pf.StringP("format", "f", "auto", "log format (auto|text|json|ndjson|apache-access|apache-error|nginx-access|nginx-error|clf|combined|syslog)")
	pf.IntP("workers", "w", 0, "number of parser workers (default: NumCPU)")
	pf.Bool("no-color", false, "disable ANSI colour output")

	viper.BindPFlag("format", pf.Lookup("format"))   //nolint:errcheck
	viper.BindPFlag("workers", pf.Lookup("workers")) //nolint:errcheck

	root.AddCommand(
		buildParse(),
		buildStats(),
		buildSearch(),
		buildFilter(),
		buildExport(),
		buildTail(),
		buildVersion(),
	)

	return root
}

// ─── parse ────────────────────────────────────────────────────────────────────

func buildParse() *cobra.Command {
	var outputFmt string

	cmd := &cobra.Command{
		Use:   "parse [file ...]",
		Short: "Parse one or more log files and display entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := signalContext()
			p := glp.New()

			pCfg, fCfg := parseCoreFlags(cmd)
			eCfg := config.ExportConfig{
				Format:  config.ExportFormat(outputFmt),
				NoColor: mustBool(cmd, "no-color"),
			}

			exp := exporter.New(os.Stdout, eCfg)

			for _, path := range args {
				result, err := p.ParseFile(ctx, path, pCfg, fCfg)
				if err != nil {
					return fmt.Errorf("parsing %q: %w", path, err)
				}
				for _, e := range result.Entries {
					if err := exp.WriteEntry(e); err != nil {
						return err
					}
				}
			}
			return exp.Flush()
		},
	}

	cmd.Flags().StringVarP(&outputFmt, "output", "o", "console", "output format (console|json|ndjson|csv)")
	addFilterFlags(cmd)
	return cmd
}

// ─── stats ────────────────────────────────────────────────────────────────────

func buildStats() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats [file ...]",
		Short: "Show statistics for one or more log files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := signalContext()
			p := glp.New()
			pCfg, fCfg := parseCoreFlags(cmd)
			noColor := mustBool(cmd, "no-color")

			for _, path := range args {
				result, err := p.ParseFile(ctx, path, pCfg, fCfg)
				if err != nil {
					return fmt.Errorf("parsing %q: %w", path, err)
				}
				fmt.Fprintf(os.Stdout, "\n  File: %s\n", path)
				if err := exporter.WriteStats(os.Stdout, result.Stats, noColor); err != nil {
					return err
				}
			}
			return nil
		},
	}

	addFilterFlags(cmd)
	return cmd
}

// ─── search ───────────────────────────────────────────────────────────────────

func buildSearch() *cobra.Command {
	var ()

	cmd := &cobra.Command{
		Use:   "search [file ...]",
		Short: "Search log entries by message content",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := signalContext()
			p := glp.New()
			pCfg, fCfg := parseCoreFlags(cmd)

			// Core flags already parse contains, regex, etc.

			eCfg := config.ExportConfig{Format: config.ExportConsole, NoColor: mustBool(cmd, "no-color")}
			exp := exporter.New(os.Stdout, eCfg)

			for _, path := range args {
				result, err := p.ParseFile(ctx, path, pCfg, fCfg)
				if err != nil {
					return fmt.Errorf("parsing %q: %w", path, err)
				}
				for _, e := range result.Entries {
					if err := exp.WriteEntry(e); err != nil {
						return err
					}
				}
			}
			return exp.Flush()
		},
	}

	addFilterFlags(cmd)
	return cmd
}

// ─── filter ───────────────────────────────────────────────────────────────────

func buildFilter() *cobra.Command {
	var outputFmt string

	cmd := &cobra.Command{
		Use:   "filter [file ...]",
		Short: "Filter log entries by level, time, service, host, or regex",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := signalContext()
			p := glp.New()
			pCfg, fCfg := parseCoreFlags(cmd)
			eCfg := config.ExportConfig{
				Format:  config.ExportFormat(outputFmt),
				NoColor: mustBool(cmd, "no-color"),
			}
			exp := exporter.New(os.Stdout, eCfg)

			for _, path := range args {
				result, err := p.ParseFile(ctx, path, pCfg, fCfg)
				if err != nil {
					return fmt.Errorf("parsing %q: %w", path, err)
				}
				for _, e := range result.Entries {
					if err := exp.WriteEntry(e); err != nil {
						return err
					}
				}
			}
			return exp.Flush()
		},
	}

	cmd.Flags().StringVarP(&outputFmt, "output", "o", "console", "output format (console|json|ndjson|csv)")
	addFilterFlags(cmd)
	return cmd
}

// ─── export ───────────────────────────────────────────────────────────────────

func buildExport() *cobra.Command {
	var (
		outputFmt string
		pretty    bool
	)

	cmd := &cobra.Command{
		Use:   "export [file ...]",
		Short: "Export parsed log entries in the requested format",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := signalContext()
			p := glp.New()
			pCfg, fCfg := parseCoreFlags(cmd)
			eCfg := config.ExportConfig{
				Format:  config.ExportFormat(outputFmt),
				Pretty:  pretty,
				NoColor: mustBool(cmd, "no-color"),
			}
			exp := exporter.New(os.Stdout, eCfg)

			for _, path := range args {
				result, err := p.ParseFile(ctx, path, pCfg, fCfg)
				if err != nil {
					return fmt.Errorf("parsing %q: %w", path, err)
				}
				for _, e := range result.Entries {
					if err := exp.WriteEntry(e); err != nil {
						return err
					}
				}
			}
			return exp.Flush()
		},
	}

	cmd.Flags().StringVarP(&outputFmt, "format", "F", "json", "export format (json|ndjson|csv|console)")
	cmd.Flags().BoolVar(&pretty, "pretty", false, "pretty-print JSON output")
	addFilterFlags(cmd)
	return cmd
}

// ─── tail ─────────────────────────────────────────────────────────────────────

func buildTail() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail [file]",
		Short: "Follow a log file in real time",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := signalContext()
			p := glp.New()
			_, fCfg := parseCoreFlags(cmd)

			tailCfg := glp.TailConfig{
				FilterConfig: fCfg,
			}

			eCfg := config.ExportConfig{Format: config.ExportConsole, NoColor: mustBool(cmd, "no-color")}
			exp := exporter.New(os.Stdout, eCfg)

			ch, err := p.Tail(ctx, args[0], tailCfg)
			if err != nil {
				return err
			}

			for line := range ch {
				if err := exp.WriteEntry(line.Entry); err != nil {
					return err
				}
			}
			return nil
		},
	}

	addFilterFlags(cmd)
	return cmd
}

// ─── version ──────────────────────────────────────────────────────────────────

var Version = "0.1.0"

func buildVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(os.Stdout, "goLogParser %s\n", Version)
		},
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// addFilterFlags attaches common filter flags to cmd.
func addFilterFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringArray("level", nil, "filter by log level (repeatable, e.g. --level ERROR)")
	f.StringArray("contains", nil, "message must contain substring (repeatable)")
	f.StringArray("not-contains", nil, "message must NOT contain substring (repeatable)")
	f.StringArray("service", nil, "filter by service name (repeatable)")
	f.StringArray("host", nil, "filter by hostname (repeatable)")
	f.String("after", "", "only entries after this timestamp (RFC3339)")
	f.String("before", "", "only entries before this timestamp (RFC3339)")
	f.String("starts-with", "", "message must start with this prefix")
	f.String("ends-with", "", "message must end with this suffix")
	f.String("regex", "", "message must match this regular expression")
}

// parseCoreFlags extracts ParseConfig and FilterConfig from cmd flags.
func parseCoreFlags(cmd *cobra.Command) (config.ParseConfig, config.FilterConfig) {
	fmtStr, _ := cmd.Flags().GetString("format")
	if fmtStr == "" {
		fmtStr = viper.GetString("format")
	}
	workers, _ := cmd.Flags().GetInt("workers")
	if workers == 0 {
		workers = viper.GetInt("workers")
	}

	pCfg := config.ParseConfig{
		Format:  config.Format(fmtStr),
		Workers: workers,
	}

	var fCfg config.FilterConfig

	if levels, err := cmd.Flags().GetStringArray("level"); err == nil {
		for _, l := range levels {
			fCfg.Levels = append(fCfg.Levels, config.Level(l))
		}
	}
	fCfg.Contains, _ = cmd.Flags().GetStringArray("contains")
	fCfg.NotContains, _ = cmd.Flags().GetStringArray("not-contains")
	fCfg.Services, _ = cmd.Flags().GetStringArray("service")
	fCfg.Hosts, _ = cmd.Flags().GetStringArray("host")
	fCfg.StartsWith, _ = cmd.Flags().GetString("starts-with")
	fCfg.EndsWith, _ = cmd.Flags().GetString("ends-with")
	fCfg.Regex, _ = cmd.Flags().GetString("regex")


	if after, _ := cmd.Flags().GetString("after"); after != "" {
		if t, err := parseTimeArg(after); err == nil {
			fCfg.After = &t
		}
	}
	if before, _ := cmd.Flags().GetString("before"); before != "" {
		if t, err := parseTimeArg(before); err == nil {
			fCfg.Before = &t
		}
	}


	return pCfg, fCfg
}

func parseTimeArg(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}

func mustBool(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	if !v {
		v, _ = cmd.Root().PersistentFlags().GetBool(name)
	}
	return v
}

// signalContext returns a context that is cancelled on SIGINT / SIGTERM.
func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()
	return ctx
}

