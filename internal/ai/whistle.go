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
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

const whistleSampleRate = 16000
const whistleChunkSamples = 30 * whistleSampleRate
const whistleOutputSize = 1 << 18

// WhistleWASM holds one non-thread-safe engine instance inside the Go server.
// It has no filesystem mounts or network imports: audio and weights enter as bytes.
type WhistleWASM struct {
	Config  Config
	mu      sync.Mutex
	runtime wazero.Runtime
	module  api.Module
}

func (w *WhistleWASM) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.close()
}
func (w *WhistleWASM) close() error {
	if w.runtime == nil {
		return nil
	}
	err := w.runtime.Close(context.Background())
	w.runtime, w.module = nil, nil
	return err
}

func (w *WhistleWASM) load(ctx context.Context) (err error) {
	if w.module != nil && !w.module.IsClosed() {
		return nil
	}
	_ = w.close()
	wasm, err := os.ReadFile(w.Config.WhistleWASM)
	if err != nil {
		return fmt.Errorf("Whistle engine unavailable: %w", err)
	}
	weights, err := os.ReadFile(w.Config.WhistleModel)
	if err != nil {
		return fmt.Errorf("Whistle weights unavailable: %w", err)
	}
	w.runtime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(8192).WithCloseOnContextDone(true))
	defer func() {
		if err != nil {
			_ = w.close()
		}
	}()
	if _, err = wasi_snapshot_preview1.Instantiate(ctx, w.runtime); err != nil {
		return err
	}
	// The official core build uses Emscripten for heap growth and POSIX imports.
	// File operations are denied; the speech C API needs only memory and clocks.
	env := w.runtime.NewHostModuleBuilder("env")
	env.NewFunctionBuilder().WithFunc(func(context.Context) { panic("Whistle aborted") }).Export("_abort_js")
	env.NewFunctionBuilder().WithFunc(func(context.Context, uint32, uint32, uint32) { panic("Whistle C++ exception") }).Export("__cxa_throw")
	env.NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, size uint32) uint32 {
		current := uint64(m.Memory().Size())
		if uint64(size) <= current {
			return 1
		}
		_, ok := m.Memory().Grow(uint32((uint64(size) - current + 65535) / 65536))
		if ok {
			return 1
		}
		return 0
	}).Export("emscripten_resize_heap")
	env.NewFunctionBuilder().WithFunc(func(uint32, uint32, uint32) int32 { return -63 }).Export("__syscall_fcntl64")
	env.NewFunctionBuilder().WithFunc(func(uint32, uint32, uint32) int32 { return -59 }).Export("__syscall_ioctl")
	env.NewFunctionBuilder().WithFunc(func(uint32, uint32, uint32, uint32) int32 { return -63 }).Export("__syscall_openat")
	env.NewFunctionBuilder().WithFunc(func(uint32) int32 { return -63 }).Export("__syscall_rmdir")
	env.NewFunctionBuilder().WithFunc(func(uint32, uint32, uint32) int32 { return -63 }).Export("__syscall_unlinkat")
	env.NewFunctionBuilder().WithFunc(func(uint32, uint32, uint32, uint32, uint32, uint64) int32 { return -63 }).Export("_munmap_js")
	if _, err = env.Instantiate(ctx); err != nil {
		return err
	}
	compiled, err := w.runtime.CompileModule(ctx, wasm)
	if err != nil {
		return fmt.Errorf("compile Whistle core WASM: %w", err)
	}
	for _, name := range []string{"malloc", "free", "needle_load", "needle_transcribe", "needle_last_error", "__wasm_call_ctors"} {
		if _, ok := compiled.ExportedFunctions()[name]; !ok {
			return fmt.Errorf("Whistle WASM missing %s; run just build-whistle", name)
		}
	}
	w.module, err = w.runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStartFunctions("__wasm_call_ctors").WithSysWalltime().WithSysNanotime())
	if err != nil {
		return fmt.Errorf("instantiate Whistle: %w", err)
	}
	pointer, err := w.put(ctx, weights)
	if err != nil {
		return err
	}
	defer w.free(pointer)
	result, err := w.module.ExportedFunction("needle_load").Call(ctx, uint64(pointer), uint64(len(weights)))
	if err != nil {
		return err
	}
	if int32(result[0]) < 0 {
		return w.engineError(ctx)
	}
	return nil
}

func (w *WhistleWASM) put(ctx context.Context, data []byte) (uint32, error) {
	result, err := w.module.ExportedFunction("malloc").Call(ctx, uint64(len(data)))
	if err != nil {
		return 0, err
	}
	pointer := uint32(result[0])
	if pointer == 0 || !w.module.Memory().Write(pointer, data) {
		return 0, fmt.Errorf("Whistle memory allocation failed")
	}
	return pointer, nil
}
func (w *WhistleWASM) free(pointer uint32) {
	if w.module != nil && !w.module.IsClosed() {
		_, _ = w.module.ExportedFunction("free").Call(context.Background(), uint64(pointer))
	}
}
func (w *WhistleWASM) stringAt(pointer, limit uint32) (string, error) {
	memory := w.module.Memory()
	if pointer >= memory.Size() {
		return "", fmt.Errorf("invalid Whistle output pointer")
	}
	data, ok := memory.Read(pointer, min(limit, memory.Size()-pointer))
	if !ok {
		return "", fmt.Errorf("invalid Whistle output")
	}
	for i, b := range data {
		if b == 0 {
			return string(data[:i]), nil
		}
	}
	return "", fmt.Errorf("Whistle output exceeds buffer")
}
func (w *WhistleWASM) engineError(ctx context.Context) error {
	result, err := w.module.ExportedFunction("needle_last_error").Call(ctx)
	if err != nil {
		return err
	}
	message, err := w.stringAt(uint32(result[0]), 4096)
	if err != nil {
		return err
	}
	return fmt.Errorf("Whistle: %s", message)
}

