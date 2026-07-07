// Package output handles writing model responses to stdout and an optional
// output file. It also owns the line writer used by both the streaming
// chat path and the structured-output path, so both can split text into
// complete lines as they arrive (matching the JS createLineWriter behavior)
// and emit dim ANSI codes to stdout only — never to the file.
package output

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LineWriter buffers text and emits it one line at a time. Complete lines
// (terminated by '\n') are flushed immediately; the trailing partial line
// is flushed on End(). When dim is true, stdout output is wrapped in ANSI
// dim escape sequences (\x1b[2m ... \x1b[0m); the file stream always
// receives the plain text.
type LineWriter struct {
	out    io.Writer
	file   io.Writer
	dim    bool
	buffer strings.Builder
}

func NewLineWriter(file io.Writer, opts ...LineOption) *LineWriter {
	w := &LineWriter{out: os.Stdout, file: file}
	for _, o := range opts {
		o(w)
	}
	return w
}

type LineOption func(*LineWriter)

func WithDim(v bool) LineOption      { return func(w *LineWriter) { w.dim = v } }
func WithStdout(v io.Writer) LineOption { return func(w *LineWriter) { w.out = v } }

func (w *LineWriter) Write(text string) {
	if text == "" {
		return
	}
	w.buffer.WriteString(text)
	for {
		s := w.buffer.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			return
		}
		line := s[:idx]
		w.buffer.Reset()
		w.buffer.WriteString(s[idx+1:])
		w.flushLine(line)
	}
}

func (w *LineWriter) End() {
	if w.buffer.Len() == 0 {
		return
	}
	rest := w.buffer.String()
	w.buffer.Reset()
	w.write(rest)
}

func (w *LineWriter) flushLine(line string) {
	w.write(line + "\n")
}

func (w *LineWriter) write(text string) {
	// When the durable file destination and the terminal mirror point at
	// the same writer (e.g. no -o flag, so the file IS stdout and the
	// default out is also stdout), collapse to a single write. Without
	// this, every line is printed twice.
	if w.file == w.out {
		if w.out == nil {
			return
		}
		if w.dim {
			_, _ = io.WriteString(w.out, "\x1b[2m"+text+"\x1b[0m")
		} else {
			_, _ = io.WriteString(w.out, text)
		}
		return
	}
	if w.file != nil {
		_, _ = io.WriteString(w.file, text)
	}
	if w.dim {
		_, _ = io.WriteString(w.out, "\x1b[2m"+text+"\x1b[0m")
	} else {
		_, _ = io.WriteString(w.out, text)
	}
}

// ResolveOutputPath implements the CLI > header precedence for the -o
// / --output flag and the `output:` frontmatter key. Returns nil when
// neither is set. Relative paths are resolved against the current working
// directory (matching the JS implementation).
func ResolveOutputPath(cliValue string, header map[string]any) (string, error) {
	var raw string
	switch {
	case cliValue != "":
		raw = cliValue
	case header != nil:
		if v, ok := header["output"]; ok && v != nil && v != "" {
			s, ok := v.(string)
			if !ok {
				return "", fmt.Errorf("header \"output\" must be a string path, got %T", v)
			}
			raw = s
		}
	}
	if raw == "" {
		return "", nil
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	return abs, nil
}

// EnsureDir creates the parent directory of path if it doesn't already
// exist (mirrors fs.mkdirSync({ recursive: true }) in the JS code).
func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

// OpenOutputStream creates (or truncates) the file at path, ensuring its
// parent directory exists. The caller is responsible for closing it.
func OpenOutputStream(path string) (*os.File, error) {
	if err := EnsureDir(path); err != nil {
		return nil, err
	}
	return os.Create(path)
}

// WriteResponse writes text to stdout and (if outputPath is non-empty) to
// the file at that path, overwriting any existing file. Mirrors the JS
// writeResponse helper. The output file receives only the plain text
// (no ANSI codes).
func WriteResponse(text, outputPath string) error {
	if outputPath != "" {
		if err := EnsureDir(outputPath); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, []byte(text), 0o644); err != nil {
			return err
		}
	}
	_, err := io.WriteString(os.Stdout, text)
	return err
}