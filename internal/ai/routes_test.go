package ai

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chand1012/lyphe/internal/backend"
	_ "github.com/chand1012/lyphe/migrations"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestApplyTranscriptProtectsEditedAnchorAndIsIdempotent(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	newRecord := func(table string) *core.Record {
		c, err := app.FindCollectionByNameOrId(table)
		if err != nil {
			t.Fatal(err)
		}
		return core.NewRecord(c)
	}
	save := func(r *core.Record) {
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	user := newRecord("users")
	user.Set("email", "apply@example.com")
	user.SetPassword("test-password-123")
	save(user)
	entry := newRecord("journal")
	entry.Set("title", "Transcript test")
	entry.Set("user", user.Id)
	save(entry)
	audio := newRecord("files")
	audio.Set("user", user.Id)
	audio.Set("name", "test.wav")
	audio.Set("mimetype", "audio/wav")
	file, err := filesystem.NewFileFromBytes([]byte("RIFF test audio"), "test.wav")
	if err != nil {
		t.Fatal(err)
	}
	audio.Set("file", file)
	save(audio)
	doc := backend.Document{Version: 1, Value: []backend.Node{{"type": "p", "id": "anchor", "children": []any{backend.Node{"text": "My edits"}}}}, AudioFileIDs: []string{audio.Id}}
	if err := backend.SaveDocument(app, entry, doc, 0); err != nil {
		t.Fatal(err)
	}
	job := newRecord("transcriptions")
	job.Set("user", user.Id)
	job.Set("file", audio.Id)
	job.Set("journal", entry.Id)
	job.Set("status", "completed")
	job.Set("anchor_id", "anchor")
	job.Set("raw_text", "hello world")
	job.Set("cleaned_text", "Hello world.")
	save(job)
	manager := New(app, DefaultConfig())
	apply := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/apply", strings.NewReader(body))
		req.SetPathValue("id", job.Id)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		e := &core.RequestEvent{App: app, Auth: user}
		e.Event = router.Event{Request: req, Response: res}
		if err := manager.apply(e); err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := apply(`{"baseRevision":1}`); res.Code != 409 {
		t.Fatalf("edited anchor overwritten: %d %s", res.Code, res.Body.String())
	}
	entry, _ = app.FindRecordById("journal", entry.Id)
	doc = backend.ReadDocument(entry)
	doc.Value = append(doc.Value, backend.Node{"type": "p", "id": "new-anchor", "children": []any{backend.Node{"text": ""}}})
	if err := backend.SaveDocument(app, entry, doc, 1); err != nil {
		t.Fatal(err)
	}
	if res := apply(`{"baseRevision":2,"anchorId":"new-anchor"}`); res.Code != 200 {
		t.Fatalf("apply failed: %s", res.Body.String())
	}
	if res := apply(`{"baseRevision":2,"anchorId":"new-anchor"}`); res.Code != 200 {
		t.Fatal("repeat apply failed")
	}
	entry, _ = app.FindRecordById("journal", entry.Id)
	if entry.GetInt("revision") != 3 || strings.Count(entry.GetString("content_text"), "Hello world.") != 1 || !strings.Contains(entry.GetString("content_text"), "My edits") {
		t.Fatal("apply duplicated text or lost edits")
	}
}

type savedTranscriptProvider struct{}

func (savedTranscriptProvider) Transcribe(context.Context, string, string) (Transcript, error) {
	return Transcript{Text: "Hello from Whistle.", Language: "en", Segments: []any{map[string]any{"text": "Hello from Whistle.", "start": 0, "end": 1}}}, nil
}

func TestProcessPersistsTranscriptAcrossRestart(t *testing.T) {
	data := t.TempDir()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: data})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	newRecord := func(table string) *core.Record {
		collection, err := app.FindCollectionByNameOrId(table)
		if err != nil {
			t.Fatal(err)
		}
		return core.NewRecord(collection)
	}
	save := func(record *core.Record) {
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	user := newRecord("users")
	user.Set("email", "storage@example.com")
	user.SetPassword("test-password-123")
	save(user)
	entry := newRecord("journal")
	entry.Set("title", "Stored transcript")
	entry.Set("user", user.Id)
	save(entry)
	audio := newRecord("files")
	audio.Set("user", user.Id)
	audio.Set("name", "recording.wav")
	audio.Set("mimetype", "audio/wav")
	upload, err := filesystem.NewFileFromBytes([]byte("audio fixture"), "recording.wav")
	if err != nil {
		t.Fatal(err)
	}
	audio.Set("file", upload)
	save(audio)
	job := newRecord("transcriptions")
	job.Set("user", user.Id)
	job.Set("file", audio.Id)
	job.Set("journal", entry.Id)
	job.Set("status", "queued")
	save(job)
	manager := New(app, DefaultConfig())
	manager.Transcriber = savedTranscriptProvider{}
	manager.process(context.Background(), job)
	if err := app.ResetBootstrapState(); err != nil {
		t.Fatal(err)
	}
	restarted := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: data})
	if err := restarted.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer restarted.ResetBootstrapState()
	stored, err := restarted.FindRecordById("transcriptions", job.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GetString("status") != "completed" || stored.GetString("raw_text") != "Hello from Whistle." || stored.GetString("cleaned_text") != "Hello from Whistle." || stored.GetString("language") != "en" || len(stored.Get("segments").(types.JSONRaw)) == 0 {
		t.Fatalf("transcript did not persist: %+v", stored.PublicExport())
	}
	fs, err := restarted.NewFilesystem()
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()
	reader, err := fs.GetReader(audio.BaseFilesPath() + "/" + audio.GetString("file"))
	if err != nil {
		t.Fatalf("audio did not persist: %v", err)
	}
	reader.Close()
}
