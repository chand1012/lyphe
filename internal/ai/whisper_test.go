package ai

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDecodeWhisperTranscript(t *testing.T) {
	result, err := decodeWhisperTranscript(strings.NewReader(`{"result":{"language":"ja"},"transcription":[{"text":" Hello. ","offsets":{"from":1230,"to":4560}},{"text":" World.","offsets":{"from":4560,"to":7890}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "Hello. World." || result.Language != "ja" || len(result.Segments) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	segment := result.Segments[0].(map[string]any)
	if segment["start"] != 1.23 || segment["end"] != 4.56 {
		t.Fatalf("incorrect seconds: %+v", segment)
	}
	for _, input := range []string{`{}`, `{"result":{"language":"en"}}`, `not JSON`} {
		if _, err := decodeWhisperTranscript(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted invalid output: %s", input)
		}
	}
	silence, err := decodeWhisperTranscript(strings.NewReader(`{"result":{"language":"en"},"transcription":[]}`))
	if err != nil || silence.Text != "" || len(silence.Segments) != 0 {
		t.Fatalf("invalid silence: %+v, %v", silence, err)
	}
}

func TestWhisperLanguageValidation(t *testing.T) {
	for _, code := range []string{"", "auto", "en", "ja", "zh", "ar", "de", "pl"} {
		if !validWhisperLanguage(code) {
			t.Fatalf("rejected %q", code)
		}
	}
	for _, code := range []string{"english", "--help", "xx", "en-US"} {
		if validWhisperLanguage(code) {
			t.Fatalf("accepted %q", code)
		}
	}
}

// Opt-in: exercise the downloaded whisperfile and real audio conversion.
func TestWhisperIntegration(t *testing.T) {
	if os.Getenv("LYPHE_WHISPER_TEST") != "1" {
		t.Skip("set LYPHE_WHISPER_TEST=1 with local tiny.whisperfile")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	c := DefaultConfig()
	if os.Getenv("LYPHE_WHISPERFILE") == "" {
		c.Whisperfile = filepath.Join(root, "models/tiny.whisperfile")
	}
	w := &Whisperfile{Config: c}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	sample := filepath.Join(root, "frontend/public/audio/sample-voice-note.wav")
	for _, language := range []string{"", "en"} {
		result, err := w.Transcribe(ctx, sample, language)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.ToLower(result.Text), "small details") || result.Language != "en" || len(result.Segments) == 0 {
			t.Fatalf("missing speech: %+v", result)
		}
		t.Logf("%s: %s", language, result.Text)
	}
	// MediaRecorder WebM has no container duration; decoded samples enforce limits.
	webm := filepath.Join(t.TempDir(), "recording.webm")
	if err := exec.CommandContext(ctx, c.FFmpeg, "-nostdin", "-v", "error", "-i", sample, "-c:a", "libopus", "-f", "webm", "-live", "1", webm).Run(); err != nil {
		t.Fatal(err)
	}
	if result, err := w.Transcribe(ctx, webm, "en"); err != nil || !strings.Contains(strings.ToLower(result.Text), "small details") {
		t.Fatalf("WebM: %+v, %v", result, err)
	}
	long := filepath.Join(t.TempDir(), "long.wav")
	if err := exec.CommandContext(ctx, c.FFmpeg, "-nostdin", "-v", "error", "-stream_loop", "4", "-i", sample, long).Run(); err != nil {
		t.Fatal(err)
	}
	result, err := w.Transcribe(ctx, long, "en")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.ToLower(result.Text), "small details") < 4 || result.Segments[len(result.Segments)-1].(map[string]any)["end"].(float64) < 40 {
		t.Fatalf("long recording truncated: %+v", result)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := w.Transcribe(canceled, sample, "en"); err == nil {
		t.Fatal("canceled job succeeded")
	}
	if _, err := w.Transcribe(ctx, sample, "xx"); err == nil {
		t.Fatal("unsupported language succeeded")
	}
}
