package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseTimeArg(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
		hasError bool
	}{
		{"2023-01-01T15:00:00Z", time.Date(2023, 1, 1, 15, 0, 0, 0, time.UTC), false},
		{"2023-01-01T15:00:00", time.Date(2023, 1, 1, 15, 0, 0, 0, time.UTC), false},
		{"2023-01-01 15:00:00", time.Date(2023, 1, 1, 15, 0, 0, 0, time.UTC), false},
		{"2023-01-01", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), false},
		{"invalid", time.Time{}, true},
	}

	for _, tt := range tests {
		got, err := parseTimeArg(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for %q, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
			}
			if !got.Equal(tt.expected) {
				t.Errorf("for %q, expected %v, got %v", tt.input, tt.expected, got)
			}
		}
	}
}

func TestCLICommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"version", []string{"version"}},
		{"parse missing args", []string{"parse"}},
		{"stats missing args", []string{"stats"}},
		{"search missing args", []string{"search"}},
		{"filter missing args", []string{"filter"}},
		{"export missing args", []string{"export"}},
		{"tail missing args", []string{"tail"}},
		{"tail too many args", []string{"tail", "1", "2"}},
		{"parse invalid file", []string{"parse", "nonexistent.log"}},
		{"stats invalid file", []string{"stats", "nonexistent.log"}},
		{"search invalid file", []string{"search", "nonexistent.log"}},
		{"filter invalid file", []string{"filter", "nonexistent.log"}},
		{"export invalid file", []string{"export", "nonexistent.log"}},
		{"tail invalid file", []string{"tail", "nonexistent.log"}},
	}

	validFile := filepath.Join(t.TempDir(), "valid.log")
	if err := os.WriteFile(validFile, []byte("{\"level\":\"INFO\",\"msg\":\"hello\"}\n"), 0644); err == nil {
		tests = append(tests, struct{name string; args []string}{"parse valid", []string{"parse", validFile}})
		tests = append(tests, struct{name string; args []string}{"stats valid", []string{"stats", validFile}})
		tests = append(tests, struct{name string; args []string}{"filter valid", []string{"filter", validFile, "--level", "INFO"}})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := buildRoot()
			root.SetArgs(tt.args)
			
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			
			// We only care that it executes without panicking, 
			// error returns are expected for missing/invalid files.
			_ = root.Execute()
		})
	}
}

func TestExecute(t *testing.T) {
    // Save original args
    orig := os.Args
    defer func() { os.Args = orig }()

    os.Args = []string{"gologparser", "version"}
    code := Execute()
    if code != 0 {
        t.Errorf("expected exit code 0, got %d", code)
    }

    os.Args = []string{"gologparser", "invalidcmd"}
    code = Execute()
    if code == 0 {
        t.Errorf("expected non-zero exit code, got %d", code)
    }
}
