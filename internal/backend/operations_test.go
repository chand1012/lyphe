package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

func TestAgentJournalBoundaryInSharedOperations(t *testing.T) {
	app := testApp(t)
	user := record(t, app, "users", "owner")
	journal := record(t, app, "journal", user.Id)
	s := &Server{App: app}
	p := Principal{UserID: user.Id, Source: Agent}
	ctx := context.Background()
	revision := 0
	before, _ := json.Marshal(journal.PublicExport())
	text := "Agent text"
	calls := []func() error{
		func() error { _, err := s.Create(ctx, p, "journal", map[string]any{"title": "Agent"}, nil); return err },
		func() error {
			_, err := s.Update(ctx, p, "journal", journal.Id, 0, map[string]any{"title": "Agent"})
			return err
		},
		func() error {
			_, err := s.WriteDocument(ctx, p, "journal", journal.Id, 0, PlainDocument("Agent"), nil)
			return err
		},
		func() error {
			_, err := s.WriteDocument(ctx, p, "journal", journal.Id, 0, Document{}, &text)
			return err
		},
		func() error { _, err := s.ReplaceText(ctx, p, "journal", journal.Id, 0, "Agent"); return err },
		func() error { _, err := s.SetDeleted(ctx, p, "journal", journal.Id, false, &revision); return err },
		func() error { _, err := s.SetDeleted(ctx, p, "journal", journal.Id, true, &revision); return err },
		func() error { _, err := s.Duplicate(ctx, p, "journal", journal.Id, &revision); return err },
	}
	for _, call := range calls {
		if err := call(); !errors.Is(err, ErrPermission) {
			t.Fatalf("journal mutation: %v", err)
		}
	}
	saved, _ := app.FindRecordById("journal", journal.Id)
	after, _ := json.Marshal(saved.PublicExport())
	if !bytes.Equal(before, after) {
		t.Fatal("journal mutated")
	}
	// Human document operations still use the same service and remain writable.
	if _, err := s.WriteDocument(ctx, Principal{UserID: user.Id, Source: Human}, "journal", journal.Id, 0, PlainDocument("Human text"), nil); err != nil {
		t.Fatal(err)
	}
}
func TestAgentDocumentIntegrityAndTransactionalRollback(t *testing.T) {
	app := testApp(t)
	u := record(t, app, "users", "owner")
	other := record(t, app, "users", "other")
	s := &Server{App: app}
	p := Principal{UserID: u.Id, Source: Agent}
	ctx := context.Background()
	c, _ := app.FindCollectionByNameOrId("files")
	f := core.NewRecord(c)
	f.Set("user", u.Id)
	f.Set("name", "Owned attachment")
	f.Set("mimetype", "text/plain")
	file, err := filesystem.NewFileFromBytes([]byte("hello"), "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Set("file", file)
	if err = app.Save(f); err != nil {
		t.Fatal(err)
	}
	task := record(t, app, "tasks", u.Id)
	d := Document{Version: 1, AudioFileIDs: []string{}, Value: []Node{{"type": "p", "id": "anchor", "children": []any{Node{"text": "Bold", "bold": true}}}, {"type": "p", "children": []any{Node{"type": "file-attachment", "fileId": f.Id, "placementId": "placement", "url": "blob:temporary", "children": []any{Node{"text": ""}}}}}}}
	out, err := s.WriteDocument(ctx, p, "task", task.Id, 0, d, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := "Appended"
	out, err = s.WriteDocument(ctx, p, "task", task.Id, int(out["revision"].(float64)), Document{}, &text)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := app.FindRecordById("tasks", task.Id)
	doc := ReadDocument(saved)
	raw, _ := json.Marshal(doc)
	if !bytes.Contains(raw, []byte(`"bold":true`)) || !bytes.Contains(raw, []byte(`"id":"anchor"`)) || bytes.Contains(raw, []byte("blob:")) {
		t.Fatal(string(raw))
	}
	out, err = s.ReplaceText(ctx, p, "task", task.Id, saved.GetInt("revision"), "Replacement")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := app.FindRecordsByFilter("entity_files", "source_task={:id}", "", 0, 0, dbx.Params{"id": task.Id})
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	foreign := record(t, app, "goals", other.Id)
	_, err = s.Update(ctx, p, "task", task.Id, int(out["revision"].(float64)), map[string]any{"title": "Bad write", "tags": []any{"rollback tag"}, "goal": foreign.Id})
	if err == nil {
		t.Fatal("foreign relationship accepted")
	}
	saved, _ = app.FindRecordById("tasks", task.Id)
	if saved.GetString("title") == "Bad write" || saved.GetInt("revision") != int(out["revision"].(float64)) {
		t.Fatal("failed mutation was committed")
	}
	tags, _ := app.FindRecordsByFilter("tags", "name='rollback tag'", "", 0, 0)
	if len(tags) != 0 {
		t.Fatal("tag survived rollback")
	}
	// Failure after initial create must roll back the entity, tags and search rows.
	invalid := PlainDocument("bad")
	invalid.Value[0]["type"] = "unsupported"
	_, err = s.Create(ctx, p, "task", map[string]any{"title": "Must rollback", "tags": []any{"new rollback"}}, &invalid)
	if err == nil {
		t.Fatal("invalid document accepted")
	}
	tasks, _ := app.FindRecordsByFilter("tasks", "title='Must rollback'", "", 0, 0)
	if len(tasks) != 0 {
		t.Fatal("partially created entity")
	}
	// Legacy conversion and append preserve original content.
	legacy := record(t, app, "tasks", u.Id)
	legacy.Set("description", "<p>Legacy <strong>bold</strong></p>")
	if err = app.Save(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err = s.WriteDocument(ctx, p, "task", legacy.Id, 0, Document{}, &text); err != nil {
		t.Fatal(err)
	}
	legacy, _ = app.FindRecordById("tasks", legacy.Id)
	raw, _ = json.Marshal(ReadDocument(legacy))
	if !bytes.Contains(raw, []byte("Legacy")) || !bytes.Contains(raw, []byte(`"bold":true`)) {
		t.Fatal(string(raw))
	}
}
func TestAgentOrderingFiltersAndCancellation(t *testing.T) {
	app := testApp(t)
	u := record(t, app, "users", "owner")
	s := &Server{App: app}
	p := Principal{UserID: u.Id, Source: Agent}
	ctx := context.Background()
	first, err := s.Create(ctx, p, "task", map[string]any{"title": "First", "due_on": "2026-10-05", "priority": "high", "effort": 30, "wait_until": "2026-10-04 00:00:00.000Z"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Create(ctx, p, "task", map[string]any{"title": "Second"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Reorder(ctx, p, []TaskOrder{{ID: first["id"].(string), Status: "done", Position: 1, Revision: 0}, {ID: second["id"].(string), Status: "done", Position: 2, Revision: 99}})
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	r, _ := app.FindRecordById("tasks", first["id"].(string))
	if r.GetString("status") != "todo" || r.GetInt("revision") != 0 {
		t.Fatal("partial reorder")
	}
	listed, err := s.List(ctx, p, "task", ListOptions{From: "2026-10-05", To: "2026-10-05", Status: "todo"})
	if err != nil || len(listed.Items) != 1 {
		t.Fatal(listed, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.Update(canceled, p, "task", r.Id, 0, map[string]any{"title": "Canceled"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, Principal{UserID: u.Id, Source: ""}, "task", r.Id, 0, map[string]any{"title": "Anonymous"}); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
