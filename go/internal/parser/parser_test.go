package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestParsesSingleSystemBlock(t *testing.T) {
	md := "# !system\n\nYou are a helpful assistant.\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(res.Messages))
	}
	if res.Messages[0].Role != llms.ChatMessageTypeSystem {
		t.Errorf("role = %v, want system", res.Messages[0].Role)
	}
	if res.Messages[0].Content != "You are a helpful assistant." {
		t.Errorf("content = %v", res.Messages[0].Content)
	}
}

func TestParsesSystemThenUser(t *testing.T) {
	md := "# !system\n\nYou are a help assistant.\n\n# !user\n\nWhat's the weather like today.\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(res.Messages))
	}
	if res.Messages[0].Role != llms.ChatMessageTypeSystem {
		t.Errorf("msg[0] role = %v", res.Messages[0].Role)
	}
	if res.Messages[1].Role != llms.ChatMessageTypeHuman {
		t.Errorf("msg[1] role = %v", res.Messages[1].Role)
	}
	if res.Messages[1].Content != "What's the weather like today." {
		t.Errorf("msg[1] content = %v", res.Messages[1].Content)
	}
}

func TestMultipleUserBlocksInSourceOrder(t *testing.T) {
	md := "# !user\n\nFirst message.\n\n# !assistant\n\nReply one.\n\n# !user\n\nSecond message.\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Messages) != 3 {
		t.Fatalf("messages len = %d", len(res.Messages))
	}
	if res.Messages[0].Role != llms.ChatMessageTypeHuman || res.Messages[0].Content != "First message." {
		t.Errorf("msg[0] = %v", res.Messages[0])
	}
	if res.Messages[1].Role != llms.ChatMessageTypeAI || res.Messages[1].Content != "Reply one." {
		t.Errorf("msg[1] = %v", res.Messages[1])
	}
	if res.Messages[2].Role != llms.ChatMessageTypeHuman || res.Messages[2].Content != "Second message." {
		t.Errorf("msg[2] = %v", res.Messages[2])
	}
}

func TestEmitsEmptyBlocksAsEmptyMessages(t *testing.T) {
	md := "# !system\n\n\n# !user\n\nHello.\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Messages[0].Content != "" {
		t.Errorf("msg[0] content = %v, want empty", res.Messages[0].Content)
	}
	if res.Messages[1].Content != "Hello." {
		t.Errorf("msg[1] content = %v", res.Messages[1].Content)
	}
}

func TestPreservesMultiLineContent(t *testing.T) {
	md := "# !user\n\nline one\nline two\nline three\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Messages[0].Content != "line one\nline two\nline three" {
		t.Errorf("content = %v", res.Messages[0].Content)
	}
}

func TestHeadersWithSurroundingWhitespace(t *testing.T) {
	md := "# !user   \n\nHi.\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Messages[0].Content != "Hi." {
		t.Errorf("content = %v", res.Messages[0].Content)
	}
}

func TestUnknownRoleThrowsWithLine(t *testing.T) {
	md := "# !foo\n\nbar\n"
	_, err := Parse(md, nil)
	if err == nil || !strings.Contains(err.Error(), `Unknown role "# !foo" at line 1`) {
		t.Errorf("err = %v", err)
	}
}

func TestContentBeforeAnyHeaderThrows(t *testing.T) {
	md := "hello world\n"
	_, err := Parse(md, nil)
	if err == nil || !strings.Contains(err.Error(), "Unexpected content at line 1") {
		t.Errorf("err = %v", err)
	}
}

