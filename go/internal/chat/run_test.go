package chat

import (
	"errors"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestExtractContentPrefersContent(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content: "hello",
			GenerationInfo: map[string]any{"Text": "ignored", "Content": "ignored"},
		}},
	}
	if got := extractContent(resp); got != "hello" {
		t.Errorf("got %q, want hello", got)
	}
}

func TestExtractContentFallsBackToText(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			Content:        "",
			GenerationInfo: map[string]any{"Text": "fallback"},
		}},
	}
	if got := extractContent(resp); got != "fallback" {
		t.Errorf("got %q, want fallback", got)
	}
}

func TestExtractContentEmpty(t *testing.T) {
	if got := extractContent(nil); got != "" {
		t.Errorf("nil resp = %q", got)
	}
	if got := extractContent(&llms.ContentResponse{}); got != "" {
		t.Errorf("empty choices = %q", got)
	}
}

func TestIsSSEErrorDetectsStreamingFailures(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("EOF"), true},
		{errors.New("unexpected EOF"), true},
		{errors.New("malformed SSE data: foo"), true},
		{errors.New("context deadline exceeded"), true},
		{errors.New("context canceled"), true},
		{errors.New("regular HTTP 400"), false},
		{errors.New(""), false},
	}
	for _, c := range cases {
		got := IsSSEError(c.err)
		if got != c.want {
			t.Errorf("IsSSEError(%q) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestPrintReasoningWritesDimmed(t *testing.T) {
	// We don't currently capture stdout; just assert it doesn't panic on
	// common shapes.
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{
			GenerationInfo: map[string]any{"ThinkingContent": "deep thought"},
		}},
	}
	PrintReasoning(resp)
}

func TestPrintReasoningEmptyNoOp(t *testing.T) {
	PrintReasoning(nil)
	PrintReasoning(&llms.ContentResponse{})
	PrintReasoning(&llms.ContentResponse{Choices: []*llms.ContentChoice{{GenerationInfo: map[string]any{"ThinkingContent": ""}}}})
}

func TestConvertSchemaSimpleObject(t *testing.T) {
	root := convertSchema(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	})
	if root.Type != "object" {
		t.Errorf("type = %q", root.Type)
	}
	if root.Properties["name"].Type != "string" {
		t.Errorf("name.type = %q", root.Properties["name"].Type)
	}
	if !containsStr(root.Required, "name") {
		t.Errorf("required = %v", root.Required)
	}
}

func TestConvertSchemaNestedArray(t *testing.T) {
	root := convertSchema(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	})
	if root.Properties["tags"].Items == nil || root.Properties["tags"].Items.Type != "string" {
		t.Errorf("tags.items not converted: %+v", root.Properties["tags"])
	}
}

func TestBuildResponseFormat(t *testing.T) {
	rf := buildResponseFormat(map[string]any{"type": "object"})
	if rf.Type != "json_schema" {
		t.Errorf("type = %q", rf.Type)
	}
	if rf.JSONSchema == nil || rf.JSONSchema.Name != "langchat_output" || !rf.JSONSchema.Strict {
		t.Errorf("json_schema not set correctly: %+v", rf.JSONSchema)
	}
}

func containsStr(arr []string, want string) bool {
	for _, s := range arr {
		if s == want {
			return true
		}
	}
	return false
}

func TestConvertSchemaFallbackForNonMap(t *testing.T) {
	root := convertSchema("oops")
	if root == nil || root.Type != "object" || !root.AdditionalProperties {
		t.Errorf("fallback: %+v", root)
	}
}

func TestSSEErrorContainsStreamKeyword(t *testing.T) {
	if !IsSSEError(errors.New("connection reset during stream")) {
		t.Errorf("should detect 'stream'")
	}
	if !strings.Contains(strings.ToLower("hello stream"), "stream") {
		// sanity check the substring logic
		t.Skip()
	}
}