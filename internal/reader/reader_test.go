package reader_test

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arthurgray2k/goLogParser/internal/reader"
)

func TestReaderPlainAndCompressed(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "glp-reader-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Plain file
	plainPath := filepath.Join(tempDir, "app.log")
	plainContent := []byte("plain line 1\nplain line 2\n")
	if err := os.WriteFile(plainPath, plainContent, 0644); err != nil {
		t.Fatalf("writing plain file: %v", err)
	}

	srcs, err := reader.OpenFile(plainPath)
	if err != nil {
		t.Fatalf("OpenFile plain: %v", err)
	}
	if len(srcs) != 1 {
		t.Fatalf("expected 1 source, got %d", len(srcs))
	}
	defer srcs[0].RC.Close()
	data, _ := io.ReadAll(srcs[0].RC)
	if !bytes.Equal(data, plainContent) {
		t.Errorf("plain content mismatch: expected %q, got %q", plainContent, data)
	}

	// 2. Gzip file
	gzPath := filepath.Join(tempDir, "app.log.gz")
	gzFile, err := os.Create(gzPath)
	if err != nil {
		t.Fatalf("creating gz file: %v", err)
	}
	gw := gzip.NewWriter(gzFile)
	gzContent := []byte("gzip line 1\ngzip line 2\n")
	gw.Write(gzContent)
	gw.Close()
	gzFile.Close()

	srcs, err = reader.OpenFile(gzPath)
	if err != nil {
		t.Fatalf("OpenFile gzip: %v", err)
	}
	if len(srcs) != 1 {
		t.Fatalf("expected 1 source, got %d", len(srcs))
	}
	defer srcs[0].RC.Close()
	data, _ = io.ReadAll(srcs[0].RC)
	if !bytes.Equal(data, gzContent) {
		t.Errorf("gzip content mismatch: expected %q, got %q", gzContent, data)
	}

	// 3. Zip file
	zipPath := filepath.Join(tempDir, "logs.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("creating zip file: %v", err)
	}
	zw := zip.NewWriter(zipFile)
	
	f1, _ := zw.Create("f1.log")
	f1.Write([]byte("zip file 1 content"))
	f2, _ := zw.Create("sub/f2.log")
	f2.Write([]byte("zip file 2 content"))
	
	zw.Close()
	zipFile.Close()

	srcs, err = reader.OpenFile(zipPath)
	if err != nil {
		t.Fatalf("OpenFile zip: %v", err)
	}
	if len(srcs) != 2 {
		t.Fatalf("expected 2 sources in zip, got %d", len(srcs))
	}
	// Verify and close all
	var names []string
	for _, src := range srcs {
		names = append(names, src.Name)
		content, _ := io.ReadAll(src.RC)
		src.RC.Close()
		if strings.Contains(src.Name, "f1.log") {
			if string(content) != "zip file 1 content" {
				t.Errorf("f1.log content mismatch: %s", content)
			}
		} else if strings.Contains(src.Name, "sub/f2.log") {
			if string(content) != "zip file 2 content" {
				t.Errorf("f2.log content mismatch: %s", content)
			}
		}
	}
}

func TestOpenDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "glp-dir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.WriteFile(filepath.Join(tempDir, "a.log"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tempDir, "b.txt"), []byte("b"), 0644)
	os.Mkdir(filepath.Join(tempDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tempDir, "sub", "c.log"), []byte("c"), 0644)

	// Walk non-recursive with pattern *.log
	srcs, err := reader.OpenDir(context.Background(), tempDir, reader.Options{
		Recursive:   false,
		FilePattern: "*.log",
	})
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	if len(srcs) != 1 {
		t.Errorf("expected 1 file (a.log) in non-recursive scan, got %d", len(srcs))
	}
	for _, s := range srcs {
		s.RC.Close()
	}

	// Walk recursive with pattern *.log
	srcs, err = reader.OpenDir(context.Background(), tempDir, reader.Options{
		Recursive:   true,
		FilePattern: "*.log",
	})
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	if len(srcs) != 2 {
		t.Errorf("expected 2 files (a.log, c.log) in recursive scan, got %d", len(srcs))
	}
	for _, s := range srcs {
		s.RC.Close()
	}
}


