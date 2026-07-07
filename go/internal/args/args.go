// Package args parses the langchat CLI argument vector into an Options
// struct. Mirrors src/cli.js#parseArgs and #parseThinkingValue from the
// Node.js implementation, with the -s/--stream flag removed (streaming is
// now the default whenever feasible).
package args

import (
	"fmt"
	"strings"
)

type Options struct {
	File              string
	Help              bool
	Model             string
	Output            string
	Debug             bool
	AllowIncludeEscape bool
	Thinking          *bool
}

// ParseThinkingValue accepts the same truthy/falsy spellings the JS parser
// did: yes/true/1/on (and uppercase variants) → true, no/false/0/off → false.
// Empty / unknown values return an error mentioning `flag` for diagnostics.
func ParseThinkingValue(raw string, flag string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "yes", "true", "1", "on":
		return true, nil
	case "no", "false", "0", "off":
		return false, nil
	case "":
		return false, fmt.Errorf("option %s requires a value (yes or no)", flag)
	default:
		return false, fmt.Errorf("option %s expects yes or no (got %q)", flag, raw)
	}
}

// Parse splits the argv vector into Options. A missing value for a flag that
// takes one (e.g. "-t") returns an error; an unknown flag returns an error;
// passing more than one positional argument returns an error.
func Parse(argv []string) (Options, error) {
	opts := Options{}
	positional := []string{}

	takeThinking := func(i int, flag string) (string, int, error) {
		if i+1 >= len(argv) {
			return "", 0, fmt.Errorf("option %s requires a value (yes or no)", flag)
		}
		v := argv[i+1]
		if strings.HasPrefix(v, "-") {
			return "", 0, fmt.Errorf("option %s requires a value (yes or no)", flag)
		}
		return v, 2, nil
	}

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			opts.Help = true
		case a == "-d" || a == "--debug":
			opts.Debug = true
		case a == "--allow-include-escape":
			opts.AllowIncludeEscape = true
		case a == "-m" || a == "--model":
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				return opts, fmt.Errorf("option %s requires a model name", a)
			}
			i++
			opts.Model = argv[i]
		case a == "-o" || a == "--output":
			if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
				return opts, fmt.Errorf("option %s requires an output path", a)
			}
			i++
			opts.Output = argv[i]
		case a == "-t" || a == "--thinking":
			if a == "-t" || a == "--thinking" {
				v, consumed, err := takeThinking(i, a)
				if err != nil {
					return opts, err
				}
				b, err := ParseThinkingValue(v, a)
				if err != nil {
					return opts, err
				}
				opts.Thinking = &b
				i += consumed - 1
			}
		case strings.HasPrefix(a, "--thinking="):
			v := strings.TrimPrefix(a, "--thinking=")
			if v == "" {
				return opts, fmt.Errorf("option %s requires a value (yes or no)", a)
			}
			b, err := ParseThinkingValue(v, a)
			if err != nil {
				return opts, err
			}
			opts.Thinking = &b
		case a == "--":
			positional = append(positional, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "-"):
			return opts, fmt.Errorf("Unknown option: %s", a)
		default:
			positional = append(positional, a)
		}
	}

	if len(positional) > 1 {
		return opts, fmt.Errorf("Expected exactly one <chat.md> argument, got %d", len(positional))
	}
	if len(positional) == 1 {
		opts.File = positional[0]
	}
	return opts, nil
}