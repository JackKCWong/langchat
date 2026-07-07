package output

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOutputPathEmpty(t *testing.T) {
	cases := []struct {
		name   string
		cli    string
		header map[string]any
	}{
		{"nothing", "", nil},
		{"empty header", "", map[string]any{}},
		{"empty header value", "", map[string]any{"output": ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveOutputPath(c.cli, c.header)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != "" {
				t.Errorf("got %q, want empty", got)
			}
		})
	}
}

func TestResolveOutputPathCLIWinsOverHeader(t *testing.T) {
	got, err := ResolveOutputPath("cli.md", map[string]any{"output": "hdr.md"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want, _ := filepath.Abs("cli.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveOutputPathHeaderFallback(t *testing.T) {
	got, err := ResolveOutputPath("", map[string]any{"output": "hdr.md"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want, _ := filepath.Abs("hdr.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveOutputPathResolvesRelative(t *testing.T) {
	got, err := ResolveOutputPath("out/reply.md", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want, _ := filepath.Abs("out/reply.md")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveOutputPathRejectsNonStringHeader(t *testing.T) {
	_, err := ResolveOutputPath("", map[string]any{"output": 42})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), `header "output" must be a string path`) {
		t.Errorf("err = %v", err)
	}
}

func TestWriteResponseWritesFileAndStdout(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "out.md")
	var stdout bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = stdout.ReadFrom(r)
		close(done)
	}()
	if err := WriteResponse("hello world\n", file); err != nil {
		t.Fatalf("err: %v", err)
	}
	w.Close()
	os.Stdout = old
	<-done

	if stdout.String() != "hello world\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("readfile: %v", err)
	}
	if string(body) != "hello world\n" {
		t.Errorf("file = %q", body)
	}
}

func TestWriteResponseNoFileOnlyStdout(t *testing.T) {
	var stdout bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = stdout.ReadFrom(r)
		close(done)
	}()
	if err := WriteResponse("just stdout\n", ""); err != nil {
		t.Fatalf("err: %v", err)
	}
	w.Close()
	os.Stdout = old
	<-done
	if stdout.String() != "just stdout\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestWriteResponseCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "nested", "deep", "out.md")
	if err := WriteResponse("nested\n", file); err != nil {
		t.Fatalf("err: %v", err)
	}
	body, _ := os.ReadFile(file)
	if string(body) != "nested\n" {
		t.Errorf("file = %q", body)
	}
}

func TestWriteResponseOverwrites(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "out.md")
	_ = os.WriteFile(file, []byte("old content"), 0o644)
	if err := WriteResponse("new content\n", file); err != nil {
		t.Fatalf("err: %v", err)
	}
	body, _ := os.ReadFile(file)
	if string(body) != "new content\n" {
		t.Errorf("file = %q", body)
	}
}

func TestLineWriterStdoutOnly(t *testing.T) {
	var stdout, file bytes.Buffer
	w := NewLineWriter(nil, WithStdout(&stdout))
	w.Write("line one\nline two")
	w.Write(" continued\n")
	w.End()
	if stdout.String() != "line one\nline two continued\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	_ = file
}

func TestLineWriterFileAndStdout(t *testing.T) {
	var stdout, file bytes.Buffer
	w := NewLineWriter(&file, WithStdout(&stdout))
	w.Write("first line\n")
	w.Write("partial ")
	w.Write("rest\n")
	w.End()
	if stdout.String() != "first line\npartial rest\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	if file.String() != "first line\npartial rest\n" {
		t.Errorf("file = %q", file.String())
	}
}

func TestLineWriterDim(t *testing.T) {
	var stdout, file bytes.Buffer
	w := NewLineWriter(&file, WithStdout(&stdout), WithDim(true))
	w.Write("reasoning here\n")
	w.End()
	want := "\x1b[2mreasoning here\n\x1b[0m"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	if file.String() != "reasoning here\n" {
		t.Errorf("file should be plain: %q", file.String())
	}
	if strings.Contains(file.String(), "\x1b[") {
		t.Errorf("file should not contain ANSI: %q", file.String())
	}
}

// TestLineWriterFileSameAsOutDoesNotDoublePrint guards against the
// regression where the main chat path passes os.Stdout as both the file
// destination (because no -o flag was given) and the default out — without
// the dedup, every line was printed twice on the terminal.
func TestLineWriterFileSameAsOutDoesNotDoublePrint(t *testing.T) {
	var stdout bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = stdout.ReadFrom(r)
		close(done)
	}()

	lw := NewLineWriter(os.Stdout) // file and out now both point at the pipe
	lw.Write("single line\n")
	lw.End()

	w.Close()
	os.Stdout = old
	<-done

	if got := strings.Count(stdout.String(), "single line"); got != 1 {
		t.Errorf("got %d occurrences of 'single line', want 1: %q", got, stdout.String())
	}
}