// Package parser parses a chat-file body (post frontmatter) into a list of
// messages and an optional JSON-schema output block. Each message has a role
// (system / user / assistant) and a body that is either a plain string (text
// only) or a []ContentBlock interleaving text and image attachments.
//
// Image includes are tracked via the attachments slice: as the parser
// encounters {{ include "..." }} directives inside a # !user block, it
// consumes the next attachment in order. The parser errors if directives
// and attachments don't line up.
//
// Mirrors src/parser.js.
package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleOutput    Role = "output"
)

// Attachment represents a single attachment recorded by includes.Resolve.
// Only image attachments are spliced into user messages; text attachments
// (rare) are also passed through as content blocks when present.
type Attachment struct {
	Type     string `json:"type"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64-encoded for images
	Text     string `json:"text,omitempty"`  // for text attachments
	Source   string `json:"source,omitempty"`
}

// ContentBlock is a single part of a multimodal user message.
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64-encoded
	Source   string `json:"source,omitempty"`
}

// Message holds a parsed message. Content is either a plain string or a
// slice of ContentBlock (for multimodal user messages).
type Message struct {
	Role    llms.ChatMessageType
	Content any
}

// ParseResult is what Parse returns.
type ParseResult struct {
	Messages     []Message
	OutputSchema map[string]any
}

var (
	headerRE    = regexp.MustCompile(`^# !(system|user|assistant|output)\s*$`)
	directiveRE = regexp.MustCompile(`\{\{\s*include\s+"([^"]+)"\s*\}\}`)
	knownRoles  = map[string]Role{"system": RoleSystem, "user": RoleUser, "assistant": RoleAssistant, "output": RoleOutput}
)

// ParseOptions is the optional argument to Parse.
type ParseOptions struct {
	LineOffset int // number of lines the body was shifted by (frontmatter header)
}

// Parse splits text into messages + an optional outputSchema.
func Parse(text string, attachments []Attachment, opts ...ParseOptions) (ParseResult, error) {
	if text == "" && len(attachments) == 0 {
		return ParseResult{}, fmt.Errorf("Parse expects a non-empty string")
	}
	o := ParseOptions{}
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.LineOffset < 0 {
		return ParseResult{}, fmt.Errorf("lineOffset must be >= 0")
	}

	lines := strings.Split(text, "\n")
	var messages []Message
	currentRole := Role("")
	var currentLines []string
	currentHeaderLine := -1
	attachmentIdx := 0

	var outputRaw string
	outputLine := -1
	outputSeen := false

	flush := func() error {
		if currentRole == "" {
			return nil
		}
		raw := strings.Trim(strings.Join(currentLines, "\n"), "\n")
		if currentRole == RoleOutput {
			if outputSeen {
				headerLine := currentHeaderLine
				if headerLine < 1 {
					headerLine = 1
				}
				return fmt.Errorf("duplicate # !output block at line %d; only one # !output block is allowed per chat file.", headerLine)
			}
			outputSeen = true
			outputRaw = raw
			outputLine = currentHeaderLine
			currentRole = ""
			currentLines = nil
			currentHeaderLine = -1
			return nil
		}
		content, nextIdx, err := expandUserContent(raw, attachments, currentRole, currentHeaderLine, attachmentIdx)
		if err != nil {
			return err
		}
		attachmentIdx = nextIdx
		messages = append(messages, Message{Role: roleToChatType(currentRole), Content: content})
		currentRole = ""
		currentLines = nil
		currentHeaderLine = -1
		return nil
	}

	for idx, line := range lines {
		lineNumber := idx + 1 + o.LineOffset
		if m := headerRE.FindStringSubmatch(line); m != nil {
			if err := flush(); err != nil {
				return ParseResult{}, err
			}
			currentRole = knownRoles[m[1]]
			currentHeaderLine = lineNumber
			continue
		}
		if strings.HasPrefix(line, "# !") {
			rest := strings.TrimSpace(line[3:])
			token := strings.SplitN(rest, " ", 2)[0]
			return ParseResult{}, fmt.Errorf(`Unknown role "# !%s" at line %d. Expected one of: system, user, assistant, output.`, token, lineNumber)
		}
		if currentRole == "" {
			if strings.TrimSpace(line) == "" {
				continue
			}
			return ParseResult{}, fmt.Errorf("Unexpected content at line %d: messages must begin with a \"# !<role>\" header.", lineNumber)
		}
		currentLines = append(currentLines, line)
	}
	if err := flush(); err != nil {
		return ParseResult{}, err
	}

	if attachmentIdx != len(attachments) {
		return ParseResult{}, fmt.Errorf("attachment count mismatch: %d image directive(s) consumed but %d attachment(s) provided", attachmentIdx, len(attachments))
	}
	if len(messages) == 0 {
		return ParseResult{}, fmt.Errorf("No messages found. The file must contain at least one \"# !system\", \"# !user\", or \"# !assistant\" block.")
	}

	var schema map[string]any
	if outputSeen {
		s, err := parseOutputSchema(outputRaw, outputLine)
		if err != nil {
			return ParseResult{}, err
		}
		schema = s
	}
	return ParseResult{Messages: messages, OutputSchema: schema}, nil
}

