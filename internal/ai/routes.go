package ai

import (
	"context"
	"fmt"
	"github.com/chand1012/lyphe/internal/backend"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

var tables = map[string]string{"journal": "journal", "task": "tasks", "goal": "goals", "habit": "habits"}

func Register(app core.App) *Manager {
	m := New(app, DefaultConfig())
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		r := e.Router.Group("/api")
		r.Bind(apis.RequireAuth("users"))
		r.GET("/ai/status", m.status)
		r.POST("/ai/completion", m.completion)
		r.POST("/ai/transcribe", m.enqueue)
		r.POST("/transcriptions/{id}/apply", m.apply)
		r.POST("/transcriptions/{id}/retry", m.retry)
		if env("LYPHE_AI_ENABLED", "true") != "false" {
			m.Start()
		} else {
			m.state.Store("disabled")
		}
		return e.Next()
	})
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error { m.Close(); return e.Next() })
	return m
}
func owned(app core.App, table, id, user string) (*core.Record, error) {
	r, err := app.FindRecordById(table, id)
	if err != nil || r.GetString("user") != user {
		return nil, fmt.Errorf("record unavailable")
	}
	return r, nil
}
func (m *Manager) status(e *core.RequestEvent) error {
	_, werr := os.Stat(m.Config.WhistleWASM)
	_, merr := os.Stat(m.Config.WhistleModel)
	_, ferr := exec.LookPath(m.Config.FFmpeg)
	_, perr := exec.LookPath(m.Config.FFprobe)
	settings, err := backend.Preferences(e.App, e.Auth.Id)
	if err != nil {
		return e.BadRequestError("Preferences unavailable", nil)
	}
	return e.JSON(200, map[string]any{"completion": map[string]any{"state": m.state.Load(), "available": m.ready.Load(), "provider": "local"}, "transcription": map[string]any{"available": werr == nil && merr == nil && ferr == nil && perr == nil && m.cancel != nil, "provider": "whistle-wazero"}, "preferences": settings})
}
func (m *Manager) completion(e *core.RequestEvent) error {
	var input struct {
		Context   string `json:"context"`
		RequestID string `json:"requestId"`
	}
	if err := e.BindBody(&input); err != nil {
		return e.BadRequestError("Invalid request", nil)
	}
	if len(input.Context) > 6000 {
		return e.BadRequestError("Context is too long", nil)
	}
	settings, err := backend.Preferences(e.App, e.Auth.Id)
	if err != nil || !settings.GetBool("enable_ai") || !settings.GetBool("autocomplete") {
		return e.JSON(200, map[string]string{"text": "", "requestId": input.RequestID})
	}
	if m.cleanupWaiting.Load() > 0 || !canAutocomplete(input.Context) {
		return e.JSON(200, map[string]string{"text": "", "requestId": input.RequestID})
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 3*time.Second)
	defer cancel()
	var text string
	verbatim := false
	if provider, ok := m.Provider.(AutocompleteProvider); ok {
		text, err = provider.Autocomplete(ctx, input.Context)
		verbatim = true
	} else {
		text, err = m.Provider.Complete(ctx, autocompletePrompt, input.Context, 12)
	}
	if err != nil {
		return e.JSON(503, map[string]string{"code": "ai_unavailable", "message": "Autocomplete is temporarily unavailable"})
	}
	if verbatim {
		text = trimAutocompleteInsertion(input.Context, text)
	} else {
		text = trimAutocomplete(input.Context, text)
	}
	return e.JSON(200, map[string]any{"text": text, "requestId": input.RequestID, "verbatim": verbatim})
}
func (m *Manager) enqueue(e *core.RequestEvent) error {
	var input struct {
		FileID     string `json:"fileId"`
		Kind       string `json:"kind"`
		EntityID   string `json:"entityId"`
		AnchorID   string `json:"anchorId"`
		RequestKey string `json:"requestKey"`
		Language   string `json:"language"`
	}
	if err := e.BindBody(&input); err != nil {
		return e.BadRequestError("Invalid request", nil)
	}
	if !validWhistleLanguage(input.Language) {
		return e.BadRequestError("Whistle supports en, de, fr, es, it, nl and pl", nil)
	}
	table, ok := tables[input.Kind]
	if !ok || len(input.RequestKey) < 8 || len(input.RequestKey) > 100 {
		return e.BadRequestError("Invalid parent or request key", nil)
	}
	file, err := owned(e.App, "files", input.FileID, e.Auth.Id)
	if err != nil {
		return e.NotFoundError("Audio unavailable", nil)
	}
	mime := file.GetString("mimetype")
	if !strings.HasPrefix(mime, "audio/") && !strings.Contains(mime, "webm") && !strings.Contains(mime, "mp4") {
		return e.BadRequestError("Choose an audio file", nil)
	}
	parent, err := owned(e.App, table, input.EntityID, e.Auth.Id)
	if err != nil || parent.GetString("deleted_at") != "" {
		return e.NotFoundError("Parent unavailable", nil)
	}
	links, err := e.App.FindRecordsByFilter("entity_files", "file={:f} && source_"+input.Kind+"={:p}", "", 1, 0, dbx.Params{"f": file.Id, "p": parent.Id})
	if err != nil || len(links) == 0 {
		return e.BadRequestError("Save the audio attachment before transcribing", nil)
	}
	settings, err := backend.Preferences(e.App, e.Auth.Id)
	if err != nil || !settings.GetBool("enable_transcription") {
		return e.BadRequestError("Transcription is disabled", nil)
	}
	var job *core.Record
	err = e.App.RunInTransaction(func(app core.App) error {
		existing, err := app.FindRecordsByFilter("transcriptions", "user={:u} && request_key={:k}", "", 1, 0, dbx.Params{"u": e.Auth.Id, "k": input.RequestKey})
		if err != nil {
			return err
		}
		if len(existing) > 0 {
			job = existing[0]
			return nil
		}
		c, _ := app.FindCollectionByNameOrId("transcriptions")
		job = core.NewRecord(c)
		job.Set("user", e.Auth.Id)
		job.Set("file", file.Id)
		job.Set(input.Kind, parent.Id)
		job.Set("status", "queued")
		job.Set("anchor_id", input.AnchorID)
		job.Set("language", input.Language)
		job.Set("request_key", input.RequestKey)
		return app.Save(job)
	})
	if err != nil {
		return e.BadRequestError("Could not queue transcription", err)
	}
	return e.JSON(202, job)
}
func (m *Manager) retry(e *core.RequestEvent) error {
	job, err := owned(e.App, "transcriptions", e.Request.PathValue("id"), e.Auth.Id)
	if err != nil {
		return e.NotFoundError("Job unavailable", nil)
	}
	if job.GetBool("applied") {
		return e.BadRequestError("Transcript already inserted", nil)
	}
	if job.GetString("status") != "failed" && job.GetString("status") != "completed" {
		return e.BadRequestError("Job is already running", nil)
	}
	job.Set("status", "queued")
	job.Set("error", "")
	if err = e.App.Save(job); err != nil {
		return err
	}
	return e.JSON(202, job)
}
func (m *Manager) work(ctx context.Context) {
	jobs, err := m.App.FindRecordsByFilter("transcriptions", "status='transcribing' || status='cleaning'", "", 0, 0)
	if err == nil {
		for _, job := range jobs {
			job.Set("status", "queued")
			_ = m.App.Save(job)
		}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := m.App.FindRecordsByFilter("transcriptions", "status='queued'", "created", 1, 0)
			if err == nil && len(jobs) > 0 {
				m.process(ctx, jobs[0])
			}
		}
	}
}
func (m *Manager) process(ctx context.Context, job *core.Record) {
	if fresh, err := m.App.FindRecordById("transcriptions", job.Id); err != nil || fresh.GetString("status") == "canceled" {
		return
	}
	job.Set("attempts", job.GetInt("attempts")+1)
	job.Set("status", "transcribing")
	if err := m.App.Save(job); err != nil {
		return
	}
	// WASM CPU inference can take longer than the recording itself.
	jobCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	fail := func(message string) {
		if fresh, err := m.App.FindRecordById("transcriptions", job.Id); err != nil || fresh.GetString("status") == "canceled" {
			return
		}
		if ctx.Err() != nil {
			job.Set("status", "queued")
		} else {
			job.Set("status", "failed")
		}
		job.Set("error", message)
		_ = m.App.Save(job)
	}
	raw := job.GetString("raw_text")
	if raw == "" {
		file, err := owned(m.App, "files", job.GetString("file"), job.GetString("user"))
		if err != nil {
			fail("Audio is unavailable")
			return
		}
		fs, err := m.App.NewFilesystem()
		if err != nil {
			fail("Storage is unavailable")
			return
		}
		defer fs.Close()
		reader, err := fs.GetReader(file.BaseFilesPath() + "/" + file.GetString("file"))
		if err != nil {
			fail("Audio is unavailable")
			return
		}
		defer reader.Close()
		temp, err := os.CreateTemp("", "lyphe-audio-*")
		if err != nil {
			fail("Could not prepare audio")
			return
		}
		defer os.Remove(temp.Name())
		_, err = io.Copy(temp, reader)
		temp.Close()
		if err != nil {
			fail("Could not read audio")
			return
		}
		result, err := m.Transcriber.Transcribe(jobCtx, temp.Name(), job.GetString("language"))
		if err != nil {
			fail(err.Error())
			return
		}
		raw = result.Text
		job.Set("raw_text", raw)
		job.Set("segments", result.Segments)
		job.Set("language", result.Language)
	}
	if fresh, err := m.App.FindRecordById("transcriptions", job.Id); err != nil || fresh.GetString("status") == "canceled" {
		return
	}
	job.Set("status", "cleaning")
	if err := m.App.Save(job); err != nil {
		return
	}
	settings, err := backend.Preferences(m.App, job.GetString("user"))
	clean := raw
	outcome := "disabled"
	if err == nil && settings.GetBool("enable_ai") && settings.GetBool("cleanup") && strings.TrimSpace(raw) != "" {
		outcome = "accepted"
		text, err := m.clean(jobCtx, raw)
		if err != nil {
			outcome = "raw_fallback"
		} else {
			clean = text
		}
	}
	job.Set("cleaned_text", clean)
	job.Set("cleanup_outcome", outcome)
	job.Set("status", "completed")
	job.Set("error", "")
	_ = m.App.RunInTransaction(func(app core.App) error {
		fresh, err := app.FindRecordById("transcriptions", job.Id)
		if err != nil {
			return err
		}
		if fresh.GetString("status") == "canceled" {
			return nil
		}
		if err := app.Save(job); err != nil {
			return err
		}
		for kind, table := range tables {
			if id := job.GetString(kind); id != "" {
				if parent, err := app.FindRecordById(table, id); err == nil {
					return backend.IndexEntity(app, parent)
				}
			}
		}
		return nil
	})
}
func (m *Manager) apply(e *core.RequestEvent) error {
	var input struct {
		BaseRevision int    `json:"baseRevision"`
		AnchorID     string `json:"anchorId"`
		Raw          bool   `json:"raw"`
	}
	if err := e.BindBody(&input); err != nil {
		return e.BadRequestError("Invalid request", nil)
	}
	var parent *core.Record
	err := e.App.RunInTransaction(func(app core.App) error {
		job, err := owned(app, "transcriptions", e.Request.PathValue("id"), e.Auth.Id)
		if err != nil {
			return err
		}
		for kind, table := range tables {
			if id := job.GetString(kind); id != "" {
				parent, err = owned(app, table, id, e.Auth.Id)
				break
			}
		}
		if err != nil || parent == nil || parent.GetString("deleted_at") != "" {
			return fmt.Errorf("parent unavailable")
		}
		if job.GetBool("applied") {
			return nil
		}
		if job.GetString("status") != "completed" {
			return fmt.Errorf("transcript is not ready")
		}
		anchor := job.GetString("anchor_id")
		if input.AnchorID != "" {
			anchor = input.AnchorID
		}
		d := backend.ReadDocument(parent)
		text := job.GetString("cleaned_text")
		if input.Raw {
			text = job.GetString("raw_text")
		}
		found := false
		for _, node := range d.Value {
			if node["id"] == anchor {
				children, _ := node["children"].([]any)
				for _, child := range children {
					if leaf, ok := child.(map[string]any); !ok || leaf["text"] != "" {
						return backend.ErrConflict
					}
				}
				node["children"] = []any{backend.Node{"text": text}}
				found = true
				break
			}
		}
		if !found {
			return backend.ErrConflict
		}
		if err = backend.SaveDocument(app, parent, d, input.BaseRevision); err != nil {
			return err
		}
		job.Set("applied", true)
		return app.Save(job)
	})
	if err == backend.ErrConflict {
		return e.JSON(409, map[string]string{"code": "anchor_changed", "message": "Choose where to insert the transcript"})
	}
	if err != nil {
		return e.BadRequestError(err.Error(), nil)
	}
	return e.JSON(200, backend.DTO(e.App, parent))
}
