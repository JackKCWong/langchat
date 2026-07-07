package includes

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JackKCWong/langchat-go/internal/parser"
)

var (
	DefaultImageExtensions = map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
	}
	DefaultMaxBytes = 5 * 1024 * 1024
)

// Result is the output of Resolve.
type Result struct {
	Text        string
	Attachments []parser.Attachment
}

// Options tunes the include resolver.
type Options struct {
	BaseDir       string                  // absolute path; resolved against cwd if empty
	MaxDepth      int                     // default 8
	AllowEscape   bool                    // permit paths outside BaseDir
	MaxBytes      int                     // default DefaultMaxBytes
	Debug         bool                    // write tiles next to source under --debug
	DebugWriter   io.Writer               // receives "[langchat] --debug ..." lines
	ImageMIMEs    map[string]string       // defaults to DefaultImageExtensions
	OnPatchifyAny func(args PatchifyArgs) // optional callback when patchify is used (for tests)
}

// DirectiveRE matches {{ include "..." }} (kept for tests that want to
// assert the regex directly).
var DirectiveRE = includeDirectiveRE

// Resolve walks the body, expanding includes and collecting attachments.
func Resolve(text string, opts Options) (Result, error) {
	if opts.BaseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Result{}, err
		}
		opts.BaseDir = wd
	} else {
		abs, err := filepath.Abs(opts.BaseDir)
		if err != nil {
			return Result{}, err
		}
		opts.BaseDir = abs
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 8
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	if opts.ImageMIMEs == nil {
		opts.ImageMIMEs = DefaultImageExtensions
	}

	r := &resolver{
		opts:  opts,
		stack: []string{},
	}
	finalText, err := r.expand(text, opts.BaseDir, 0)
	if err != nil {
		return Result{}, err
	}
	finalText, err = r.applyPatchify(finalText, opts.BaseDir)
	if err != nil {
		return Result{}, err
	}
	return Result{Text: finalText, Attachments: r.attachments}, nil
}

type resolver struct {
	opts        Options
	stack       []string
	attachments []parser.Attachment
}

func (r *resolver) push(file string) {
	r.stack = append(r.stack, file)
}

func (r *resolver) pop() {
	r.stack = r.stack[:len(r.stack)-1]
}

func (r *resolver) recordImage(rawPath, resolved string, data []byte) error {
	if len(data) > r.opts.MaxBytes {
		return fmt.Errorf(`include "%s" (%s) is %d bytes; limit is %d bytes`, rawPath, resolved, len(data), r.opts.MaxBytes)
	}
	ext := strings.ToLower(filepath.Ext(resolved))
	mime, ok := r.opts.ImageMIMEs[ext]
	if !ok {
		mime = "application/octet-stream"
	}
	r.attachments = append(r.attachments, parser.Attachment{
		Type:     "image",
		MIMEType: mime,
		Data:     base64.StdEncoding.EncodeToString(data),
		Source:   rawPath,
	})
	return nil
}

func (r *resolver) recordTextAttachment(rawPath, resolved string, content string) {
	r.attachments = append(r.attachments, parser.Attachment{
		Type:   "text",
		Text:   content,
		Source: rawPath,
	})
}

func (r *resolver) resolvePath(rawPath, fileDir string) (string, error) {
	var resolved string
	if filepath.IsAbs(rawPath) {
		resolved = filepath.Clean(rawPath)
	} else {
		resolved = filepath.Clean(filepath.Join(fileDir, rawPath))
	}
	absBase, err := filepath.Abs(r.opts.BaseDir)
	if err != nil {
		return "", err
	}
	absBase = filepath.Clean(absBase)
	inside := resolved == absBase || strings.HasPrefix(resolved, absBase+string(filepath.Separator))
	if !inside && !r.opts.AllowEscape {
		return "", fmt.Errorf(`include "%s" escapes base directory %s`, rawPath, absBase)
	}
	if r.stackContains(resolved) {
		return "", fmt.Errorf("cyclic include detected: %s -> %s", strings.Join(r.stack, " -> "), resolved)
	}
	return resolved, nil
}

func (r *resolver) stackContains(target string) bool {
	for _, p := range r.stack {
		if p == target {
			return true
		}
	}
	return false
}

