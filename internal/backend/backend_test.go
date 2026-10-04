package backend

import (
	_ "github.com/chand1012/lyphe/migrations"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"net/http/httptest"
	"strings"
	"testing"
)

func testApp(t *testing.T) core.App {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	return app
}
func record(t *testing.T, app core.App, table, user string) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(table)
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	if table == "users" {
		r.Set("email", user+"@example.com")
		r.SetPassword("test-password-123")
	} else {
		r.Set("user", user)
		r.Set("title", "Entry")
		if table == "tasks" {
			r.Set("status", "todo")
		}
		if table == "habits" {
			r.Set("cadence", "daily")
		}
	}
	if err = app.Save(r); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestDocumentTransactionsAndOwnership(t *testing.T) {
	app := testApp(t)
	u := record(t, app, "users", "first")
	other := record(t, app, "users", "second")
	task := record(t, app, "tasks", u.Id)
	foreign := record(t, app, "tasks", other.Id)
	journal := record(t, app, "journal", u.Id)
	d := Document{Version: 1, Value: []Node{{"type": "p", "id": "anchor", "children": []any{map[string]any{"text": "Working on "}, map[string]any{"type": "entity-mention", "kind": "task", "entityId": task.Id, "children": []any{map[string]any{"text": ""}}}, map[string]any{"text": " today"}}}}, AudioFileIDs: []string{}}
	if err := app.RunInTransaction(func(tx core.App) error { return SaveDocument(tx, journal, d, 0) }); err != nil {
		t.Fatal(err)
	}
	saved, _ := app.FindRecordById("journal", journal.Id)
	if saved.GetInt("revision") != 1 {
		t.Fatal("revision not advanced")
	}
	refs, err := app.FindAllRecords("document_references")
	if err != nil || len(refs) != 1 {
		t.Fatalf("references %v %v", len(refs), err)
	}
	if err := SaveDocument(app, saved, d, 0); err != ErrConflict {
		t.Fatalf("stale revision accepted: %v", err)
	}
	d.Value[0]["children"].([]any)[1].(map[string]any)["entityId"] = foreign.Id
	if err := app.RunInTransaction(func(tx core.App) error { return SaveDocument(tx, saved, d, 1) }); err == nil {
		t.Fatal("cross-owner mention accepted")
	}
	stored, _ := app.FindRecordById("journal", journal.Id)
	if stored.GetInt("revision") != 1 {
		t.Fatal("failed write changed revision")
	}
}
func TestSearchIsPrivate(t *testing.T) {
	app := testApp(t)
	u := record(t, app, "users", "first")
	other := record(t, app, "users", "second")
	for _, owner := range []string{u.Id, other.Id} {
		r := record(t, app, "journal", owner)
		r.Set("title", "Private telescope")
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
		if err := IndexEntity(app, r); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest("GET", "/api/search?q=telescope", nil)
	res := httptest.NewRecorder()
	e := &core.RequestEvent{App: app, Auth: u}
	e.Event = router.Event{Request: req, Response: res}
	if err := (&Server{App: app}).search(e); err != nil {
		t.Fatal(err)
	}
	if strings.Count(res.Body.String(), "Private telescope") != 1 {
		t.Fatal(res.Body.String())
	}
}
func TestLegacyDescriptionIsPreserved(t *testing.T) {
	d := legacyDocument("<h2>Thoughts</h2><p>Hello <strong>world</strong>.</p><script>bad()</script>")
	if len(d.Value) != 2 {
		t.Fatalf("unexpected document: %#v", d)
	}
	if d.Value[1]["children"].([]any)[1].(Node)["bold"] != true {
		t.Fatal("mark lost")
	}
}

func request(app core.App, user *core.Record, method, path, body string, values map[string]string) (*core.RequestEvent, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range values {
		req.SetPathValue(k, v)
	}
	res := httptest.NewRecorder()
	e := &core.RequestEvent{App: app, Auth: user}
	e.Event = router.Event{Request: req, Response: res}
	return e, res
}
func TestHabitDayIsIdempotentAndNotesPreserveCompletion(t *testing.T) {
	app := testApp(t)
	user := record(t, app, "users", "habit-owner")
	habit := record(t, app, "habits", user.Id)
	server := &Server{App: app}
	for _, body := range []string{`{"completed":true}`, `{"completed":true}`, `{"notes":"A quiet day"}`} {
		e, _ := request(app, user, "PUT", "/api/habits/day", body, map[string]string{"id": habit.Id, "date": "2026-10-02"})
		if err := server.habitDay(e); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := app.FindAllRecords("habit_completions")
	if err != nil || len(rows) != 1 {
		t.Fatalf("duplicate habit days: %v %v", len(rows), err)
	}
	if !rows[0].GetBool("is_completed") || rows[0].GetString("notes") != "A quiet day" {
		t.Fatal("notes update lost completion")
	}
}
func TestDeleteRestoreAndDuplicate(t *testing.T) {
	app := testApp(t)
	user := record(t, app, "users", "entry-owner")
	entry := record(t, app, "journal", user.Id)
	server := &Server{App: app}
	d := Document{Version: 1, Value: []Node{{"type": "p", "children": []any{Node{"text": "Unique reflection"}}}}, AudioFileIDs: []string{}}
	if err := SaveDocument(app, entry, d, 0); err != nil {
		t.Fatal(err)
	}
	e, res := request(app, user, "POST", "/delete", "", map[string]string{"kind": "journal", "id": entry.Id})
	if err := server.remove(e); err != nil {
		t.Fatal(err)
	}
	_ = res
	e, res = request(app, user, "GET", "/api/search?q=Unique", "", nil)
	if err := server.search(e); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Body.String(), "Unique") {
		t.Fatal("deleted entry searchable")
	}
	e, _ = request(app, user, "POST", "/restore", "", map[string]string{"kind": "journal", "id": entry.Id})
	if err := server.restore(e); err != nil {
		t.Fatal(err)
	}
	e, res = request(app, user, "GET", "/api/search?q=Unique", "", nil)
	if err := server.search(e); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Body.String(), entry.Id) {
		t.Fatal("restored entry missing from search")
	}
	e, _ = request(app, user, "POST", "/duplicate", "", map[string]string{"kind": "journal", "id": entry.Id})
	if err := server.duplicate(e); err != nil {
		t.Fatal(err)
	}
	rows, _ := app.FindAllRecords("journal")
	if len(rows) != 2 {
		t.Fatal("duplicate not saved")
	}
	for _, row := range rows {
		if !strings.Contains(row.GetString("content_text"), "Unique reflection") {
			t.Fatal("duplicate lost content")
		}
	}
}
