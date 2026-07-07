package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JackKCWong/langchat-go/internal/args"
)

// TestRunEndToEnd mocks an OpenAI-compatible /v1/chat/completions server
// that returns a single SSE-style chunk and verifies the CLI streams the
// response to stdout and (optionally) the output file.
func TestRunEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		chunk := `data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"fake","choices":[{"index":0,"delta":{"role":"assistant","content":"hello world"},"finish_reason":null}]}` + "\n\n"
		w.Write([]byte(chunk))
		if flusher != nil {
			flusher.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat.md")
	if err := os.WriteFile(chatPath, []byte("# !user\n\nhi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("LANGCHAT_MODEL", "fake")
	t.Setenv("LANGCHAT_BASE_URL", srv.URL)
	t.Setenv("LANGCHAT_API_KEY", "test-key")

	// capture stdout
	stdoutR, stdoutW, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = stdoutW

	outPath := filepath.Join(dir, "out.txt")
	err := run(args.Options{File: chatPath, Output: outPath})
	stdoutW.Close()
	os.Stdout = oldStdout

	var got bytes.Buffer
	got.ReadFrom(stdoutR)

	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if !strings.Contains(got.String(), "hello world") {
		t.Errorf("stdout missing 'hello world': %q", got.String())
	}
	body, _ := os.ReadFile(outPath)
	if !strings.Contains(string(body), "hello world") {
		t.Errorf("output file missing 'hello world': %q", body)
	}
}

// TestRunStructuredEndToEnd exercises the structured-output branch by
// mocking the server to return a JSON object matching the schema.
func TestRunStructuredEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{
			"id":      "x",
			"object":  "chat.completion",
			"created": 1,
			"model":   "fake",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": `{"name":"Sun Goku","signature_attack":"Kamehameha"}`,
				},
			}},
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat.md")
	md := `# !user

who?
# !output

{
  "type": "object",
  "properties": {
    "name":             { "type": "string" },
    "signature_attack": { "type": "string" }
  },
  "required": ["name", "signature_attack"]
}
`
	if err := os.WriteFile(chatPath, []byte(md), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("LANGCHAT_MODEL", "fake")
	t.Setenv("LANGCHAT_BASE_URL", srv.URL)
	t.Setenv("LANGCHAT_API_KEY", "test-key")

	stdoutR, stdoutW, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = stdoutW

	err := run(args.Options{File: chatPath})
	stdoutW.Close()
	os.Stdout = oldStdout
	var got bytes.Buffer
	got.ReadFrom(stdoutR)

	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if !strings.Contains(got.String(), `"name": "Sun Goku"`) {
		t.Errorf("stdout missing JSON result: %q", got.String())
	}
	if !strings.Contains(got.String(), `"signature_attack": "Kamehameha"`) {
		t.Errorf("stdout missing JSON result: %q", got.String())
	}
}

// TestRunImageAttachmentUsesImageURL verifies the outgoing chat request
// body uses the image_url content type (not binary). OpenAI-compatible
// servers reject {"type":"binary",...}; langchaingo's BinaryContent
// serializes that way, so langchat must use ImageURLContent with a data
// URI instead.
func TestRunImageAttachmentUsesImageURL(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := readAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "ok"},
			}},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "img.png")
	os.WriteFile(imgPath, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, 0o644)
	chatPath := filepath.Join(dir, "chat.md")
	os.WriteFile(chatPath, []byte("# !user\n\nsee {{ include \"img.png\" }}\n"), 0o644)

	t.Setenv("LANGCHAT_MODEL", "fake")
	t.Setenv("LANGCHAT_BASE_URL", srv.URL)
	t.Setenv("LANGCHAT_API_KEY", "test-key")

	stdoutR, stdoutW, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = stdoutW
	err := run(args.Options{File: chatPath})
	stdoutW.Close()
	os.Stdout = oldStdout
	stdoutR.Read(make([]byte, 1024))

	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	msgs, _ := captured["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatalf("no messages in captured body: %+v", captured)
	}
	content, _ := msgs[0].(map[string]any)["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content parts: %+v", msgs[0])
	}
	last := content[len(content)-1].(map[string]any)
	if last["type"] != "image_url" {
		t.Errorf("image content type = %v, want image_url (full body: %+v)", last["type"], content)
	}
	if _, ok := last["image_url"]; !ok {
		t.Errorf("image_url field missing: %+v", last)
	}
}

// TestRunFallsBackWhenStreamingFails verifies the silent non-streaming
// fallback path: a server that returns HTTP 500 on streaming requests
// triggers a retry as a plain non-streaming request.
func TestRunFallsBackWhenStreamingFails(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := readAll(r.Body)
		var req struct {
			Stream bool `json:"stream"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Stream {
			http.Error(w, "synthetic stream failure", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		respBody := map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "fallback answer"},
			}},
		}
		json.NewEncoder(w).Encode(respBody)
	}))
	defer srv.Close()

	dir := t.TempDir()
	chatPath := filepath.Join(dir, "chat.md")
	if err := os.WriteFile(chatPath, []byte("# !user\n\nhi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Setenv("LANGCHAT_MODEL", "fake")
	t.Setenv("LANGCHAT_BASE_URL", srv.URL)
	t.Setenv("LANGCHAT_API_KEY", "test-key")

	stdoutR, stdoutW, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = stdoutW

	err := run(args.Options{File: chatPath})
	stdoutW.Close()
	os.Stdout = oldStdout
	var got bytes.Buffer
	got.ReadFrom(stdoutR)

	if err != nil {
		t.Fatalf("run err: %v", err)
	}
	if !strings.Contains(got.String(), "fallback answer") {
		t.Errorf("stdout missing fallback content: %q", got.String())
	}
	if calls < 2 {
		t.Errorf("expected at least 2 server calls (stream + fallback), got %d", calls)
	}
}

func readAll(r interface{ Read(p []byte) (int, error) }) ([]byte, error) {
	var out []byte
	buf := make([]byte, 1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return out, nil
			}
			return out, err
		}
	}
}