// expandUserContent returns either a string or a []ContentBlock for the
// given raw block content. Image directives consume attachments in order.
// Text attachments (rare) are spliced in as text blocks.
func expandUserContent(raw string, attachments []Attachment, role Role, lineNumber, startIdx int) (any, int, error) {
	matches := directiveRE.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return raw, startIdx, nil
	}
	if role != RoleUser {
		snippet := ""
		if m := directiveRE.FindString(raw); m != "" {
			snippet = m
		}
		return nil, startIdx, fmt.Errorf(`image include %s at line %d is only supported inside a # !user block (found # !%s)`, snippet, lineNumber, role)
	}

	var blocks []ContentBlock
	cursor := 0
	idx := startIdx
	for _, m := range matches {
		offset, end := m[0], m[1]
		if offset > cursor {
			blocks = append(blocks, ContentBlock{Type: "text", Text: raw[cursor:offset]})
		}
		if idx >= len(attachments) {
			matchText := raw[offset:end]
			return nil, startIdx, fmt.Errorf("image include %s at line %d has no remaining attachment (directives exceed attachments)", matchText, lineNumber)
		}
		a := attachments[idx]
		switch a.Type {
		case "image":
			blocks = append(blocks, ContentBlock{
				Type:     "image",
				MIMEType: a.MIMEType,
				Data:     a.Data,
				Source:   a.Source,
			})
		case "text":
			blocks = append(blocks, ContentBlock{Type: "text", Text: a.Text, Source: a.Source})
		default:
			blocks = append(blocks, ContentBlock{Type: a.Type, MIMEType: a.MIMEType, Data: a.Data, Source: a.Source})
		}
		idx++
		cursor = end
	}
	if cursor < len(raw) {
		blocks = append(blocks, ContentBlock{Type: "text", Text: raw[cursor:]})
	}
	return blocks, idx, nil
}

func roleToChatType(r Role) llms.ChatMessageType {
	switch r {
	case RoleSystem:
		return llms.ChatMessageTypeSystem
	case RoleAssistant:
		return llms.ChatMessageTypeAI
	default:
		return llms.ChatMessageTypeHuman
	}
}

func parseOutputSchema(raw string, lineNumber int) (map[string]any, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, fmt.Errorf("# !output block at line %d is empty", lineNumber)
	}
	text = stripCodeFence(text)
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("# !output block at line %d is empty", lineNumber)
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(text), &schema); err != nil {
		return nil, fmt.Errorf("# !output block at line %d is not valid JSON: %v", lineNumber, err)
	}
	return schema, nil
}

var fenceRE = regexp.MustCompile("^```[a-zA-Z]*\\s*\\n([\\s\\S]*?)\\n```\\s*$")

func stripCodeFence(text string) string {
	m := fenceRE.FindStringSubmatch(text)
	if m == nil {
		return text
	}
	return m[1]
}

// AttachmentFromImage builds an image attachment from raw bytes and a source label.
func AttachmentFromImage(mimeType string, data []byte, source string) Attachment {
	return Attachment{Type: "image", MIMEType: mimeType, Data: base64.StdEncoding.EncodeToString(data), Source: source}
}