package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JackKCWong/langchat-go/internal/output"
	"github.com/tmc/langchaingo/llms"
	openai "github.com/tmc/langchaingo/llms/openai"
)

// RunChat sends messages to the LLM using streaming (the default), with
// silent fallback to a non-streaming call if the SSE stream fails.
//
// Reasoning tokens are written dimmed to stdout; main content is written
// to stdout (and the output file if non-nil) line-by-line via the line
// writer.
func RunChat(ctx context.Context, llm *openai.LLM, msgs []llms.MessageContent, file string) error {
	main, err := openOutput(file)
	if err != nil {
		return err
	}
	defer main.Close()

	mainWriter := output.NewLineWriter(main, output.WithDim(false))
	defer mainWriter.End()

	dimWriter := output.NewLineWriter(nil, output.WithDim(true))
	defer dimWriter.End()

	// Build the per-call options. We pass a streaming-reasoning func so
	// reasoning chunks (DeepSeek/Anthropic) and content chunks can be
	// routed independently, matching the JS extractThinking split.
	opts := []llms.CallOption{
		llms.WithStreamingReasoningFunc(func(_ context.Context, reasoning, chunk []byte) error {
			if len(reasoning) > 0 {
				dimWriter.Write(string(reasoning))
			}
			if len(chunk) > 0 {
				mainWriter.Write(string(chunk))
			}
			return nil
		}),
	}

	_, err = llm.GenerateContent(ctx, msgs, opts...)
	if err == nil {
		return nil
	}

	// Streaming call failed → fall back silently to a non-streaming call.
	dimWriter.End()
	mainWriter.End()
	if err := resetOutput(main); err != nil {
		return fmt.Errorf("langchat: request failed: %v", err)
	}

	resp, err := llm.GenerateContent(ctx, msgs)
	if err != nil {
		return fmt.Errorf("langchat: request failed: %v", err)
	}
	if len(resp.Choices) == 0 {
		return errors.New("langchat: empty response")
	}
	mainWriter.Write(extractContent(resp))
	return nil
}

func extractContent(resp *llms.ContentResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	c := resp.Choices[0]
	if c.Content != "" {
		return c.Content
	}
	// Defensive: langchaingo also returns the full text in GenerationInfo
	// for some providers. Fall back to that if Content is empty.
	if s, ok := c.GenerationInfo["Text"].(string); ok {
		return s
	}
	if s, ok := c.GenerationInfo["Content"].(string); ok {
		return s
	}
	return ""
}

// PrintReasoning extracts reasoning text from a non-streaming response
// (langchaingo stores it in GenerationInfo["ThinkingContent"]) and writes
// it dimmed to stdout. Used by the structured-output fallback.
func PrintReasoning(resp *llms.ContentResponse) {
	if resp == nil || len(resp.Choices) == 0 {
		return
	}
	info := resp.Choices[0].GenerationInfo
	if info == nil {
		return
	}
	if s, ok := info["ThinkingContent"].(string); ok && s != "" {
		fmt.Print("\x1b[2m" + strings.TrimRight(s, "\n") + "\x1b[0m\n")
	}
}
