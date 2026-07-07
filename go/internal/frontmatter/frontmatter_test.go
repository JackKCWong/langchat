package frontmatter

import (
	"reflect"
	"strings"
	"testing"
)

func TestReturnsEmptyOptsWhenNoFrontmatter(t *testing.T) {
	md := "# !system\n\nhi\n"
	r, err := Parse(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(r.Opts, map[string]any{}) {
		t.Errorf("opts = %v, want empty", r.Opts)
	}
	if r.HeaderLines != 0 {
		t.Errorf("headerLines = %d, want 0", r.HeaderLines)
	}
	if r.Body != md {
		t.Errorf("body = %q, want %q", r.Body, md)
	}
}

func TestReturnsEmptyOptsWhenNonDelimiterFirstLine(t *testing.T) {
	md := "   ---\nmodel: x\n---\nbody"
	r, err := Parse(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.HeaderLines != 0 {
		t.Errorf("headerLines = %d, want 0", r.HeaderLines)
	}
	if r.Body != md {
		t.Errorf("body = %q, want %q", r.Body, md)
	}
}

func TestReturnsEmptyOptsForLoneDelimiter(t *testing.T) {
	r, err := Parse("---")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.HeaderLines != 0 {
		t.Errorf("headerLines = %d, want 0", r.HeaderLines)
	}
	if r.Body != "---" {
		t.Errorf("body = %q, want ---", r.Body)
	}
}

func TestParsesSingleStringKey(t *testing.T) {
	md := "---\nmodel: qwen-vl-plus\n---\n# !user\n\nhi\n"
	r, err := Parse(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(r.Opts, map[string]any{"model": "qwen-vl-plus"}) {
		t.Errorf("opts = %v", r.Opts)
	}
	if r.HeaderLines != 3 {
		t.Errorf("headerLines = %d, want 3", r.HeaderLines)
	}
	if r.Body != "# !user\n\nhi\n" {
		t.Errorf("body = %q", r.Body)
	}
}

func TestParsesMultipleKeysOfMixedTypes(t *testing.T) {
	md := strings.Join([]string{
		"---",
		"model: qwen-vl-plus",
		"streaming: true",
		"temperature: 0.7",
		"max_tokens: 1024",
		"thinking: false",
		"---",
		"# !user",
		"",
		"hi",
		"",
	}, "\n")
	r, err := Parse(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{
		"model":       "qwen-vl-plus",
		"streaming":   true,
		"temperature": 0.7,
		"max_tokens":  1024,
		"thinking":    false,
	}
	if !reflect.DeepEqual(r.Opts, want) {
		t.Errorf("opts = %v, want %v", r.Opts, want)
	}
	if r.HeaderLines != 7 {
		t.Errorf("headerLines = %d, want 7", r.HeaderLines)
	}
	if r.Body != "# !user\n\nhi\n" {
		t.Errorf("body = %q", r.Body)
	}
}

func TestParsesIntegersVsFloatsDistinctly(t *testing.T) {
	r, err := Parse("---\na: 1\nb: 1.0\nc: -3\nd: 0.25\n---\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, _ := r.Opts["a"].(int); v != 1 {
		t.Errorf("a = %v, want 1", r.Opts["a"])
	}
	if v, _ := r.Opts["b"].(float64); v != 1.0 {
		t.Errorf("b = %v, want 1.0", r.Opts["b"])
	}
	if v, _ := r.Opts["c"].(int); v != -3 {
		t.Errorf("c = %v, want -3", r.Opts["c"])
	}
	if v, _ := r.Opts["d"].(float64); v != 0.25 {
		t.Errorf("d = %v, want 0.25", r.Opts["d"])
	}
}

func TestParsesQuotedStrings(t *testing.T) {
	r, err := Parse("---\na: \"hello world\"\nb: 'with: colon'\n---\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Opts["a"] != "hello world" {
		t.Errorf("a = %v", r.Opts["a"])
	}
	if r.Opts["b"] != "with: colon" {
		t.Errorf("b = %v", r.Opts["b"])
	}
}

func TestParsesNullAndEmptyValues(t *testing.T) {
	r, err := Parse("---\na: null\nb: ~\nc: \n---\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Opts["a"] != nil {
		t.Errorf("a = %v, want nil", r.Opts["a"])
	}
	if r.Opts["b"] != nil {
		t.Errorf("b = %v, want nil", r.Opts["b"])
	}
	if r.Opts["c"] != "" {
		t.Errorf("c = %v, want \"\"", r.Opts["c"])
	}
}

func TestSkipsBlanksAndComments(t *testing.T) {
	r, err := Parse("---\n# a comment\n\nmodel: foo\n# trailing\n---\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(r.Opts, map[string]any{"model": "foo"}) {
		t.Errorf("opts = %v", r.Opts)
	}
}

func TestHandlesCRLFLineEndings(t *testing.T) {
	md := "---\r\nmodel: x\r\n---\r\n# !user\r\n\r\nhi\r\n"
	r, err := Parse(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(r.Opts, map[string]any{"model": "x"}) {
		t.Errorf("opts = %v", r.Opts)
	}
	if r.HeaderLines != 3 {
		t.Errorf("headerLines = %d, want 3", r.HeaderLines)
	}
	if !strings.HasPrefix(r.Body, "# !user") {
		t.Errorf("body does not start with # !user: %q", r.Body)
	}
	if !strings.Contains(r.Body, "hi") {
		t.Errorf("body missing 'hi': %q", r.Body)
	}
}

func TestLoneDelimiterWithoutCloserIsNotFrontmatter(t *testing.T) {
	md := "---\nmodel: x\n# !user\n\nhi\n"
	r, err := Parse(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.HeaderLines != 0 {
		t.Errorf("headerLines = %d, want 0", r.HeaderLines)
	}
	if r.Body != md {
		t.Errorf("body = %q", r.Body)
	}
}

func TestThrowsOnLineWithoutColon(t *testing.T) {
	_, err := Parse("---\nbareword\n---\n")
	if err == nil || !strings.Contains(err.Error(), `expected "key: value" but got "bareword"`) {
		t.Errorf("err = %v, want colon-missing message", err)
	}
}

func TestThrowsOnEmptyKey(t *testing.T) {
	_, err := Parse("---\n: value\n---\n")
	if err == nil || !strings.Contains(err.Error(), "empty key") {
		t.Errorf("err = %v, want empty-key message", err)
	}
}

func TestThrowsOnInvalidKeyCharacters(t *testing.T) {
	_, err := Parse("---\n1bad: x\n---\n")
	if err == nil || !strings.Contains(err.Error(), `invalid key "1bad"`) {
		t.Errorf("err = %v, want invalid-key message", err)
	}
}

func TestThrowsOnIndentedKey(t *testing.T) {
	_, err := Parse("---\n  model: x\n---\n")
	if err == nil || !strings.Contains(err.Error(), "keys must not be indented") {
		t.Errorf("err = %v, want indent message", err)
	}
}

func TestRejectsNonStringInput(t *testing.T) {
	if _, err := Parse(""); err != nil {
		t.Errorf("empty string is allowed (no frontmatter), got %v", err)
	}
}

func TestParseScalarValueSubset(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"foo", "foo"},
		{"  foo  ", "foo"},
		{`"with spaces"`, "with spaces"},
		{"'single quoted'", "single quoted"},
		{"true", true},
		{"false", false},
		{"null", nil},
		{"~", nil},
		{"", ""},
		{"3", 3},
		{"3.14", 3.14},
		{"-2", -2},
	}
	for _, c := range cases {
		got := ParseScalarValue(c.in)
		if got != c.want {
			t.Errorf("ParseScalarValue(%q) = %v (%T), want %v (%T)", c.in, got, got, c.want, c.want)
		}
	}
}