package ai

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Whisperfile runs the bundled tiny model once per queued recording.
type Whisperfile struct{ Config Config }

// Codes accepted by the multilingual Whisper model. Empty means detection.
func validWhisperLanguage(language string) bool {
	if language == "" || language == "auto" {
		return true
	}
	for _, code := range strings.Fields("en zh de es ru ko fr ja pt tr pl ca nl ar sv it id hi fi vi he uk el ms cs ro da hu ta no th ur hr bg lt la mi ml cy sk te fa lv bn sr az sl kn et mk br eu is hy ne mn bs kk sq sw gl mr pa si km sn yo so af oc ka be tg sd gu am yi lo uz fo ht ps tk nn mt sa lb my bo tl mg as tt haw ln ha ba jw su yue") {
		if language == code {
			return true
		}
	}
	return false
}

func (w *Whisperfile) Transcribe(ctx context.Context, input, language string) (Transcript, error) {
	if !validWhisperLanguage(language) {
		return Transcript{}, fmt.Errorf("unsupported Whisper language code")
	}
	if language == "" {
		language = "auto"
	}

	probe := exec.CommandContext(ctx, w.Config.FFprobe, "-v", "error", "-show_entries", "format=duration", "-of", "json", input)
	raw, err := probe.Output()
	if err != nil {
		return Transcript{}, fmt.Errorf("audio cannot be decoded (ffprobe required)")
	}
	var info struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err = json.Unmarshal(raw, &info); err != nil {
		return Transcript{}, fmt.Errorf("invalid audio duration")
	}
	var seconds float64
	// MediaRecorder WebM streams may omit container duration. In that case,
	// enforce the duration limit on decoded samples instead.
	if info.Format.Duration != "" && info.Format.Duration != "N/A" {
		if _, err = fmt.Sscan(info.Format.Duration, &seconds); err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > 1800 {
			return Transcript{}, fmt.Errorf("audio must be between 0 and 1800 seconds")
		}
	}
	cmd := exec.CommandContext(ctx, w.Config.FFmpeg, "-nostdin", "-v", "error", "-i", input, "-t", "1800.01", "-ac", "1", "-ar", "16000", "-f", "s16le", "-c:a", "pcm_s16le", "pipe:1")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return Transcript{}, err
	}
	if err = cmd.Start(); err != nil {
		return Transcript{}, fmt.Errorf("audio normalization failed (ffmpeg required)")
	}
	// Bound memory even for malicious inputs; 30 min of mono 16-bit PCM is 58 MB.
	pcm, readErr := io.ReadAll(io.LimitReader(pipe, 1800*16000*2+1))
	if readErr != nil || len(pcm) > 1800*16000*2 {
		_ = cmd.Process.Kill()
	}
	runErr := cmd.Wait()
	if len(pcm) > 1800*16000*2 {
		return Transcript{}, fmt.Errorf("audio must be between 0 and 1800 seconds")
	}
	if readErr != nil || runErr != nil || len(pcm) == 0 || len(pcm)%2 != 0 {
		return Transcript{}, fmt.Errorf("audio normalization failed")
	}

	dir, err := os.MkdirTemp("", "lyphe-whisper-*")
	if err != nil {
		return Transcript{}, err
	}
	defer os.RemoveAll(dir)
	audio := filepath.Join(dir, "audio.wav")
	// Write a canonical WAV header; piping WAV from FFmpeg leaves unknown lengths.
	header := make([]byte, 44)
	copy(header, "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(len(pcm)+36))
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 1)
	binary.LittleEndian.PutUint32(header[24:], 16000)
	binary.LittleEndian.PutUint32(header[28:], 32000)
	binary.LittleEndian.PutUint16(header[32:], 2)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(len(pcm)))
	file, err := os.Create(audio)
	if err != nil {
		return Transcript{}, err
	}
	_, err = file.Write(header)
	if err == nil {
		_, err = file.Write(pcm)
	}
	closeErr := file.Close()
	if err != nil {
		return Transcript{}, err
	}
	if closeErr != nil {
		return Transcript{}, closeErr
	}
	output := filepath.Join(dir, "transcript")
	command := launch(ctx, w.Config.Whisperfile, "-ng", "-t", fmt.Sprint(max(1, w.Config.Threads)), "-l", language, "-oj", "-of", output, "-f", audio)
	// Engine logs include transcript data; do not leak them through API errors.
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err = command.Run(); err != nil {
		if ctx.Err() != nil {
			return Transcript{}, ctx.Err()
		}
		return Transcript{}, fmt.Errorf("Whisper transcription failed: %w", err)
	}
	resultFile, err := os.Open(output + ".json")
	if err != nil {
		return Transcript{}, fmt.Errorf("Whisper output unavailable: %w", err)
	}
	defer resultFile.Close()
	return decodeWhisperTranscript(io.LimitReader(resultFile, 8<<20))
}

func decodeWhisperTranscript(reader io.Reader) (Transcript, error) {
	var output struct {
		Result struct {
			Language string `json:"language"`
		} `json:"result"`
		Transcription []struct {
			Text    string `json:"text"`
			Offsets struct {
				From float64 `json:"from"`
				To   float64 `json:"to"`
			} `json:"offsets"`
		} `json:"transcription"`
	}
	if err := json.NewDecoder(reader).Decode(&output); err != nil {
		return Transcript{}, fmt.Errorf("invalid Whisper JSON: %w", err)
	}
	if output.Result.Language == "" || output.Transcription == nil {
		return Transcript{}, fmt.Errorf("incomplete Whisper output")
	}
	result := Transcript{Language: output.Result.Language, Segments: []any{}}
	texts := []string{}
	for _, segment := range output.Transcription {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}
		texts = append(texts, text)
		result.Segments = append(result.Segments, map[string]any{"text": text, "start": segment.Offsets.From / 1000, "end": segment.Offsets.To / 1000, "language": result.Language})
	}
	result.Text = strings.Join(texts, " ")
	return result, nil
}