func (r *resolver) expand(snippet, fileDir string, depth int) (string, error) {
	if depth > r.opts.MaxDepth {
		return "", fmt.Errorf("include depth exceeded %d", r.opts.MaxDepth)
	}

	var out strings.Builder
	cursor := 0
	for _, m := range includeMatches(snippet) {
		if m.Start > cursor {
			out.WriteString(snippet[cursor:m.Start])
		}

		resolved, err := r.resolvePath(m.Path, fileDir)
		if err != nil {
			return "", err
		}

		ext := strings.ToLower(filepath.Ext(resolved))
		_, isImage := r.opts.ImageMIMEs[ext]

		r.push(resolved)

		if isImage {
			data, err := os.ReadFile(resolved)
			if err != nil {
				r.pop()
				return "", fmt.Errorf("include failed: %s (%s): %v", m.Path, resolved, err)
			}
			if err := r.recordImage(m.Path, resolved, data); err != nil {
				r.pop()
				return "", err
			}
			out.WriteString(m.Full)
		} else {
			content, err := os.ReadFile(resolved)
			if err != nil {
				r.pop()
				return "", fmt.Errorf("include failed: %s (%s): %v", m.Path, resolved, err)
			}
			if len(content) > r.opts.MaxBytes {
				r.pop()
				return "", fmt.Errorf(`include "%s" (%s) exceeds %d bytes`, m.Path, resolved, r.opts.MaxBytes)
			}
			nested, err := r.expand(string(content), filepath.Dir(resolved), depth+1)
			if err != nil {
				r.pop()
				return "", err
			}
			out.WriteString(nested)
		}

		r.pop()
		cursor = m.End
	}
	if cursor < len(snippet) {
		out.WriteString(snippet[cursor:])
	}
	return out.String(), nil
}

func (r *resolver) applyPatchify(text, fileDir string) (string, error) {
	matches := patchifyMatches(text)
	if len(matches) == 0 {
		return text, nil
	}

	result := text
	// walk in reverse so offsets remain valid as we splice
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		args, err := ParsePatchifyArgs(m.Full)
		if err != nil {
			return "", err
		}
		if err := ValidatePatchifyArgs(args); err != nil {
			return "", err
		}

		resolved, err := r.resolvePath(args.Path, fileDir)
		if err != nil {
			return "", err
		}

		data, err := os.ReadFile(resolved)
		if err != nil {
			return "", fmt.Errorf("patchify failed: %s (%s): %v", args.Path, resolved, err)
		}
		if len(data) > r.opts.MaxBytes {
			return "", fmt.Errorf(`patchify "%s" (%s) is %d bytes; limit is %d bytes`, args.Path, resolved, len(data), r.opts.MaxBytes)
		}

		r.push(resolved)
		tiles, err := Patchify(data, args)
		r.pop()
		if err != nil {
			return "", err
		}

		placeholders := make([]string, 0, len(tiles))
		debugWritten := 0
		for _, t := range tiles {
			label := SourceLabelFor(args.Path, t.Row, t.Col)
			r.attachments = append(r.attachments, parser.Attachment{
				Type:     "image",
				MIMEType: t.MIMEType,
				Data:     base64.StdEncoding.EncodeToString(t.Patch),
				Source:   label,
			})
			placeholders = append(placeholders, fmt.Sprintf(`{{ include %q }}`, label))
			if r.opts.Debug {
				out := filepath.Join(filepath.Dir(resolved), label)
				if err := os.WriteFile(out, t.Patch, 0o644); err == nil {
					debugWritten++
				} else if r.opts.DebugWriter != nil {
					fmt.Fprintf(r.opts.DebugWriter, "[langchat] --debug failed to write %s: %v\n", out, err)
				}
			}
		}
		if r.opts.Debug && debugWritten > 0 && r.opts.DebugWriter != nil {
			fmt.Fprintf(r.opts.DebugWriter, "[langchat] --debug wrote %d patch%s next to %s\n",
				debugWritten, plural(debugWritten, "", "es"), resolved)
		}
		if r.opts.OnPatchifyAny != nil {
			r.opts.OnPatchifyAny(args)
		}

		replacement := strings.Join(placeholders, " ")
		result = result[:m.Start] + replacement + result[m.End:]
	}
	return result, nil
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
