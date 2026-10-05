package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chand1012/lyphe/internal/backend"
	_ "github.com/chand1012/lyphe/migrations"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type fixture struct {
	app         core.App
	backend     *backend.Server
	server      *Server
	handler     http.Handler
	user, other *core.Record
	token       string
	credential  Credential
}

func setup(t *testing.T) *fixture {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	f := &fixture{app: app}
	f.backend = backend.Register(app)
	f.server = Register(app, f.backend)
	f.user = newUser(t, app, "first")
	f.other = newUser(t, app, "second")
	var err error
	f.credential, f.token, err = IssueToken(app, f.user.Id, "test agent", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	event := &core.ServeEvent{App: app, Router: r}
	if err = app.OnServe().Trigger(event); err != nil {
		t.Fatal(err)
	}
	// Model the main SPA catch-all to prove specific MCP/API routes win.
	r.GET("/{path...}", func(e *core.RequestEvent) error { return e.String(200, "SPA") })
	f.handler, err = r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func newUser(t *testing.T, app core.App, name string) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	r.Set("email", name+"@example.com")
	r.SetPassword("test-password-123")
	if err = app.Save(r); err != nil {
		t.Fatal(err)
	}
	return r
}
func newFile(t *testing.T, app core.App, user string) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId("files")
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	r.Set("user", user)
	r.Set("name", "Private file")
	r.Set("mimetype", "text/plain")
	file, err := filesystem.NewFileFromBytes([]byte("Private attachment"), "private.txt")
	if err != nil {
		t.Fatal(err)
	}
	r.Set("file", file)
	if err = app.Save(r); err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) request(method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://127.0.0.1:8090"+path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

// Exercise the actual HTTP router without opening a listening socket.
type routerTransport struct {
	handler http.Handler
	token   string
}

func (rt routerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+rt.token)
	rec := httptest.NewRecorder()
	rt.handler.ServeHTTP(rec, clone)
	res := rec.Result()
	res.Request = clone
	return res, nil
}
func (f *fixture) client(t *testing.T, token string) *sdk.ClientSession {
	t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: "http://127.0.0.1:8090/mcp", HTTPClient: &http.Client{Transport: routerTransport{f.handler, token}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}
func call(t *testing.T, client *sdk.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res, err := client.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("%s failed: %+v", name, res.Content)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	if err = json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func failed(t *testing.T, client *sdk.ClientSession, name string, args map[string]any) {
	t.Helper()
	res, err := client.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err == nil && !res.IsError {
		t.Fatalf("%s unexpectedly succeeded", name)
	}
}
func create(t *testing.T, f *fixture, user, kind, title string) map[string]any {
	t.Helper()
	out, err := f.backend.Create(context.Background(), backend.Principal{UserID: user, Source: backend.Human}, kind, map[string]any{"title": title}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestHTTPToolsAndExistingData(t *testing.T) {
	f := setup(t)
	human := create(t, f, f.user.Id, "journal", "Human reflection")
	create(t, f, f.other.Id, "journal", "Private other reflection")
	create(t, f, f.user.Id, "task", "Task reflection excluded by kind")
	ownedFile := newFile(t, f.app, f.user.Id)
	journalRecord, _ := f.app.FindRecordById("journal", human["id"].(string))
	attached := backend.PlainDocument("Human attachment notes")
	attached.AudioFileIDs = []string{ownedFile.Id}
	if err := f.app.RunInTransaction(func(app core.App) error { return backend.SaveDocument(app, journalRecord, attached, 0) }); err != nil {
		t.Fatal(err)
	}
	client := f.client(t, f.token)
	tools, err := client.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 14 {
		t.Fatalf("tools: %v %v", tools, err)
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("missing tool annotations: %s", tool.Name)
		}
	}
	journal := call(t, client, "get_entity", map[string]any{"kind": "journal", "id": human["id"]})
	if len(journal["attachments"].([]any)) != 1 {
		t.Fatal("missing attachment metadata", journal)
	}
	if journal["title"] != "Human reflection" {
		t.Fatal(journal)
	}
	for _, kind := range []string{"task", "habit", "goal"} {
		item := call(t, client, "create_entity", map[string]any{"kind": kind, "patch": map[string]any{"title": "Agent " + kind, "tags": []string{"agent"}}, "text": "Initial notes"})
		id := item["id"].(string)
		rev := item["revision"]
		item = call(t, client, "update_entity", map[string]any{"kind": kind, "id": id, "baseRevision": rev, "patch": map[string]any{"title": "Edited " + kind}})
		failed(t, client, "update_entity", map[string]any{"kind": kind, "id": id, "baseRevision": rev, "patch": map[string]any{"title": "Stale"}})
		item = call(t, client, "append_to_document", map[string]any{"kind": kind, "id": id, "baseRevision": item["revision"], "text": "More notes"})
		call(t, client, "duplicate_entity", map[string]any{"kind": kind, "id": id, "baseRevision": item["revision"]})
		if kind == "habit" {
			for range 2 {
				call(t, client, "set_habit_day", map[string]any{"id": id, "date": "2026-10-05", "completed": true, "notes": "Today"})
			}
			history := call(t, client, "get_habit_history", map[string]any{"id": id})
			if len(history["items"].([]any)) != 1 {
				t.Fatal(history)
			}
		}
		item = call(t, client, "delete_entity", map[string]any{"kind": kind, "id": id, "baseRevision": item["revision"]})
		call(t, client, "restore_entity", map[string]any{"kind": kind, "id": id, "baseRevision": item["revision"]})
		listed := call(t, client, "list_entities", map[string]any{"kind": kind, "tag": "agent", "limit": 1})
		if len(listed["items"].([]any)) != 1 || listed["hasMore"] != true {
			t.Fatal(listed)
		}
		if _, ok := listed["items"].([]any)[0].(map[string]any)["content"]; ok {
			t.Fatal("full document returned in list")
		}
	}
	res, err := client.CallTool(context.Background(), &sdk.CallToolParams{Name: "search", Arguments: map[string]any{"query": "reflection", "kind": "journal"}})
	if err != nil || res.IsError {
		t.Fatal(res, err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	if !bytes.Contains(raw, []byte("Human reflection")) || bytes.Contains(raw, []byte("Private other")) || bytes.Contains(raw, []byte("Task reflection")) {
		t.Fatal(string(raw))
	}
	if f.request("GET", "/journal", "", "").Body.String() != "SPA" {
		t.Fatal("SPA fallback failed")
	}
}
func TestJournalWritesAndIdentitySpoofingFail(t *testing.T) {
	f := setup(t)
	journal := create(t, f, f.user.Id, "journal", "Human journal")
	id := journal["id"].(string)
	r, _ := f.app.FindRecordById("journal", id)
	before, _ := json.Marshal(r.PublicExport())
	client := f.client(t, f.token)
	for _, name := range []string{"create_entity", "update_entity", "save_document", "append_to_document", "duplicate_entity", "delete_entity", "restore_entity"} {
		args := map[string]any{"kind": "journal"}
		switch name {
		case "create_entity":
			args["patch"] = map[string]any{"title": "Agent"}
		default:
			args["id"] = id
			args["baseRevision"] = 0
		}
		if name == "update_entity" {
			args["patch"] = map[string]any{"title": "Agent"}
		}
		if name == "save_document" || name == "append_to_document" {
			args["text"] = "Agent"
		}
		failed(t, client, name, args)
	}
	failed(t, client, "update_entity", map[string]any{"kind": "task", "id": id, "baseRevision": 0, "patch": map[string]any{"title": "Forged kind"}})
	failed(t, client, "create_entity", map[string]any{"kind": "task", "user": f.other.Id})
	failed(t, client, "create_entity", map[string]any{"kind": "task", "patch": map[string]any{"user": f.other.Id}})
	failed(t, client, "create_entity", map[string]any{"kind": "task", "patch": map[string]any{"folder": "Journal folder"}})
	foreign := create(t, f, f.other.Id, "task", "Foreign")
	failed(t, client, "get_entity", map[string]any{"kind": "task", "id": foreign["id"]})
	failed(t, client, "update_entity", map[string]any{"kind": "task", "id": foreign["id"], "baseRevision": 0, "patch": map[string]any{"title": "Stolen"}})
	otherGoal := create(t, f, f.other.Id, "goal", "Foreign goal")
	failed(t, client, "create_entity", map[string]any{"kind": "task", "patch": map[string]any{"goal": otherGoal["id"]}})
	foreignMention := backend.Document{Version: 1, Value: []backend.Node{{"type": "p", "children": []any{backend.Node{"type": "entity-mention", "kind": "goal", "entityId": otherGoal["id"], "children": []any{backend.Node{"text": ""}}}}}}, AudioFileIDs: []string{}}
	failed(t, client, "create_entity", map[string]any{"kind": "task", "document": foreignMention})
	forgedFile := backend.Document{Version: 1, Value: []backend.Node{{"type": "file-attachment", "fileId": otherGoal["id"], "placementId": "fake", "children": []any{backend.Node{"text": ""}}}}, AudioFileIDs: []string{}}
	failed(t, client, "create_entity", map[string]any{"kind": "task", "document": forgedFile})
	failed(t, client, "set_habit_day", map[string]any{"id": id, "date": "2026-10-05", "completed": true})
	failed(t, client, "reorder_tasks", map[string]any{"tasks": []map[string]any{{"id": id, "status": "done", "position": 0, "revision": 0}}})
	// A valid journal mention updates only the writable source and derived references.
	document := backend.Document{Version: 1, Value: []backend.Node{{"type": "p", "children": []any{backend.Node{"type": "entity-mention", "kind": "journal", "entityId": id, "children": []any{backend.Node{"text": ""}}}}}}, AudioFileIDs: []string{}}
	item := call(t, client, "create_entity", map[string]any{"kind": "task", "document": document})
	edges := callArray(t, client, "get_relationships", map[string]any{"kind": "journal", "id": id})
	if len(edges) != 1 {
		t.Fatal(edges)
	}
	call(t, client, "append_to_document", map[string]any{"kind": "task", "id": item["id"], "baseRevision": item["revision"], "text": "Append"})
	r, _ = f.app.FindRecordById("journal", id)
	after, _ := json.Marshal(r.PublicExport())
	if !bytes.Equal(before, after) {
		t.Fatal("journal changed")
	}
}
func callArray(t *testing.T, c *sdk.ClientSession, name string, args map[string]any) []any {
	t.Helper()
	r, err := c.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil || r.IsError {
		t.Fatal(r, err)
	}
	raw, _ := json.Marshal(r.StructuredContent)
	var items []any
	if err = json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	return items
}
func TestCredentialsAreEndpointLimited(t *testing.T) {
	f := setup(t)
	for _, header := range []string{"", "Bearer wrong", "Bearer " + f.token + "x", "Basic " + f.token} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:8090/mcp", strings.NewReader(`{}`))
		req.Header.Set("Authorization", header)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != 401 {
			t.Fatalf("header %q: %d", header, rec.Code)
		}
	}
	for _, path := range []string{"/api/entities/task", "/api/search", "/api/mcp/tokens"} {
		if rec := f.request("GET", path, f.token, ""); rec.Code < 400 {
			t.Fatalf("credential accepted by %s: %d", path, rec.Code)
		}
	}
	if rec := f.request("POST", "/api/files/token", f.token, ""); rec.Code != 401 {
		t.Fatal("MCP token accepted for file tokens", rec.Code)
	}
	ownedTask := create(t, f, f.user.Id, "task", "Owned task")
	if rec := f.request("GET", "/api/collections/tasks/records/"+ownedTask["id"].(string), f.token, ""); rec.Code != 404 {
		t.Fatal("MCP token authenticated to PocketBase", rec.Code)
	}
	recList := f.request("GET", "/api/collections/tasks/records", f.token, "")
	var list struct {
		Items []any `json:"items"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &list); err != nil || len(list.Items) != 0 {
		t.Fatal("PocketBase list exposed data", recList.Body.String())
	}
	file := newFile(t, f.app, f.user.Id)
	path := "/api/files/files/" + file.Id + "/" + file.GetString("file")
	if rec := f.request("GET", path, f.token, ""); rec.Code != 404 {
		t.Fatal("MCP token read protected file", rec.Code)
	}
	if rec := f.request("GET", path+"?token="+f.token, "", ""); rec.Code != 404 {
		t.Fatal("MCP token used as a file token", rec.Code)
	}
	fileToken, err := f.user.NewFileToken()
	if err != nil {
		t.Fatal(err)
	}
	if rec := f.request("GET", path+"?token="+fileToken, "", ""); rec.Code != 200 || rec.Body.String() != "Private attachment" {
		t.Fatal("human protected file access failed", rec.Code)
	}
	// Human sign-in can manage credentials but cannot directly mutate the collection.
	auth, err := f.user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	rec := f.request("POST", "/api/mcp/tokens", auth, `{"name":"New"}`)
	if rec.Code != 201 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var issued struct {
		Credential Credential `json:"credential"`
		Token      string     `json:"token"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	stored, _ := f.app.FindRecordById("tokens", issued.Credential.ID)
	if stored.GetString("hash") == "" || strings.Contains(stored.GetString("hash"), issued.Token) {
		t.Fatal("secret storage")
	}
	rec = f.request("GET", "/api/mcp/tokens", auth, "")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), issued.Token) || strings.Contains(rec.Body.String(), stored.GetString("hash")) {
		t.Fatal(rec.Body.String())
	}
	for _, method := range []string{"POST", "PATCH", "DELETE"} {
		path := "/api/collections/tokens/records"
		if method != "POST" {
			path += "/" + issued.Credential.ID
		}
		if rec = f.request(method, path, auth, `{"purpose":"mcp"}`); rec.Code < 400 {
			t.Fatalf("direct credential mutation: %s %d", method, rec.Code)
		}
	}
	otherAuth, _ := f.other.NewAuthToken()
	if rec = f.request("DELETE", "/api/mcp/tokens/"+issued.Credential.ID, otherAuth, ""); rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	if rec = f.request("DELETE", "/api/mcp/tokens/"+issued.Credential.ID, auth, ""); rec.Code != 204 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	if rec = f.request("POST", "/mcp", issued.Token, `{}`); rec.Code != 401 {
		t.Fatal("revoked credential accepted")
	}
	stored, _ = f.app.FindRecordById("tokens", f.credential.ID)
	stored.Set("purpose", "")
	if err = f.app.Save(stored); err != nil {
		t.Fatal(err)
	}
	if f.request("POST", "/mcp", f.token, `{}`).Code != 401 {
		t.Fatal("legacy token accepted")
	}
	stored.Set("purpose", "mcp")
	stored.Set("expiration", time.Now().Add(-time.Hour))
	if err = f.app.Save(stored); err != nil {
		t.Fatal(err)
	}
	if f.request("POST", "/mcp", f.token, `{}`).Code != 401 {
		t.Fatal("expired token accepted")
	}
}
func TestOriginBodyLimitsAndProtocolVersions(t *testing.T) {
	f := setup(t)
	for _, origin := range []string{"https://evil.example", "null", "http://127.0.0.1:8090/path"} {
		req := httptest.NewRequest("POST", "http://127.0.0.1:8090/mcp", strings.NewReader(`{}`))
		req.Header.Set("Origin", origin)
		req.Header.Set("Authorization", "Bearer "+f.token)
		rec := httptest.NewRecorder()
		f.handler.ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Fatal(origin, rec.Code)
		}
	}
	rec := f.request("POST", "/mcp", f.token, strings.Repeat(" ", 2<<20)+`{}`)
	if rec.Code != 413 {
		t.Fatal("body limit", rec.Code, rec.Body.String())
	}
	for _, version := range []string{"2025-03-26", "2025-06-18", "2025-11-25"} {
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, version)
		rec = f.request("POST", "/mcp", f.token, body)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"protocolVersion":"`+version+`"`) {
			t.Fatal(version, rec.Code, rec.Body.String())
		}
	}
	// Current protocol can discover tools without a session/initialization handshake.
	req := httptest.NewRequest("POST", "http://127.0.0.1:8090/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}`))
	req.Header.Set("MCP-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "tools/list")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec = httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "get_entity") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestDataAndCredentialsSurviveRestart(t *testing.T) {
	f := setup(t)
	client := f.client(t, f.token)
	item := call(t, client, "create_entity", map[string]any{"kind": "habit", "patch": map[string]any{"title": "Persistent", "cadence": "weekly", "hour": 8, "minute": 30, "day": 1}, "text": "Saved notes"})
	call(t, client, "set_habit_day", map[string]any{"id": item["id"], "date": "2026-10-05", "completed": true})
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	dir := f.app.DataDir()
	f.app.ResetBootstrapState()
	restarted := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: dir})
	if err := restarted.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restarted.ResetBootstrapState() })
	if err := restarted.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	b := backend.Register(restarted)
	m := New(restarted, b)
	who, err := authenticate(restarted, "Bearer "+f.token)
	if err != nil || who.UserID != f.user.Id {
		t.Fatal(who, err)
	}
	f.app = restarted
	f.backend = b
	f.handler = m.Handler
	newClient := f.client(t, f.token)
	got := call(t, newClient, "get_entity", map[string]any{"kind": "habit", "id": item["id"]})
	if got["title"] != "Persistent" || !strings.Contains(got["content_text"].(string), "Saved notes") {
		t.Fatal(got)
	}
	history := call(t, newClient, "get_habit_history", map[string]any{"id": item["id"], "from": "2026-10-05", "to": "2026-10-05"})
	if len(history["items"].([]any)) != 1 {
		t.Fatal(history)
	}
}

