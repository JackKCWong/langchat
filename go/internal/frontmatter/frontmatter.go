// Package frontmatter parses the optional "---"-delimited YAML-style metadata
// header that may precede the body of a chat file. Values are parsed as
// scalars: strings (bare, double-, or single-quoted), integers, floats,
// booleans, and null / ~. Comments (# ...) and blank lines inside the header
// are ignored. Keys must not be indented and must match [a-zA-Z_][\w.-]*.
package frontmatter

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Result struct {
	Body        string
	Opts        map[string]any
	HeaderLines int
}

var (
	delimiterRE = regexp.MustCompile(`^---\s*$`)
	keyRE       = regexp.MustCompile(`^[a-zA-Z_][\w.-]*$`)
	intRE       = regexp.MustCompile(`^-?\d+$`)
	floatRE     = regexp.MustCompile(`^-?\d*\.\d+([eE][-+]?\d+)?$|^-?\d+[eE][-+]?\d+$`)
	dqRE        = regexp.MustCompile(`^"(.*)"\s*$`)
	sqRE        = regexp.MustCompile(`^'(.*)'\s*$`)
)

// ParseScalarValue is exported for testing. Mirrors src/frontmatter.js#parseScalarValue.
func ParseScalarValue(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "|" || trimmed == ">" {
		return ""
	}
	if m := dqRE.FindStringSubmatch(trimmed); m != nil {
		return m[1]
	}
	if m := sqRE.FindStringSubmatch(trimmed); m != nil {
		return m[1]
	}
	switch trimmed {
	case "true":
		return true
	case "false":
		return false
	case "null", "~":
		return nil
	}
	if intRE.MatchString(trimmed) {
		n, _ := strconv.Atoi(trimmed)
		return n
	}
	if floatRE.MatchString(trimmed) {
		f, _ := strconv.ParseFloat(trimmed, 64)
		return f
	}
	return trimmed
}

// Parse splits a chat-file body into the frontmatter header (if present) and
// the remaining body. HeaderErrors carry the 1-based line number in the
// original file.
func Parse(text string) (Result, error) {
	normalized := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(normalized, "\n")

	if len(lines) == 0 || !delimiterRE.MatchString(lines[0]) {
		return Result{Body: text, Opts: map[string]any{}, HeaderLines: 0}, nil
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if delimiterRE.MatchString(lines[i]) {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return Result{Body: text, Opts: map[string]any{}, HeaderLines: 0}, nil
	}

	opts := map[string]any{}
	for i := 1; i < closeIdx; i++ {
		rawLine := lines[i]
		lineNumber := i + 1
		trimmed := strings.TrimSpace(rawLine)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(rawLine, " ") || strings.HasPrefix(rawLine, "\t") {
			return Result{}, fmt.Errorf(
				"frontmatter at line %d: keys must not be indented (got %q)",
				lineNumber, rawLine,
			)
		}

		colonIdx := strings.Index(rawLine, ":")
		if colonIdx == -1 {
			return Result{}, fmt.Errorf(
				"frontmatter at line %d: expected \"key: value\" but got %q",
				lineNumber, rawLine,
			)
		}

		key := strings.TrimSpace(rawLine[:colonIdx])
		if key == "" {
			return Result{}, fmt.Errorf(
				"frontmatter at line %d: empty key before \":\"",
				lineNumber,
			)
		}
		if !keyRE.MatchString(key) {
			return Result{}, fmt.Errorf(
				"frontmatter at line %d: invalid key %q",
				lineNumber, key,
			)
		}

		opts[key] = ParseScalarValue(rawLine[colonIdx+1:])
	}

	headerLines := closeIdx + 1
	body := strings.Join(lines[headerLines:], "\n")
	return Result{Body: body, Opts: opts, HeaderLines: headerLines}, nil
}