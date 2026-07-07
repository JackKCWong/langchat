package args

import "testing"

func ptr[T any](v T) *T { return &v }

func TestEmptyArgv(t *testing.T) {
	opts, err := Parse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.File != "" {
		t.Errorf("file = %q, want empty", opts.File)
	}
	if opts.Help {
		t.Errorf("help = true, want false")
	}
}

func TestHelpShort(t *testing.T) {
	opts, err := Parse([]string{"-h"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Help {
		t.Errorf("help = false, want true")
	}
}

func TestHelpLong(t *testing.T) {
	opts, err := Parse([]string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Help {
		t.Errorf("help = false, want true")
	}
}

func TestDebugFlag(t *testing.T) {
	opts, err := Parse([]string{"-d", "chat.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Debug {
		t.Errorf("debug = false, want true")
	}
	if opts.File != "chat.md" {
		t.Errorf("file = %q, want chat.md", opts.File)
	}
}

func TestBareFileArgument(t *testing.T) {
	opts, err := Parse([]string{"chat.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.File != "chat.md" {
		t.Errorf("file = %q", opts.File)
	}
	if opts.Help {
		t.Errorf("help = true, want false")
	}
}

func TestMultipleFilesThrow(t *testing.T) {
	_, err := Parse([]string{"a.md", "b.md"})
	if err == nil {
		t.Fatalf("expected error")
	}
	want := "Expected exactly one <chat.md> argument"
	if got := err.Error(); !contains(got, want) {
		t.Errorf("err = %q, want substring %q", got, want)
	}
}

func TestUnknownLongFlag(t *testing.T) {
	_, err := Parse([]string{"--bogus"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); !contains(got, "Unknown option: --bogus") {
		t.Errorf("err = %q", got)
	}
}

func TestUnknownShortFlag(t *testing.T) {
	_, err := Parse([]string{"-x"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); !contains(got, "Unknown option: -x") {
		t.Errorf("err = %q", got)
	}
}

func TestDashDashTerminates(t *testing.T) {
	opts, err := Parse([]string{"--", "chat.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.File != "chat.md" {
		t.Errorf("file = %q", opts.File)
	}
	if opts.Debug {
		t.Errorf("debug should be false after --")
	}
}

func TestDashDashMakesFlagShapedPositional(t *testing.T) {
	opts, err := Parse([]string{"--", "-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.File != "-x" {
		t.Errorf("file = %q, want -x", opts.File)
	}
}

func TestHelpWithFileHelpWins(t *testing.T) {
	opts, err := Parse([]string{"-h", "chat.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Help {
		t.Errorf("help = false, want true")
	}
	if opts.File != "chat.md" {
		t.Errorf("file = %q", opts.File)
	}
}

func TestAllowIncludeEscapeDefaultsFalse(t *testing.T) {
	opts, _ := Parse([]string{"chat.md"})
	if opts.AllowIncludeEscape {
		t.Errorf("allowIncludeEscape = true, want false")
	}
}

func TestAllowIncludeEscapeSets(t *testing.T) {
	opts, _ := Parse([]string{"--allow-include-escape", "chat.md"})
	if !opts.AllowIncludeEscape {
		t.Errorf("allowIncludeEscape = false, want true")
	}
	if opts.File != "chat.md" {
		t.Errorf("file = %q", opts.File)
	}
}

func TestModelDefaultsEmpty(t *testing.T) {
	opts, _ := Parse([]string{"chat.md"})
	if opts.Model != "" {
		t.Errorf("model = %q, want empty", opts.Model)
	}
}

func TestModelShort(t *testing.T) {
	opts, err := Parse([]string{"-m", "gpt-4o-mini", "chat.md"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if opts.Model != "gpt-4o-mini" {
		t.Errorf("model = %q", opts.Model)
	}
}

func TestModelLong(t *testing.T) {
	opts, _ := Parse([]string{"--model", "llama3.1", "chat.md"})
	if opts.Model != "llama3.1" {
		t.Errorf("model = %q", opts.Model)
	}
}

func TestModelAfterFile(t *testing.T) {
	opts, _ := Parse([]string{"chat.md", "-m", "deepseek-chat"})
	if opts.Model != "deepseek-chat" {
		t.Errorf("model = %q", opts.Model)
	}
}

func TestModelWithoutValueThrows(t *testing.T) {
	if _, err := Parse([]string{"-m"}); err == nil {
		t.Errorf("expected error")
	}
	if _, err := Parse([]string{"--model"}); err == nil {
		t.Errorf("expected error")
	}
	if _, err := Parse([]string{"--model", "-d"}); err == nil {
		t.Errorf("expected error")
	}
}

func TestModelWithFlagShapedValueThrows(t *testing.T) {
	_, err := Parse([]string{"-m", "-d"})
	if err == nil {
		t.Errorf("expected error")
	}
	if got := err.Error(); !contains(got, "requires a model name") {
		t.Errorf("err = %q", got)
	}
}

func TestOutputDefaultsEmpty(t *testing.T) {
	opts, _ := Parse([]string{"chat.md"})
	if opts.Output != "" {
		t.Errorf("output = %q, want empty", opts.Output)
	}
}

func TestOutputShort(t *testing.T) {
	opts, _ := Parse([]string{"-o", "out.md", "chat.md"})
	if opts.Output != "out.md" {
		t.Errorf("output = %q", opts.Output)
	}
}

func TestOutputLong(t *testing.T) {
	opts, _ := Parse([]string{"--output", "results/reply.md", "chat.md"})
	if opts.Output != "results/reply.md" {
		t.Errorf("output = %q", opts.Output)
	}
}

func TestOutputAfterFile(t *testing.T) {
	opts, _ := Parse([]string{"chat.md", "-o", "reply.md"})
	if opts.Output != "reply.md" {
		t.Errorf("output = %q", opts.Output)
	}
}

func TestOutputWithoutValueThrows(t *testing.T) {
	for _, in := range [][]string{
		{"-o"},
		{"--output"},
	} {
		if _, err := Parse(in); err == nil {
			t.Errorf("expected error for %v", in)
		}
	}
}

func TestOutputWithFlagShapedValueThrows(t *testing.T) {
	_, err := Parse([]string{"-o", "-d"})
	if err == nil {
		t.Errorf("expected error")
	}
	if got := err.Error(); !contains(got, "requires an output path") {
		t.Errorf("err = %q", got)
	}
}

func TestDashDashMakesFlagShapedOutputPositional(t *testing.T) {
	opts, err := Parse([]string{"--", "-o"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if opts.File != "-o" {
		t.Errorf("file = %q, want -o", opts.File)
	}
	if opts.Output != "" {
		t.Errorf("output = %q", opts.Output)
	}
}

func TestThinkingDefaultsNil(t *testing.T) {
	opts, _ := Parse([]string{"chat.md"})
	if opts.Thinking != nil {
		t.Errorf("thinking = %v, want nil", *opts.Thinking)
	}
}

func TestThinkingYes(t *testing.T) {
	opts, _ := Parse([]string{"-t", "yes", "chat.md"})
	if opts.Thinking == nil || !*opts.Thinking {
		t.Errorf("thinking = %v, want true", opts.Thinking)
	}
}

func TestThinkingNo(t *testing.T) {
	opts, _ := Parse([]string{"-t", "no", "chat.md"})
	if opts.Thinking == nil || *opts.Thinking {
		t.Errorf("thinking = %v, want false", opts.Thinking)
	}
}

func TestThinkingEqualsYes(t *testing.T) {
	opts, _ := Parse([]string{"--thinking=yes", "chat.md"})
	if opts.Thinking == nil || !*opts.Thinking {
		t.Errorf("thinking = %v, want true", opts.Thinking)
	}
}

func TestThinkingEqualsNo(t *testing.T) {
	opts, _ := Parse([]string{"--thinking=no", "chat.md"})
	if opts.Thinking == nil || *opts.Thinking {
		t.Errorf("thinking = %v, want false", opts.Thinking)
	}
}

func TestThinkingSpaceSeparated(t *testing.T) {
	opts, _ := Parse([]string{"--thinking", "yes", "chat.md"})
	if opts.Thinking == nil || !*opts.Thinking {
		t.Errorf("thinking = %v, want true", opts.Thinking)
	}
}

func TestThinkingAcceptsManySpellings(t *testing.T) {
	cases := []struct {
		val string
		want bool
	}{
		{"YES", true}, {"true", true}, {"1", true}, {"on", true},
		{"NO", false}, {"false", false}, {"0", false}, {"off", false},
	}
	for _, c := range cases {
		opts, err := Parse([]string{"-t", c.val})
		if err != nil {
			t.Errorf("err for %q: %v", c.val, err)
			continue
		}
		if opts.Thinking == nil || *opts.Thinking != c.want {
			t.Errorf("thinking for %q = %v, want %v", c.val, opts.Thinking, c.want)
		}
	}
}

func TestThinkingWithoutValueThrows(t *testing.T) {
	for _, in := range [][]string{
		{"-t"},
		{"--thinking"},
	} {
		if _, err := Parse(in); err == nil {
			t.Errorf("expected error for %v", in)
		}
	}
}

func TestThinkingEqualsEmptyThrows(t *testing.T) {
	_, err := Parse([]string{"--thinking="})
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestThinkingFlagShapedValueThrows(t *testing.T) {
	_, err := Parse([]string{"-t", "-d"})
	if err == nil {
		t.Errorf("expected error")
	}
	if got := err.Error(); !contains(got, "requires a value") {
		t.Errorf("err = %q", got)
	}
}

func TestThinkingInvalidValueThrows(t *testing.T) {
	_, err := Parse([]string{"-t", "maybe"})
	if err == nil {
		t.Errorf("expected error")
	}
	if got := err.Error(); !contains(got, "expects yes or no") {
		t.Errorf("err = %q", got)
	}
}

func TestThinkingEqualsInvalidThrows(t *testing.T) {
	_, err := Parse([]string{"--thinking=maybe"})
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestThinkingAfterFile(t *testing.T) {
	opts, _ := Parse([]string{"chat.md", "-t", "yes"})
	if opts.Thinking == nil || !*opts.Thinking {
		t.Errorf("thinking after file = %v", opts.Thinking)
	}
	if opts.File != "chat.md" {
		t.Errorf("file = %q", opts.File)
	}
}

func TestParseThinkingValueTruthyStrings(t *testing.T) {
	for _, v := range []string{"yes", "Yes", "YES", "true", "1", "on", "On"} {
		b, err := ParseThinkingValue(v, "-t/--thinking")
		if err != nil {
			t.Errorf("err for %q: %v", v, err)
		}
		if !b {
			t.Errorf("ParseThinkingValue(%q) = false, want true", v)
		}
	}
}

func TestParseThinkingValueFalsyStrings(t *testing.T) {
	for _, v := range []string{"no", "No", "NO", "false", "0", "off"} {
		b, err := ParseThinkingValue(v, "-t/--thinking")
		if err != nil {
			t.Errorf("err for %q: %v", v, err)
		}
		if b {
			t.Errorf("ParseThinkingValue(%q) = true, want false", v)
		}
	}
}

func TestParseThinkingValueEmptyThrows(t *testing.T) {
	if _, err := ParseThinkingValue("", "-t/--thinking"); err == nil {
		t.Errorf("expected error")
	}
}

func TestParseThinkingValueUnknownThrows(t *testing.T) {
	if _, err := ParseThinkingValue("maybe", "-t/--thinking"); err == nil {
		t.Errorf("expected error")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}