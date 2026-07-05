// Package reader provides streaming io.ReadCloser sources for log data.
// It transparently handles plain files, gzip-compressed files, zip archives,
// directories (recursive), and stdin.
package reader

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Source wraps a named io.ReadCloser so callers can associate the stream with
// a file path or description for error reporting.
type Source struct {
	// Name is the human-readable origin (file path, "stdin", archive entry, …).
	Name string
	// RC is the open readable stream. The caller must close it when done.
	RC io.ReadCloser
}

// Options configures directory traversal behaviour.
type Options struct {
	// Recursive enables recursive directory traversal.
	Recursive bool
	// FilePattern is a glob pattern matched against base file names.
	// Defaults to "*" when empty.
	FilePattern string
	// FollowSymlinks enables following symbolic links.
	FollowSymlinks bool
}

// OpenFile returns a single Source for the given path.
// It transparently decompresses .gz files and expands .zip archives into
// multiple Sources.
func OpenFile(path string) ([]Source, error) {
	switch {
	case strings.HasSuffix(path, ".gz"):
		return openGzip(path)
	case strings.HasSuffix(path, ".zip"):
		return openZip(path)
	default:
		return openPlain(path)
	}
}

// OpenDir returns Sources for all matching files under dir.
// It respects the Recursive and FilePattern options.
func OpenDir(ctx context.Context, dir string, opts Options) ([]Source, error) {
	if opts.FilePattern == "" {
		opts.FilePattern = "*"
	}

	var sources []Source

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if !opts.Recursive && path != dir {
				return fs.SkipDir
			}
			return nil
		}
		if !opts.FollowSymlinks && d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		matched, err := filepath.Match(opts.FilePattern, filepath.Base(path))
		if err != nil || !matched {
			return err
		}
		srcs, err := OpenFile(path)
		if err != nil {
			// Non-fatal — skip unreadable files.
			return nil
		}
		sources = append(sources, srcs...)
		return nil
	}

	if err := filepath.WalkDir(dir, walkFn); err != nil {
		return nil, fmt.Errorf("walking directory %q: %w", dir, err)
	}
	return sources, nil
}

// Stdin returns a Source wrapping os.Stdin.
func Stdin() Source {
	return Source{Name: "stdin", RC: io.NopCloser(os.Stdin)}
}

// ─── internal helpers ─────────────────────────────────────────────────────────

func openPlain(path string) ([]Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	return []Source{{Name: path, RC: f}}, nil
}

func openGzip(path string) ([]Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", path, err)
	}
	gr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("creating gzip reader for %q: %w", path, err)
	}
	// Wrap so that closing gr also closes f.
	return []Source{{Name: path, RC: &gzipCloser{gr: gr, f: f}}}, nil
}

// gzipCloser closes both the gzip reader and the underlying file.
type gzipCloser struct {
	gr *gzip.Reader
	f  *os.File
}

func (gc *gzipCloser) Read(p []byte) (int, error) { return gc.gr.Read(p) }
func (gc *gzipCloser) Close() error {
	grErr := gc.gr.Close()
	fErr := gc.f.Close()
	if grErr != nil {
		return grErr
	}
	return fErr
}

func openZip(path string) ([]Source, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("opening zip %q: %w", path, err)
	}

	var sources []Source
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		name := path + "!/" + zf.Name
		// Wrap rc so Close() also closes the zip reader when this is the last entry.
		sources = append(sources, Source{Name: name, RC: rc})
	}

	// The zip.ReadCloser must remain open while any entry RC is in use.
	// We close zr only when all RCs have been read; a simple approach is to
	// attach a finaliser-style wrapper on the last entry. For simplicity we
	// defer close of zr by wrapping the last source.
	if len(sources) > 0 {
		last := sources[len(sources)-1]
		sources[len(sources)-1] = Source{
			Name: last.Name,
			RC:   &zipFinalCloser{rc: last.RC, zr: zr},
		}
	} else {
		zr.Close()
	}

	return sources, nil
}

// zipFinalCloser closes both the entry reader and the zip archive.
type zipFinalCloser struct {
	rc io.ReadCloser
	zr *zip.ReadCloser
}

func (z *zipFinalCloser) Read(p []byte) (int, error) {
	return z.rc.Read(p)
}

func (z *zipFinalCloser) Close() error {
	rcErr := z.rc.Close()
	zrErr := z.zr.Close()
	if rcErr != nil {
		return rcErr
	}
	return zrErr
}