func TestEveryRequestUsesItsOwnCredential(t *testing.T) {
	f := setup(t)
	first := create(t, f, f.user.Id, "task", "First user's task")
	second := create(t, f, f.other.Id, "task", "Second user's task")
	_, otherToken, err := IssueToken(f.app, f.other.Id, "Other agent", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	a := f.client(t, f.token)
	b := f.client(t, otherToken)
	call(t, a, "get_entity", map[string]any{"kind": "task", "id": first["id"]})
	call(t, b, "get_entity", map[string]any{"kind": "task", "id": second["id"]})
	failed(t, a, "get_entity", map[string]any{"kind": "task", "id": second["id"]})
	failed(t, b, "get_entity", map[string]any{"kind": "task", "id": first["id"]})
	row, _ := f.app.FindRecordById("tokens", f.credential.ID)
	if err = f.app.Delete(row); err != nil {
		t.Fatal(err)
	}
	// Even an already connected SDK client must stop after revocation.
	failed(t, a, "get_entity", map[string]any{"kind": "task", "id": first["id"]})
	call(t, b, "get_entity", map[string]any{"kind": "task", "id": second["id"]})
	req := httptest.NewRequest("POST", "http://127.0.0.1:8090/mcp", strings.NewReader(`{}`))
	req.Header.Set("Mcp-Session-Id", f.credential.ID)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatal("session ID substituted for credential", rec.Code)
	}
}
