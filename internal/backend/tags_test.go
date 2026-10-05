package backend

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestTagRemovalPrunesOnlyAfterLastUse(t *testing.T) {
	app := testApp(t)
	user := record(t, app, "users", "tag-owner")
	other := record(t, app, "users", "other-owner")
	tag, err := namedRecord(app, "tags", user.Id, "shared")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := namedRecord(app, "tags", other.Id, "unused-other-owner")
	if err != nil {
		t.Fatal(err)
	}
	journal := record(t, app, "journal", user.Id)
	habit := record(t, app, "habits", user.Id)
	for _, entry := range []*core.Record{journal, habit} {
		entry.Set("tags", []string{tag.Id})
		if err := app.Save(entry); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{App: app}
	for i, entry := range []*core.Record{journal, habit} {
		kind, _ := entityKind(entry.Collection().Name)
		event, res := request(app, user, "PATCH", "/api/entities/"+kind+"/"+entry.Id, `{"baseRevision":0,"patch":{"tags":[]}}`, map[string]string{"kind": kind, "id": entry.Id})
		if err := server.patch(event); err != nil {
			t.Fatal(err)
		}
		if res.Code != 200 {
			t.Fatalf("status %d", res.Code)
		}
		_, err := app.FindRecordById("tags", tag.Id)
		if i == 0 && err != nil {
			t.Fatal("tag deleted while still used by habit")
		}
		if i == 1 && err == nil {
			t.Fatal("tag retained after last use removed")
		}
	}
	if _, err := app.FindRecordById("tags", foreign.Id); err != nil {
		t.Fatal("another user's tags pruned by this user's save")
	}
}

func TestTagCleanupPreservesEveryEntityKindAndUndo(t *testing.T) {
	app := testApp(t)
	user := record(t, app, "users", "cleanup-owner")
	Register(app)
	unused, err := namedRecord(app, "tags", user.Id, "unused")
	if err != nil {
		t.Fatal(err)
	}
	used := []*core.Record{}
	entries := []*core.Record{}
	for _, table := range collections {
		tag, err := namedRecord(app, "tags", user.Id, table)
		if err != nil {
			t.Fatal(err)
		}
		entry := record(t, app, table, user.Id)
		entry.Set("tags", []string{tag.Id})
		entry.Set("deleted_at", "2026-10-01 12:00:00.000Z")
		if err := app.Save(entry); err != nil {
			t.Fatal(err)
		}
		used = append(used, tag)
		entries = append(entries, entry)
	}
	if err := app.RunInTransaction(func(tx core.App) error { return pruneUnusedTags(tx, "") }); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindRecordById("tags", unused.Id); err == nil {
		t.Fatal("existing unused tag retained")
	}
	for _, tag := range used {
		if _, err := app.FindRecordById("tags", tag.Id); err != nil {
			t.Fatal("tag required for restore was deleted")
		}
	}
	for i, entry := range entries {
		if err := app.Delete(entry); err != nil {
			t.Fatal(err)
		}
		if _, err := app.FindRecordById("tags", used[i].Id); err == nil {
			t.Fatal("permanent deletion left unused tag")
		}
	}
}

func TestTagPruningFailureRollsBackTheSave(t *testing.T) {
	app := testApp(t)
	user := record(t, app, "users", "rollback-owner")
	tag, err := namedRecord(app, "tags", user.Id, "keep-on-failure")
	if err != nil {
		t.Fatal(err)
	}
	entry := record(t, app, "journal", user.Id)
	entry.Set("tags", []string{tag.Id})
	if err := app.Save(entry); err != nil {
		t.Fatal(err)
	}
	app.OnRecordDelete("tags").BindFunc(func(e *core.RecordEvent) error { return errors.New("test deletion failure") })
	event, _ := request(app, user, "PATCH", "/api/entities/journal/"+entry.Id, `{"baseRevision":0,"patch":{"tags":[]}}`, map[string]string{"kind": "journal", "id": entry.Id})
	if err := (&Server{App: app}).patch(event); err == nil {
		t.Fatal("expected pruning failure")
	}
	fresh, err := app.FindRecordById("journal", entry.Id)
	if err != nil || len(fresh.GetStringSlice("tags")) != 1 || fresh.GetInt("revision") != 0 {
		t.Fatal("failed pruning partially saved tag removal")
	}
	if _, err := app.FindRecordById("tags", tag.Id); err != nil {
		t.Fatal("failed pruning removed tag")
	}
}
