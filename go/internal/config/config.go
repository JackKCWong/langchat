// Package config assembles the runtime configuration for a langchat
// invocation. It loads .env files from the current working directory
// (existing env wins), then merges CLI flags, frontmatter header keys,
// and LANGCHAT_* env vars into a single Config struct that the chat
// package consumes.
//
// Mirrors src/cli.js#resolveConfig from the Node.js implementation.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the merged configuration. Only fields that are non-nil/empty
// after resolution are populated. Extra is a catch-all map for unknown
// header keys (and CLI keys the caller wants forwarded) that get sent as
// request-body fields via the chat.Transport wrapper.
type Config struct {
	Model    string
	BaseURL  string
	APIKey   string
	Timeout  *int
	MaxRetries *int

	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stop        []string
	Presence    *float64
	Frequency   *float64
	Seed        *int

	Thinking *bool

	Extra map[string]any
}

// knownHeaderFields maps snake_case header keys to the Config field
// they populate (when set via frontmatter). Mirrors src/cli.js.
type headerField struct {
	Header string
	Set    func(c *Config, v any)
	EnvVar string
}

func (h headerField) EnvName() string { return h.EnvVar }

var knownHeaderFields = []headerField{
	{
		Header: "temperature",
		EnvVar: "LANGCHAT_TEMPERATURE",
		Set: func(c *Config, v any) {
			if f, ok := toFloat(v); ok {
				c.Temperature = &f
			}
		},
	},
	{
		Header: "top_p",
		EnvVar: "LANGCHAT_TOP_P",
		Set: func(c *Config, v any) {
			if f, ok := toFloat(v); ok {
				c.TopP = &f
			}
		},
	},
	{
		Header: "max_tokens",
		EnvVar: "LANGCHAT_MAX_TOKENS",
		Set: func(c *Config, v any) {
			if n, ok := toInt(v); ok {
				c.MaxTokens = &n
			}
		},
	},
	{
		Header: "timeout",
		EnvVar: "LANGCHAT_TIMEOUT",
		Set: func(c *Config, v any) {
			if n, ok := toInt(v); ok {
				c.Timeout = &n
			}
		},
	},
	{
		Header: "max_retries",
		EnvVar: "LANGCHAT_MAX_RETRIES",
		Set: func(c *Config, v any) {
			if n, ok := toInt(v); ok {
				c.MaxRetries = &n
			}
		},
	},
}

// IsKnownHeaderKey reports whether key (in snake_case form) is one of the
// recognized header fields.
func IsKnownHeaderKey(key string) bool {
	for _, h := range knownHeaderFields {
		if h.Header == key {
			return true
		}
	}
	return false
}

// LoadDotenv reads .env from the given directory and sets any var that is
// not already defined. Existing os.Getenv values win (matches
// process.loadEnvFile behavior).
func LoadDotenv(dir string) error {
	path := dir
	if path == "" {
		path, _ = os.Getwd()
	}
	f, err := os.Open(path + "/.env")
	if err != nil {
		// Missing .env is not fatal.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}

// ResolveInput is the input to Resolve.
type ResolveInput struct {
	CLIModel     string
	CLIThinking  *bool
	Header       map[string]any
}

// Resolve merges CLI > header > env into a Config.
func Resolve(in ResolveInput) (Config, error) {
	cfg := Config{Extra: map[string]any{}}

	cfg.Model = pickStr(in.CLIModel, stringFromHeader(in.Header, "model"), os.Getenv("LANGCHAT_MODEL"))
	if cfg.Model == "" {
		return cfg, fmt.Errorf(
			"LANGCHAT_MODEL is required. Set it to the model name " +
				"(e.g. export LANGCHAT_MODEL=gpt-4o-mini) or pass -m/--model, " +
				`or add "model: <name>" to a "---" header in the chat file.`,
		)
	}

	if v := os.Getenv("LANGCHAT_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}

	if v := os.Getenv("LANGCHAT_API_KEY"); v != "" {
		cfg.APIKey = v
	} else if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.APIKey = v
	} else {
		fmt.Fprintln(os.Stderr,
			"[langchat] warning: no API key found in LANGCHAT_API_KEY or OPENAI_API_KEY; "+
				`using placeholder "sk-no-key-needed" so local servers (Ollama, LM Studio, vLLM) work.`)
		cfg.APIKey = "sk-no-key-needed"
	}

	// Env-driven known fields.
	for _, h := range knownHeaderFields {
		if v, ok := readEnvNumber(h.EnvVar); ok {
			h.Set(&cfg, v)
		}
	}

	// Header keys: known fields set typed fields; unknown ones land in Extra.
	for k, v := range in.Header {
		if v == nil {
			continue
		}
		switch k {
		case "model", "streaming", "thinking", "output":
			// model is handled above; streaming is no longer a flag;
			// thinking is applied below; output is consumed by the caller.
			if k == "thinking" {
				if b, ok := v.(bool); ok {
					bv := b
					cfg.Thinking = &bv
				}
			}
			continue
		}
		matched := false
		for _, h := range knownHeaderFields {
			if h.Header == k {
				h.Set(&cfg, v)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		// Convenience aliases: snake_case → camelCase fields, plus extras.
		switch k {
		case "stop":
			if arr, ok := v.([]any); ok {
				for _, e := range arr {
					if s, ok := e.(string); ok {
						cfg.Stop = append(cfg.Stop, s)
					}
				}
			}
		case "presence_penalty":
			if f, ok := toFloat(v); ok {
				cfg.Presence = &f
			}
		case "frequency_penalty":
			if f, ok := toFloat(v); ok {
				cfg.Frequency = &f
			}
		case "seed":
			if n, ok := toInt(v); ok {
				cfg.Seed = &n
			}
		default:
			cfg.Extra[k] = v
		}
	}

	// CLI thinking wins over header thinking.
	if in.CLIThinking != nil {
		cfg.Thinking = in.CLIThinking
	}

	return cfg, nil
}

// ReadEnvNumber parses env[name] as a float64; returns (n, true) on success.
// Returns (0, false) for missing/empty. Returns an error for non-numeric
// values, mirroring src/cli.js#readEnvNumber.
func ReadEnvNumber(name string) (float64, error) {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return 0, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("env %s=%q is not a valid number", name, v)
	}
	return n, nil
}

func readEnvNumber(name string) (float64, bool) {
	n, err := ReadEnvNumber(name)
	if err != nil {
		// The JS code throws on bad numbers; we surface the same error from
		// Resolve via this side channel.
		// To keep the signature simple, we just return false here; Resolve
		// callers that want strictness should call ReadEnvNumber directly.
		return 0, false
	}
	return n, true
}

func pickStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func stringFromHeader(h map[string]any, key string) string {
	if h == nil {
		return ""
	}
	if v, ok := h[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(x)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}