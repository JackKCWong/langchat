package chat

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets a test inject a fake transport that returns a canned
// response, while still exercising the extraTransport's body merge.
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestExtraTransportMergesFieldsIntoBody(t *testing.T) {
	var captured []byte
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		captured, _ = io.ReadAll(req.Body)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Header:     make(http.Header),
		}, nil
	})
	tr := &extraTransport{
		extra: map[string]any{
			"thinking":          true,
			"reasoning_effort": "low",
		},
		base: base,
	}
	req, _ := http.NewRequest("POST", "https://example.com/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o-mini","messages":[]}`))
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp.Body.Close()

	var merged map[string]any
	if err := json.Unmarshal(captured, &merged); err != nil {
		t.Fatalf("unmarshal: %v\nbody=%s", err, captured)
	}
	if merged["model"] != "gpt-4o-mini" {
		t.Errorf("model lost: %v", merged)
	}
	if merged["thinking"] != true {
		t.Errorf("thinking missing: %v", merged)
	}
	if merged["reasoning_effort"] != "low" {
		t.Errorf("reasoning_effort missing: %v", merged)
	}
}

func TestExtraTransportPassesThroughNonChatRequests(t *testing.T) {
	called := false
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})
	tr := &extraTransport{extra: map[string]any{"x": 1}, base: base}
	req, _ := http.NewRequest("GET", "https://example.com/v1/models", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp.Body.Close()
	if !called {
		t.Errorf("expected pass-through")
	}
}

func TestExtraTransportAllowsExtraOverridesExistingFields(t *testing.T) {
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		// assert temperature was overridden to 0.1
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		if m["temperature"] != 0.1 {
			t.Errorf("temperature = %v, want 0.1", m["temperature"])
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
	})
	tr := &extraTransport{extra: map[string]any{"temperature": 0.1}, base: base}
	req, _ := http.NewRequest("POST", "https://example.com/chat/completions",
		strings.NewReader(`{"temperature":0.7}`))
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp.Body.Close()
}

func TestShouldInject(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://x/v1/chat/completions", true},
		{"https://x/chat/completions", true},
		{"https://x/v1/models", false},
		{"https://x/v1/completions", false},
		{"https://x/v1/files", false},
	}
	for _, c := range cases {
		req, _ := http.NewRequest("POST", c.url, nil)
		if got := shouldInject(req); got != c.want {
			t.Errorf("shouldInject(%s) = %v, want %v", c.url, got, c.want)
		}
	}
}