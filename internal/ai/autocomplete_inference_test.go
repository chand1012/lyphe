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

func TestAutocompleteUsesInstructionPrefill(t *testing.T) {
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
		if payload["prompt"] != autocompleteInput("I need") || payload["temperature"] != float64(0) || payload["n_predict"] != float64(32) || payload["repeat_penalty"] != 1.05 {
			t.Errorf("wrong inference settings: %+v", payload)
		}
		if _, ok := payload["messages"]; ok {
			t.Error("native completion received chat messages")
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
		t.Skip("set LYPHE_SMOLLM_TEST=1 with the local 1.7B instruction model")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	c := DefaultConfig()
	c.Address = address
	c.Llamafile, err = filepath.Abs("../../models/smollm2-1.7b.llamafile")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir()) // Keep the model's generated logs out of the source tree.
	m := New(nil, c)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	for _, sample := range []struct{ prompt, keywords string }{
		{"I have been working too much and I need", "break|rest|recharge|time"},
		{"I went for a walk and it helped me", "mind"},
		{"We agreed to keep the scope small so that", "focus|manage"},
	} {
		started := time.Now()
		raw, err := m.Autocomplete(ctx, sample.prompt)
		if err != nil {
			t.Fatal(err)
		}
		text := trimAutocompleteInsertion(sample.prompt, raw)
		relevant := false
		for _, keyword := range strings.Split(sample.keywords, "|") {
			relevant = relevant || strings.Contains(strings.ToLower(text), keyword)
		}
		if !relevant || len(strings.Fields(text)) > 20 {
			t.Errorf("%q: unhelpful continuation %q", sample.prompt, text)
		}
		if !strings.HasPrefix(text, " ") {
			t.Errorf("missing insertion space: %q", text)
		}
		if time.Since(started) > 8*time.Second {
			t.Error("completion exceeded the request deadline")
		}
		t.Logf("%s%s (%s)", sample.prompt, text, time.Since(started))
	}
}