func TestNoMessagesThrows(t *testing.T) {
	for _, in := range []string{"", "\n\n\n"} {
		if _, err := Parse(in, nil); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestUserWithOneImageDirectiveBecomesContentArray(t *testing.T) {
	md := `# !user

Look: {{ include "a.png" }}
`
	att := []Attachment{{Type: "image", MIMEType: "image/png", Data: "AAAA", Source: "a.png"}}
	res, err := Parse(md, att)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	blocks, ok := res.Messages[0].Content.([]ContentBlock)
	if !ok {
		t.Fatalf("content type = %T, want []ContentBlock", res.Messages[0].Content)
	}
	want := []ContentBlock{
		{Type: "text", Text: "Look: "},
		{Type: "image", MIMEType: "image/png", Data: "AAAA", Source: "a.png"},
	}
	if !reflect.DeepEqual(blocks, want) {
		t.Errorf("blocks = %+v", blocks)
	}
}

func TestTextOnlyUserStaysStringWithAttachments(t *testing.T) {
	md := "# !user\n\njust text, no directive\n# !user\n\nalso just text\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	for i, m := range res.Messages {
		if _, ok := m.Content.(string); !ok {
			t.Errorf("msg[%d] content = %T, want string", i, m.Content)
		}
	}
}

func TestImageDirectiveInSystemBlockThrows(t *testing.T) {
	md := `# !system

You see {{ include "a.png" }}
`
	att := []Attachment{{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"}}
	_, err := Parse(md, att)
	if err == nil || !strings.Contains(err.Error(), "image include") || !strings.Contains(err.Error(), "only supported") {
		t.Errorf("err = %v", err)
	}
}

func TestImageDirectiveInAssistantBlockThrows(t *testing.T) {
	md := `# !assistant

I see {{ include "a.png" }}
`
	att := []Attachment{{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"}}
	_, err := Parse(md, att)
	if err == nil || !strings.Contains(err.Error(), "image include") {
		t.Errorf("err = %v", err)
	}
}

func TestDirectivesExceedAttachments(t *testing.T) {
	md := `# !user

{{ include "a.png" }} and {{ include "b.jpg" }}
`
	att := []Attachment{{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"}}
	_, err := Parse(md, att)
	if err == nil || !strings.Contains(err.Error(), "directives exceed attachments") {
		t.Errorf("err = %v", err)
	}
}

func TestAttachmentsExceedDirectives(t *testing.T) {
	md := `# !user

plain text no directive
`
	att := []Attachment{
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
	}
	_, err := Parse(md, att)
	if err == nil || !strings.Contains(err.Error(), "attachment count mismatch") {
		t.Errorf("err = %v", err)
	}
}

func TestMultipleImagesPreserveDirectiveOrder(t *testing.T) {
	md := `# !user

first {{ include "a.png" }} second {{ include "b.jpg" }} end
`
	att := []Attachment{
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
	}
	res, err := Parse(md, att)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []ContentBlock{
		{Type: "text", Text: "first "},
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
		{Type: "text", Text: " second "},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
		{Type: "text", Text: " end"},
	}
	if !reflect.DeepEqual(res.Messages[0].Content, want) {
		t.Errorf("blocks = %+v", res.Messages[0].Content)
	}
}

func TestMultipleIncludesAcrossBlocksConsumeInOrder(t *testing.T) {
	md := `# !user

first {{ include "a.png" }}
# !assistant

ok
# !user

then {{ include "b.jpg" }} and {{ include "c.gif" }}
`
	att := []Attachment{
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
		{Type: "image", MIMEType: "image/gif", Data: "C", Source: "c.gif"},
	}
	res, err := Parse(md, att)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Messages) != 3 {
		t.Fatalf("messages len = %d", len(res.Messages))
	}
	if blocks, _ := res.Messages[0].Content.([]ContentBlock); !reflect.DeepEqual(blocks, []ContentBlock{
		{Type: "text", Text: "first "},
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
	}) {
		t.Errorf("msg[0] = %+v", res.Messages[0].Content)
	}
	if res.Messages[1].Content != "ok" {
		t.Errorf("msg[1] = %v", res.Messages[1].Content)
	}
	if blocks, _ := res.Messages[2].Content.([]ContentBlock); !reflect.DeepEqual(blocks, []ContentBlock{
		{Type: "text", Text: "then "},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
		{Type: "text", Text: " and "},
		{Type: "image", MIMEType: "image/gif", Data: "C", Source: "c.gif"},
	}) {
		t.Errorf("msg[2] = %+v", res.Messages[2].Content)
	}
}

func TestSingleUserMessageMixesTextAndImages(t *testing.T) {
	md := `# !user

Compare these:
1) {{ include "a.png" }}
2) {{ include "b.jpg" }}
3) {{ include "c.webp" }}
Which differ?
`
	att := []Attachment{
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
		{Type: "image", MIMEType: "image/webp", Data: "C", Source: "c.webp"},
	}
	res, err := Parse(md, att)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	blocks := res.Messages[0].Content.([]ContentBlock)
	got := blocks
	want := []ContentBlock{
		{Type: "text", Text: "Compare these:\n1) "},
		{Type: "image", MIMEType: "image/png", Data: "A", Source: "a.png"},
		{Type: "text", Text: "\n2) "},
		{Type: "image", MIMEType: "image/jpeg", Data: "B", Source: "b.jpg"},
		{Type: "text", Text: "\n3) "},
		{Type: "image", MIMEType: "image/webp", Data: "C", Source: "c.webp"},
		{Type: "text", Text: "\nWhich differ?"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("blocks = %+v", got)
	}
}

func TestMixesTextAttachmentsAndImages(t *testing.T) {
	md := `# !user

Log: {{ include "run.log" }}
Screenshot: {{ include "shot.png" }}
Config: {{ include "config.yaml" }}
`
	att := []Attachment{
		{Type: "text", Text: "INFO boot ok", Source: "run.log"},
		{Type: "image", MIMEType: "image/png", Data: "SHOT", Source: "shot.png"},
		{Type: "text", Text: "port: 8080", Source: "config.yaml"},
	}
	res, err := Parse(md, att)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	blocks := res.Messages[0].Content.([]ContentBlock)
	want := []ContentBlock{
		{Type: "text", Text: "Log: "},
		{Type: "text", Text: "INFO boot ok", Source: "run.log"},
		{Type: "text", Text: "\nScreenshot: "},
		{Type: "image", MIMEType: "image/png", Data: "SHOT", Source: "shot.png"},
		{Type: "text", Text: "\nConfig: "},
		{Type: "text", Text: "port: 8080", Source: "config.yaml"},
	}
	if !reflect.DeepEqual(blocks, want) {
		t.Errorf("blocks = %+v", blocks)
	}
}

func TestNoOutputBlockSchemaIsNil(t *testing.T) {
	md := "# !system\n\nsys\n# !user\n\nhi\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.OutputSchema != nil {
		t.Errorf("schema = %v, want nil", res.OutputSchema)
	}
	if len(res.Messages) != 2 {
		t.Errorf("messages len = %d, want 2", len(res.Messages))
	}
}

func TestFencedJSONOutput(t *testing.T) {
	md := "# !system\n\nsys\n# !user\n\nhi\n# !output\n\n```json\n{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"}}}\n```\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(res.Messages) != 2 {
		t.Errorf("messages len = %d, want 2", len(res.Messages))
	}
	want := map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}
	if !reflect.DeepEqual(res.OutputSchema, want) {
		t.Errorf("schema = %v", res.OutputSchema)
	}
}

func TestPlainUnfencedJSONOutput(t *testing.T) {
	md := "# !user\n\nhi\n# !output\n\n{\"type\":\"object\"}\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(res.OutputSchema, map[string]any{"type": "object"}) {
		t.Errorf("schema = %v", res.OutputSchema)
	}
}

func TestFenceWithoutLanguage(t *testing.T) {
	md := "# !user\n\nhi\n# !output\n\n```\n{\"a\":1}\n```\n"
	res, err := Parse(md, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !reflect.DeepEqual(res.OutputSchema, map[string]any{"a": float64(1)}) {
		t.Errorf("schema = %v", res.OutputSchema)
	}
}

func TestMultipleOutputBlocksThrow(t *testing.T) {
	md := "# !user\n\nhi\n# !output\n\n{\"a\":1}\n# !output\n\n{\"b\":2}\n"
	_, err := Parse(md, nil)
	if err == nil || !strings.Contains(err.Error(), "duplicate # !output block") {
		t.Errorf("err = %v", err)
	}
}

func TestEmptyOutputBlockThrows(t *testing.T) {
	md := "# !user\n\nhi\n# !output\n\n\n"
	_, err := Parse(md, nil)
	if err == nil || !strings.Contains(err.Error(), "# !output block at line 4 is empty") {
		t.Errorf("err = %v", err)
	}
}

func TestInvalidJSONInOutput(t *testing.T) {
	md := "# !user\n\nhi\n# !output\n\n```json\n{ not json }\n```\n"
	_, err := Parse(md, nil)
	if err == nil || !strings.Contains(err.Error(), "# !output block at line 4 is not valid JSON") {
		t.Errorf("err = %v", err)
	}
}

func TestOutputOnlyFileStillRequiresMessages(t *testing.T) {
	md := "# !output\n\n{\"type\":\"object\"}\n"
	_, err := Parse(md, nil)
	if err == nil || !strings.Contains(err.Error(), "No messages found") {
		t.Errorf("err = %v", err)
	}
}