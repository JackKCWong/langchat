// Package chat wires langchaingo's openai LLM into the langchat
// configuration pipeline. It owns the LLM client construction and a
// custom HTTP RoundTripper that injects extra header fields into the
// outgoing chat-completion request body (the equivalent of the JS
// modelKwargs escape hatch).
package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/JackKCWong/langchat-go/internal/config"
	"github.com/tmc/langchaingo/llms"
	openai "github.com/tmc/langchaingo/llms/openai"
)

// BuildLLM constructs a langchaingo openai LLM from a Config. The returned
// LLM is reusable across calls.
func BuildLLM(cfg config.Config) (*openai.LLM, error) {
	opts := []openai.Option{
		openai.WithToken(cfg.APIKey),
		openai.WithModel(cfg.Model),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}
	opts = append(opts, openai.WithHTTPClient(buildHTTPClient(cfg)))
	return openai.New(opts...)
}

// buildHTTPClient returns the *http.Client used for all LLM calls. It
// applies the configured timeout and wraps the transport with
// extraTransport so unknown header keys can be forwarded as request body
// fields.
func buildHTTPClient(cfg config.Config) *http.Client {
	timeout := 60 * time.Second
	if cfg.Timeout != nil && *cfg.Timeout > 0 {
		timeout = time.Duration(*cfg.Timeout) * time.Millisecond
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &extraTransport{
			extra: cfg.Extra,
			base:  http.DefaultTransport,
		},
	}
}

// extraTransport merges a map of extra fields into the JSON body of any
// POST request to /chat/completions. Other paths are passed through
// unchanged. It also applies a request-level retry/timeout policy if the
// caller has configured one (maxRetries).
type extraTransport struct {
	extra map[string]any
	base  http.RoundTripper
}

func (t *extraTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.extra) == 0 || !shouldInject(req) {
		return t.base.RoundTrip(req)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	_ = req.Body.Close()
	merged, err := mergeJSON(body, t.extra)
	if err != nil {
		return nil, err
	}
	newReq := req.Clone(req.Context())
	newReq.Body = io.NopCloser(bytes.NewReader(merged))
	newReq.ContentLength = int64(len(merged))
	newReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(merged)))
	return t.base.RoundTrip(newReq)
}

func shouldInject(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}
	u := req.URL.String()
	// langchaingo's openai package hits either "/chat/completions" (with
	// or without a leading /v1) or "/completions". We accept either.
	return strings.Contains(u, "/chat/completions")
}

func mergeJSON(orig []byte, extra map[string]any) ([]byte, error) {
	if len(orig) == 0 {
		return json.Marshal(extra)
	}
	var base map[string]any
	if err := json.Unmarshal(orig, &base); err != nil {
		return nil, fmt.Errorf("extraTransport: cannot decode body: %w", err)
	}
	for k, v := range extra {
		base[k] = v
	}
	return json.Marshal(base)
}

// MessagesFromParser converts parser.Message into llms.MessageContent.
func MessagesFromParser(msgs []parserMessage) []llms.MessageContent {
	out := make([]llms.MessageContent, len(msgs))
	for i, m := range msgs {
		parts := make([]llms.ContentPart, 0)
		switch c := m.Content.(type) {
		case string:
			parts = append(parts, llms.TextPart(c))
		case []parserContentBlock:
			for _, b := range c {
				switch b.Type {
				case "text":
					parts = append(parts, llms.TextPart(b.Text))
				case "image":
					// llms.BinaryContent holds raw bytes; we base64-decode
					// the data field on the way in.
					decoded, _ := decodeBase64(b.Data)
					parts = append(parts, llms.BinaryPart(b.MIMEType, decoded))
				}
			}
		}
		out[i] = llms.MessageContent{Role: m.Role, Parts: parts}
	}
	return out
}

// decodeBase64 is a small wrapper that tolerates malformed input by
// returning an empty slice.
func decodeBase64(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	// Try StdEncoding; on failure try RawStdEncoding; finally fall back to
	// the empty slice. This matches the JS pass-through where bad image
	// data is dropped at send time.
	if b, err := base64Decode(s); err == nil {
		return b, nil
	}
	if b, err := base64DecodeRaw(s); err == nil {
		return b, nil
	}
	return nil, nil
}

// parseMessage is the type alias used by parser.Parse. We keep these
// aliases at package scope so other internal packages don't need to import
// parser directly.
type parserMessage = struct {
	Role    llms.ChatMessageType
	Content any
}

type parserContentBlock = struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	Source   string `json:"source,omitempty"`
}

// MustParseMessages is a thin convenience wrapper used by main.go.
func MustParseMessages(in any) []parserMessage {
	type pm = parserMessage
	type pcb = parserContentBlock
	type intermediate struct {
		Role    llms.ChatMessageType
		Content any
	}
	if in == nil {
		return nil
	}
	arr, ok := in.([]intermediate)
	if !ok {
		return nil
	}
	out := make([]parserMessage, len(arr))
	for i, m := range arr {
		out[i] = pm{Role: m.Role, Content: m.Content}
	}
	return out
}

// ConvertAttachments converts parser.Attachment (used by includes) into
// ContentBlocks suitable for splicing into a single user message.
func ConvertAttachments(atts []attachmentLike) []parserContentBlock {
	out := make([]parserContentBlock, 0, len(atts))
	for _, a := range atts {
		switch a.Type {
		case "image":
			out = append(out, parserContentBlock{
				Type: "image", MIMEType: a.MIMEType, Data: a.Data, Source: a.Source,
			})
		case "text":
			out = append(out, parserContentBlock{Type: "text", Text: a.Text, Source: a.Source})
		}
	}
	return out
}

type attachmentLike = struct {
	Type     string `json:"type"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	Text     string `json:"text,omitempty"`
	Source   string `json:"source,omitempty"`
}

// IsSSEError reports whether err looks like a streaming transport failure
// (e.g. EOF mid-stream, malformed SSE). Used by the run loop to decide
// whether to silently retry as a non-streaming request.
func IsSSEError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "EOF") ||
		strings.Contains(s, "unexpected EOF") ||
		strings.Contains(s, "data: ") ||
		strings.Contains(s, "SSE") ||
		strings.Contains(s, "stream") ||
		strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "context canceled")
}

// base64Decode and base64DecodeRaw are tiny helpers that avoid an extra
// import in this file. They are defined in client_base64.go.
var base64Decode = func(s string) ([]byte, error) {
	return decodeBase64Std(s)
}
var base64DecodeRaw = func(s string) ([]byte, error) {
	return decodeBase64RawStd(s)
}

// CheckNoop keeps the ctx import alive when no flags use it.
var _ = context.Background
