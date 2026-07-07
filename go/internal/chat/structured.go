package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JackKCWong/langchat-go/internal/config"
	"github.com/JackKCWong/langchat-go/internal/output"
	"github.com/tmc/langchaingo/llms"
	openai "github.com/tmc/langchaingo/llms/openai"
)

// RunStructured sends messages to the LLM with a JSON-schema response
// format and returns the parsed object. Streaming is disabled (silent).
// Reasoning tokens, if any, are printed dimmed to stdout before the JSON.
//
// Because langchaingo's openai.WithResponseFormat is a client option (not a
// per-call option), RunStructured builds a sibling LLM instance with the
// response format pinned, rather than mutating the streaming LLM.
func RunStructured(ctx context.Context, cfg config.Config, msgs []llms.MessageContent, schema map[string]any) (map[string]any, error) {
	rf := buildResponseFormat(schema)

	opts := []openai.Option{
		openai.WithToken(cfg.APIKey),
		openai.WithModel(cfg.Model),
		openai.WithResponseFormat(rf),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}
	opts = append(opts, openai.WithHTTPClient(buildHTTPClient(cfg)))

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("langchat: cannot construct LLM: %v", err)
	}
	resp, err := llm.GenerateContent(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("langchat: request failed: %v", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("langchat: empty response")
	}
	PrintReasoning(resp)
	text := extractContent(resp)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("langchat: model response is not valid JSON: %v\n--- response ---\n%s", err, text)
	}
	return out, nil
}

// WriteStructuredJSON pretty-prints the result and writes it to both
// stdout and the output file (if any).
func WriteStructuredJSON(result map[string]any, outputPath string) error {
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return output.WriteResponse(string(b), outputPath)
}

// buildResponseFormat constructs an openai.ResponseFormat that pins the
// response to a JSON Schema. We translate the JS-style schema (type /
// properties / required / items / enum / description) into the
// openaiclient's ResponseFormatJSONSchemaProperty tree.
func buildResponseFormat(schema map[string]any) *openai.ResponseFormat {
	root := convertSchema(schema)
	return &openai.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &openai.ResponseFormatJSONSchema{
			Name:   "langchat_output",
			Strict: true,
			Schema: root,
		},
	}
}

func convertSchema(v any) *openai.ResponseFormatJSONSchemaProperty {
	m, ok := v.(map[string]any)
	if !ok {
		return &openai.ResponseFormatJSONSchemaProperty{Type: "object", AdditionalProperties: true}
	}
	t, _ := m["type"].(string)
	if t == "" {
		t = "object"
	}
	p := &openai.ResponseFormatJSONSchemaProperty{Type: t}
	if d, ok := m["description"].(string); ok {
		p.Description = d
	}
	if e, ok := m["enum"].([]any); ok {
		p.Enum = e
	}
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				p.Required = append(p.Required, s)
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		p.Items = convertSchema(items)
	}
	if props, ok := m["properties"].(map[string]any); ok {
		p.Properties = make(map[string]*openai.ResponseFormatJSONSchemaProperty, len(props))
		for k, val := range props {
			p.Properties[k] = convertSchema(val)
		}
		p.AdditionalProperties = false
	}
	return p
}
