// Package scanner provides a streaming, memory-efficient line scanner that
// reads from an io.Reader and emits lines through a channel. It is safe for
// use with files larger than available RAM.
package scanner

import (
	"bufio"
	"context"
	"io"
)

const defaultBufSize = 1 << 20 // 1 MiB — handles very long log lines

// Line represents a single log line together with its 1-based line number and
// the name of the source it came from.
type Line struct {
	// Source is the name of the originating file or stream.
	Source string
	// Number is the 1-based line number within Source.
	Number int64
	// Text is the raw line content (without the trailing newline).
	Text string
}

// Scan reads lines from r and sends them on the returned channel.
// The channel is closed when r is exhausted or ctx is cancelled.
// The caller must drain the channel to avoid goroutine leaks.
func Scan(ctx context.Context, r io.Reader, sourceName string) <-chan Line {
	ch := make(chan Line, 512)

	go func() {
		defer close(ch)

		br := bufio.NewReaderSize(r, defaultBufSize)
		var lineNum int64

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			text, err := br.ReadString('\n')
			if len(text) > 0 {
				// Trim the newline characters so parsers receive clean text.
				if len(text) > 0 && text[len(text)-1] == '\n' {
					text = text[:len(text)-1]
				}
				if len(text) > 0 && text[len(text)-1] == '\r' {
					text = text[:len(text)-1]
				}

				lineNum++
				select {
				case <-ctx.Done():
					return
				case ch <- Line{Source: sourceName, Number: lineNum, Text: text}:
				}
			}

			if err != nil {
				if err != io.EOF {
					// Non-EOF errors: the channel is simply closed; the caller
					// will notice missing data.
					_ = err
				}
				return
			}
		}
	}()

	return ch
}
