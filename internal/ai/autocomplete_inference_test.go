package ai

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutocompleteUsesNativeCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/completion" {
			t.Errorf("wrong endpoint: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing API key")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["prompt"] != "I need" || payload["temperature"] != float64(0) || payload["n_predict"] != float64(12) || payload["repeat_penalty"] != 1.1 {
			t.Errorf("wrong inference settings: %+v", payload)
		}
		if _, ok := payload["messages"]; ok {
			t.Error("base model received chat messages")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":" to stop."}`))
	}))
	defer server.Close()
	m := New(nil, Config{Address: strings.TrimPrefix(server.URL, "http://")})
	m.ready.Store(true)
	text, err := m.Autocomplete(context.Background(), "I need")
	if err != nil || text != " to stop." {
		t.Fatalf("text %q, error %v", text, err)
	}
}

func TestSmolLMAutocompleteIntegration(t *testing.T) {
	if os.Getenv("LYPHE_SMOLLM_TEST") != "1" {
		t.Skip("set LYPHE_SMOLLM_TEST=1 with the local SmolLM file")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	c := DefaultConfig()
	c.Address = address
	c.Llamafile, err = filepath.Abs("../../models/SmolLM2-135M.Q8_0.llamafile")
	if err != nil {
		t.Fatal(err)
	}
	m := New(nil, c)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	done := make(chan struct{})
	go func() { defer close(done); m.supervise(ctx) }()
	defer func() { cancel(); <-done }()
	for !m.ready.Load() {
		select {
		case <-ctx.Done():
			t.Fatal("SmolLM did not become ready")
		case <-time.After(50 * time.Millisecond):
		}
	}
	for _, sample := range []struct{ prompt, want string }{
		{"I'm feeling burnt out and a bit overwhelmed", "."},
		{"I have been working too much and I need", " to stop."},
	} {
		raw, err := m.Autocomplete(ctx, sample.prompt)
		if err != nil {
			t.Fatal(err)
		}
		text := trimAutocompleteInsertion(sample.prompt, raw)
		if text != sample.want {
			t.Errorf("%q: got %q, want %q", sample.prompt, text, sample.want)
		}
		t.Logf("%s%s", sample.prompt, text)
	}
}
