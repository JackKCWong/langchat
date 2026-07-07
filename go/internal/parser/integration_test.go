package parser_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JackKCWong/langchat/go/internal/frontmatter"
	"github.com/JackKCWong/langchat/go/internal/parser"
	"github.com/tmc/langchaingo/llms"
)

func TestFrontmatterStripsAndParserSeesBody(t *testing.T) {
	md := strings.Join([]string{
		"---",
		"model: qwen-vl-plus",
		"temperature: 0.3",
		"---",
		"# !system",
		"",
		"You are a help assistant.",
		"",
		"# !user",
		"",
		"Hi.",
		"",
	}, "\n")
	fm, err := frontmatter.Parse(md)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := map[string]any{"model": "qwen-vl-plus", "temperature": 0.3}
	if !reflect.DeepEqual(fm.Opts, want) {
		t.Errorf("opts = %v, want %v", fm.Opts, want)
	}
	if fm.HeaderLines != 4 {
		t.Errorf("headerLines = %d, want 4", fm.HeaderLines)
	}
	res, err := parser.Parse(fm.Body, nil, parser.ParseOptions{LineOffset: fm.HeaderLines})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.OutputSchema != nil {
		t.Errorf("schema = %v, want nil", res.OutputSchema)
	}
	if len(res.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(res.Messages))
	}
	if res.Messages[1].Role != llms.ChatMessageTypeHuman || res.Messages[1].Content != "Hi." {
		t.Errorf("msg[1] = %+v", res.Messages[1])
	}
}

func TestFrontmatterParserErrorLineNumbersReferToOriginalFile(t *testing.T) {
	md := strings.Join([]string{
		"---",
		"model: x",
		"---",
		"# !system",
		"",
		"sys",
		"",
		"# !foo",
		"",
		"bar",
		"",
	}, "\n")
	fm, err := frontmatter.Parse(md)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err = parser.Parse(fm.Body, nil, parser.ParseOptions{LineOffset: fm.HeaderLines})
	if err == nil || !strings.Contains(err.Error(), `Unknown role "# !foo" at line 8`) {
		t.Errorf("err = %v", err)
	}
}

func TestFrontmatterParserOutputErrorUsesOriginalLineNumbers(t *testing.T) {
	md := strings.Join([]string{
		"---",
		"model: x",
		"---",
		"# !user",
		"",
		"hi",
		"",
		"# !output",
		"",
		"",
	}, "\n")
	fm, err := frontmatter.Parse(md)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	_, err = parser.Parse(fm.Body, nil, parser.ParseOptions{LineOffset: fm.HeaderLines})
	if err == nil || !strings.Contains(err.Error(), "# !output block at line 8 is empty") {
		t.Errorf("err = %v", err)
	}
}

func TestFrontmatterParserNoFrontmatter(t *testing.T) {
	md := strings.Join([]string{"# !user", "", "hi", ""}, "\n")
	fm, err := frontmatter.Parse(md)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if fm.HeaderLines != 0 || fm.Body != md {
		t.Errorf("headerLines = %d, body mismatch", fm.HeaderLines)
	}
	res, err := parser.Parse(fm.Body, nil, parser.ParseOptions{LineOffset: fm.HeaderLines})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Messages) != 1 || res.Messages[0].Content != "hi" {
		t.Errorf("messages = %+v", res.Messages)
	}
}

func TestFrontmatterParserMVP3Example(t *testing.T) {
	md := strings.Join([]string{
		"---",
		"model: qwen-vl-plus",
		"---",
		"",
		"# !system",
		"",
		"You are a help assistant.",
		"",
		"# !user",
		"",
		"Answer my questions based on the below context:",
		"",
		`{{ include "Goku.png" }}`,
		"",
		"# !user",
		"",
		"What is Sun Goku saying?",
		"",
	}, "\n")
	fm, err := frontmatter.Parse(md)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(fm.Opts, map[string]any{"model": "qwen-vl-plus"}) {
		t.Errorf("opts = %v", fm.Opts)
	}
	att := []parser.Attachment{{Type: "image", MIMEType: "image/png", Data: "A", Source: "Goku.png"}}
	res, err := parser.Parse(fm.Body, att, parser.ParseOptions{LineOffset: fm.HeaderLines})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.OutputSchema != nil {
		t.Errorf("schema = %v", res.OutputSchema)
	}
	if len(res.Messages) != 3 {
		t.Errorf("messages len = %d, want 3", len(res.Messages))
	}
	if _, ok := res.Messages[1].Content.([]parser.ContentBlock); !ok {
		t.Errorf("msg[1] should be content blocks, got %T", res.Messages[1].Content)
	}
}