type whistleWord struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Probability float64 `json:"probability"`
}
type whistleResult struct {
	Text     string        `json:"text"`
	Language string        `json:"language"`
	Words    []whistleWord `json:"words"`
}

func validWhistleLanguage(language string) bool {
	switch language {
	case "", "auto", "en", "de", "fr", "es", "it", "nl", "pl":
		return true
	}
	return false
}
func (w *WhistleWASM) transcribeChunk(ctx context.Context, pcm []byte, language string) (whistleResult, error) {
	var result whistleResult
	if err := w.load(ctx); err != nil {
		return result, err
	}
	audio, err := w.put(ctx, pcm)
	if err != nil {
		return result, err
	}
	defer w.free(audio)
	output, err := w.put(ctx, make([]byte, whistleOutputSize))
	if err != nil {
		return result, err
	}
	defer w.free(output)
	var lang uint32
	if language != "" && language != "auto" {
		lang, err = w.put(ctx, append([]byte(language), 0))
		if err != nil {
			return result, err
		}
		defer w.free(lang)
	}
	code, err := w.module.ExportedFunction("needle_transcribe").Call(ctx, uint64(audio), uint64(len(pcm)/4), uint64(lang), 0, 1, uint64(output), whistleOutputSize)
	if err != nil {
		return result, err
	}
	if int32(code[0]) < 0 {
		return result, w.engineError(ctx)
	}
	raw, err := w.stringAt(output, whistleOutputSize)
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal([]byte(raw), &result); err != nil {
		return result, fmt.Errorf("invalid Whistle JSON: %w", err)
	}
	return result, nil
}

// splitPCM chooses a quiet boundary near 30 s to reduce cuts inside spoken words.
// Chunks never overlap, so their timestamps remain relative to the original file.
func splitPCM(pcm []byte) int {
	count := len(pcm) / 4
	if count <= whistleChunkSamples {
		return count
	}
	const window = whistleSampleRate / 10
	best, energy := whistleChunkSamples, math.Inf(1)
	for end := 25 * whistleSampleRate; end <= whistleChunkSamples; end += window {
		sum := 0.0
		for i := end - window; i < end; i++ {
			sample := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[i*4:])))
			sum += sample * sample
		}
		if sum <= energy {
			best, energy = end, sum
		}
	}
	return best
}
func (w *WhistleWASM) Transcribe(ctx context.Context, input, language string) (Transcript, error) {
	if !validWhistleLanguage(language) {
		return Transcript{}, fmt.Errorf("Whistle supports en, de, fr, es, it, nl and pl")
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
	cmd := exec.CommandContext(ctx, w.Config.FFmpeg, "-nostdin", "-v", "error", "-i", input, "-t", "1800.01", "-ac", "1", "-ar", "16000", "-f", "f32le", "-c:a", "pcm_f32le", "pipe:1")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return Transcript{}, err
	}
	if err = cmd.Start(); err != nil {
		return Transcript{}, fmt.Errorf("audio normalization failed (ffmpeg required)")
	}
	// Bound memory even for malicious inputs; 30 min of mono float PCM is 115 MB.
	pcm, readErr := io.ReadAll(io.LimitReader(pipe, 1800*whistleSampleRate*4+1))
	if readErr != nil || len(pcm) > 1800*whistleSampleRate*4 {
		_ = cmd.Process.Kill()
	}
	runErr := cmd.Wait()
	if len(pcm) > 1800*whistleSampleRate*4 {
		return Transcript{}, fmt.Errorf("audio must be between 0 and 1800 seconds")
	}
	if readErr != nil || runErr != nil || len(pcm) == 0 || len(pcm)%4 != 0 {
		return Transcript{}, fmt.Errorf("audio normalization failed")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	result := Transcript{Segments: []any{}}
	text := []string{}
	offset := 0
	for len(pcm) > 0 {
		if err := ctx.Err(); err != nil {
			return Transcript{}, err
		}
		count := splitPCM(pcm)
		chunk, err := w.transcribeChunk(ctx, pcm[:count*4], language)
		if err != nil {
			_ = w.close()
			return Transcript{}, fmt.Errorf("Whistle transcription failed: %w", err)
		}
		start := float64(offset) / whistleSampleRate
		for i := range chunk.Words {
			chunk.Words[i].Start += start
			chunk.Words[i].End += start
		}
		if strings.TrimSpace(chunk.Text) != "" {
			text = append(text, strings.TrimSpace(chunk.Text))
		}
		if result.Language == "" {
			result.Language = chunk.Language
		}
		result.Segments = append(result.Segments, map[string]any{"text": chunk.Text, "start": start, "end": float64(offset+count) / whistleSampleRate, "language": chunk.Language, "words": chunk.Words})
		offset += count
		pcm = pcm[count*4:]
	}
	result.Text = strings.Join(text, " ")
	return result, nil
}
