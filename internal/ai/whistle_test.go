package ai

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWhistleQuietBoundary(t *testing.T) {
	pcm := make([]byte, 35*whistleSampleRate*4)
	for i := 0; i < len(pcm)/4; i++ {
		binary.LittleEndian.PutUint32(pcm[i*4:], math.Float32bits(0.5))
	}
	// One quiet window ends at 27 seconds.
	for i := 27*whistleSampleRate - whistleSampleRate/10; i < 27*whistleSampleRate; i++ {
		binary.LittleEndian.PutUint32(pcm[i*4:], 0)
	}
	if count := splitPCM(pcm); count != 27*whistleSampleRate {
		t.Fatalf("boundary = %d", count)
	}
	if count := splitPCM(pcm[:123*4]); count != 123 {
		t.Fatalf("last chunk = %d", count)
	}
}

func TestWhistleLanguageValidation(t *testing.T) {
	for _, language := range []string{"", "auto", "en", "de", "fr", "es", "it", "nl", "pl"} {
		if !validWhistleLanguage(language) {
			t.Fatalf("rejected %q", language)
		}
	}
	if validWhistleLanguage("ja") {
		t.Fatal("unsupported language accepted")
	}
}

// Opt-in to exercising the real engine, weights, and FFmpeg without downloads.
// LYPHE_WHISTLE_TEST=1 go test ./internal/ai -run TestWhistleIntegration -v
func TestWhistleIntegration(t *testing.T) {
	if os.Getenv("LYPHE_WHISTLE_TEST") != "1" {
		t.Skip("set LYPHE_WHISTLE_TEST=1 with local Whistle assets")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	c := DefaultConfig()
	c.WhistleWASM = filepath.Join(root, "bin/whistle/needle.wasm")
	c.WhistleModel = filepath.Join(root, "bin/whistle/whistle.cact")
	w := &WhistleWASM{Config: c}
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := w.Transcribe(ctx, filepath.Join(root, "frontend/public/audio/sample-voice-note.wav"), "en")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("transcript: %s; language: %s; segments: %+v", result.Text, result.Language, result.Segments)
	if strings.TrimSpace(result.Text) == "" || result.Language != "en" || len(result.Segments) == 0 {
		t.Fatal("missing speech result")
	}

	// Repeat the sample past the engine's limit and verify every chunk is retained.
	long := filepath.Join(t.TempDir(), "long.wav")
	cmd := exec.CommandContext(ctx, c.FFmpeg, "-nostdin", "-v", "error", "-stream_loop", "4", "-i", filepath.Join(root, "frontend/public/audio/sample-voice-note.wav"), long)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	repeated, err := w.Transcribe(ctx, long, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(repeated.Segments) < 2 || strings.Count(strings.ToLower(repeated.Text), "small details") < 4 {
		t.Fatalf("long audio was truncated: %+v", repeated)
	}
	last := repeated.Segments[len(repeated.Segments)-1].(map[string]any)
	if last["start"].(float64) < 25 || last["end"].(float64) < 40 {
		t.Fatalf("bad global chunk times: %+v", last)
	}
	for _, segment := range repeated.Segments {
		chunk := segment.(map[string]any)
		for _, word := range chunk["words"].([]whistleWord) {
			if word.Start < chunk["start"].(float64) || word.End < word.Start {
				t.Fatalf("bad global word times: %+v", word)
			}
		}
	}
	module := w.module
	silence, err := w.transcribeChunk(ctx, make([]byte, whistleSampleRate*4), "auto")
	if err != nil {
		t.Fatal(err)
	}
	if silence.Text != "" {
		t.Fatalf("silence hallucination: %q", silence.Text)
	}
	if module != w.module {
		t.Fatal("model was reloaded between calls")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := w.transcribeChunk(canceled, make([]byte, whistleSampleRate*4), "en"); err == nil {
		t.Fatal("canceled request succeeded")
	}
	// A canceled guest must be recreated for the next request.
	if _, err := w.transcribeChunk(ctx, make([]byte, whistleSampleRate*4), "auto"); err != nil {
		t.Fatalf("recover after cancellation: %v", err)
	}
}

func TestWhistleWebMIntegration(t *testing.T) {
	if os.Getenv("LYPHE_WHISTLE_TEST") != "1" {
		t.Skip("set LYPHE_WHISTLE_TEST=1 with local Whistle assets")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	c := DefaultConfig()
	c.WhistleWASM = filepath.Join(root, "bin/whistle/needle.wasm")
	c.WhistleModel = filepath.Join(root, "bin/whistle/whistle.cact")
	w := &WhistleWASM{Config: c}
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	webm := filepath.Join(t.TempDir(), "recording.webm")
	cmd := exec.CommandContext(ctx, c.FFmpeg, "-nostdin", "-v", "error", "-i", filepath.Join(root, "frontend/public/audio/sample-voice-note.wav"), "-c:a", "libopus", "-live", "1", webm)
	if err = cmd.Run(); err != nil {
		t.Fatal(err)
	}
	probe, err := exec.CommandContext(ctx, c.FFprobe, "-v", "error", "-show_entries", "format=duration", "-of", "json", webm).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(probe), `"duration"`) {
		t.Fatalf("fixture unexpectedly has duration: %s", probe)
	}
	result, err := w.Transcribe(ctx, webm, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(result.Text), "small details") || result.Language != "en" {
		t.Fatalf("failed MediaRecorder-style audio: %+v", result)
	}
}
