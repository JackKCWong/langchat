package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JackKCWong/langchat-go/internal/args"
	"github.com/JackKCWong/langchat-go/internal/chat"
	"github.com/JackKCWong/langchat-go/internal/config"
	"github.com/JackKCWong/langchat-go/internal/frontmatter"
	"github.com/JackKCWong/langchat-go/internal/includes"
	"github.com/JackKCWong/langchat-go/internal/parser"

	"github.com/spf13/cobra"
	"github.com/tmc/langchaingo/llms"
)

const usage = `Usage: langchat [options] <chat.md>

Options:
  -m, --model <name>         Model name (overrides LANGCHAT_MODEL)
  -o, --output <path>        Write the response to <path> as well as stdout
  -d, --debug                Write patchify tiles next to each source image
      --allow-include-escape  Permit include paths outside the chat file's directory
  -t, --thinking <yes|no>    Send "thinking: true"/"thinking: false" in the
                             API request. Reasoning tokens returned by the model
                             are always displayed on stdout in dimmed text.
  -h, --help                 Show this help and exit

A chat file may begin with a "---" metadata header declaring per-file options
such as "model: name", "temperature: 0.7", "thinking: true",
"output: path/to/file.md", or any other key forwarded to the API as a request
body field. Precedence is CLI flag > header > env.

Use a "# !output" block containing a JSON Schema to constrain the response shape.
When present, the model returns a parsed object which is pretty-printed as JSON.

Environment (auto-loaded from ./.env if present; existing env vars win):
  LANGCHAT_MODEL      Model name (required if -m/--model not given), e.g. gpt-4o-mini
  LANGCHAT_BASE_URL   OpenAI-compatible base URL (optional)
  LANGCHAT_API_KEY    API key (optional; falls back to OPENAI_API_KEY)
  LANGCHAT_TEMPERATURE, LANGCHAT_TOP_P, LANGCHAT_MAX_TOKENS,
  LANGCHAT_TIMEOUT, LANGCHAT_MAX_RETRIES
`

func NewRoot() *cobra.Command {
	var (
		flagModel    string
		flagOutput   string
		flagDebug    bool
		flagEscape   bool
		flagThinking string
	)

	cmd := &cobra.Command{
		Use:   "langchat [options] <chat.md>",
		Short: "Run a chat file against an OpenAI-compatible API",
		Long:  usage,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, positional []string) error {
			opts := args.Options{
				File:               positional[0],
				Help:               false,
				Model:              flagModel,
				Output:             flagOutput,
				Debug:              flagDebug,
				AllowIncludeEscape: flagEscape,
			}
			if flagThinking != "" {
				b, err := args.ParseThinkingValue(flagThinking, "-t/--thinking")
				if err != nil {
					return err
				}
				opts.Thinking = &b
			}
			return run(opts)
		},
	}

	cmd.Flags().StringVarP(&flagModel, "model", "m", "", "model name (overrides LANGCHAT_MODEL)")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "write response to <path> as well as stdout")
	cmd.Flags().BoolVarP(&flagDebug, "debug", "d", false, "write patchify tiles next to each source image")
	cmd.Flags().BoolVar(&flagEscape, "allow-include-escape", false, "permit {{ include }} paths outside the chat file's directory")
	cmd.Flags().StringVarP(&flagThinking, "thinking", "t", "", `send "thinking: true"/"thinking: false" in the API request`)
	cmd.SetHelpTemplate(usage)

	return cmd
}

func run(opts args.Options) error {
	if err := config.LoadDotenv(""); err != nil {
		return err
	}

	abs, err := filepath.Abs(opts.File)
	if err != nil {
		return fmt.Errorf("langchat: cannot resolve path: %v", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Errorf("langchat: cannot read %s: %v", opts.File, err)
	}
	body := string(data)

	fm, err := frontmatter.Parse(body)
	if err != nil {
		return fmt.Errorf("langchat: failed to parse frontmatter in %s: %v", opts.File, err)
	}

	res, err := includes.Resolve(fm.Body, includes.Options{
		BaseDir:     filepath.Dir(abs),
		AllowEscape: opts.AllowIncludeEscape,
		Debug:       opts.Debug,
		DebugWriter: os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("langchat: %v", err)
	}

	parsed, err := parser.Parse(res.Text, res.Attachments, parser.ParseOptions{LineOffset: fm.HeaderLines})
	if err != nil {
		return fmt.Errorf("langchat: failed to parse %s: %v", opts.File, err)
	}

	cfg, err := config.Resolve(config.ResolveInput{
		CLIModel:    opts.Model,
		CLIThinking: opts.Thinking,
		Header:      fm.Opts,
	})
	if err != nil {
		return fmt.Errorf("langchat: %v", err)
	}

	outputPath, err := resolveOutputPath(opts.Output, fm.Opts)
	if err != nil {
		return fmt.Errorf("langchat: %v", err)
	}

	msgs := toLLMMessages(parsed.Messages)

	ctx := context.Background()
	if parsed.OutputSchema != nil {
		result, err := chat.RunStructured(ctx, cfg, msgs, parsed.OutputSchema)
		if err != nil {
			return err
		}
		return chat.WriteStructuredJSON(result, outputPath)
	}

	llm, err := chat.BuildLLM(cfg)
	if err != nil {
		return fmt.Errorf("langchat: cannot construct LLM: %v", err)
	}
	return chat.RunChat(ctx, llm, msgs, outputPath)
}

func resolveOutputPath(cliValue string, header map[string]any) (string, error) {
	if cliValue != "" {
		abs, err := filepath.Abs(cliValue)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	if header != nil {
		if v, ok := header["output"]; ok && v != nil && v != "" {
			s, ok := v.(string)
			if !ok {
				return "", fmt.Errorf(`header "output" must be a string path, got %T`, v)
			}
			abs, err := filepath.Abs(s)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	return "", nil
}

func toLLMMessages(msgs []parser.Message) []llms.MessageContent {
	out := make([]llms.MessageContent, len(msgs))
	for i, m := range msgs {
		parts := []llms.ContentPart{}
		switch c := m.Content.(type) {
		case string:
			parts = append(parts, llms.TextPart(c))
		case []parser.ContentBlock:
			for _, b := range c {
				switch b.Type {
				case "text":
					parts = append(parts, llms.TextPart(b.Text))
				case "image":
					// OpenAI-compatible APIs require image_url, not binary.
					// The data URI form keeps everything inline.
					parts = append(parts, llms.ImageURLPart("data:"+b.MIMEType+";base64,"+b.Data))
				}
			}
		}
		out[i] = llms.MessageContent{Role: m.Role, Parts: parts}
	}
	return out
}
