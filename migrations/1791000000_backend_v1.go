package migrations

import (
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
	"strings"
)

func init() {
	m.Register(func(app core.App) error {
		ids := map[string]string{}
		for _, name := range []string{"users", "journal", "goals", "tasks", "habits", "files", "folders"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			ids[name] = c.Id
		}
		for _, name := range []string{"journal", "goals", "tasks", "habits"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			c.Fields.Add(&core.JSONField{Name: "content", MaxSize: 1048576}, &core.TextField{Name: "content_text"}, &core.NumberField{Name: "revision", OnlyInt: true}, &core.DateField{Name: "deleted_at"})
			if name == "journal" {
				c.Fields.Add(&core.TextField{Name: "date", Pattern: `^\d{4}-\d{2}-\d{2}$`}, &core.RelationField{Name: "folder", CollectionId: ids["folders"], MaxSelect: 1})
			}
			if name == "goals" {
				c.Fields.Add(&core.TextField{Name: "title"}, &core.SelectField{Name: "status", Values: []string{"active", "completed"}, MaxSelect: 1}, &core.NumberField{Name: "progress", Min: types.Pointer(0.0), Max: types.Pointer(100.0)})
			}
			if name == "tasks" {
				c.Fields.Add(&core.TextField{Name: "due_on", Pattern: `^\d{4}-\d{2}-\d{2}$`}, &core.NumberField{Name: "position", OnlyInt: true})
			}
			rule := "@request.auth.id != '' && user = @request.auth.id && deleted_at = ''"
			c.ListRule = types.Pointer(rule)
			c.ViewRule = types.Pointer("@request.auth.id != '' && user = @request.auth.id")
			c.DeleteRule = nil
			if err = app.Save(c); err != nil {
				return err
			}
			records, err := app.FindAllRecords(c)
			if err != nil {
				return err
			}
			for _, r := range records {
				if name == "goals" {
					r.Set("title", r.GetString("goal"))
					r.Set("status", "active")
				}
				if name == "journal" {
					s := r.GetString("created")
					if len(s) >= 10 {
						r.Set("date", s[:10])
					}
				}
				if err = app.Save(r); err != nil {
					return err
				}
			}
		}
		// Preserve legacy folder membership; children remains available for compatibility.
		folderRows, err := app.FindAllRecords("folders")
		if err != nil {
			return err
		}
		for _, folder := range folderRows {
			for _, id := range folder.GetStringSlice("children") {
				entry, err := app.FindRecordById("journal", id)
				if err != nil {
					continue
				}
				if entry.GetString("user") == folder.GetString("user") && entry.GetString("folder") == "" {
					entry.Set("folder", folder.Id)
					if err = app.Save(entry); err != nil {
						return err
					}
				}
			}
		}
		for _, name := range []string{"document_references", "entity_files"} {
			c := core.NewBaseCollection(name)
			c.Fields.Add(&core.RelationField{Name: "user", CollectionId: ids["users"], Required: true, MaxSelect: 1})
			for _, kind := range []string{"journal", "task", "goal", "habit"} {
				table := map[string]string{"journal": "journal", "task": "tasks", "goal": "goals", "habit": "habits"}[kind]
				c.Fields.Add(&core.RelationField{Name: "source_" + kind, CollectionId: ids[table], MaxSelect: 1})
				if name == "document_references" {
					c.Fields.Add(&core.RelationField{Name: "target_" + kind, CollectionId: ids[table], MaxSelect: 1})
				}
			}
			if name == "entity_files" {
				c.Fields.Add(&core.RelationField{Name: "file", CollectionId: ids["files"], Required: true, MaxSelect: 1}, &core.TextField{Name: "placement_id"}, &core.TextField{Name: "location"})
			}
			c.ListRule = types.Pointer("@request.auth.id != '' && user = @request.auth.id")
			c.ViewRule = c.ListRule
			if err := app.Save(c); err != nil {
				return err
			}
		}
		c, err := app.FindCollectionByNameOrId("files")
		if err != nil {
			return err
		}
		c.Fields.Add(&core.NumberField{Name: "size"}, &core.NumberField{Name: "duration"})
		c.CreateRule = nil
		c.UpdateRule = nil
		c.DeleteRule = nil
		if err = app.Save(c); err != nil {
			return err
		}
		c, err = app.FindCollectionByNameOrId("settings")
		if err != nil {
			return err
		}
		c.Fields.Add(&core.TextField{Name: "timezone"}, &core.BoolField{Name: "autocomplete"}, &core.BoolField{Name: "cleanup"})
		if err = deduplicate(app, c, []string{"user"}); err != nil {
			return err
		}
		c.Indexes = append(c.Indexes, "CREATE UNIQUE INDEX idx_settings_user ON settings (user)")
		if err = app.Save(c); err != nil {
			return err
		}
		c, err = app.FindCollectionByNameOrId("habit_completions")
		if err != nil {
			return err
		}
		c.Fields.Add(&core.TextField{Name: "date", Pattern: `^\d{4}-\d{2}-\d{2}$`})
		if err = app.Save(c); err != nil {
			return err
		}
		records, err := app.FindAllRecords(c)
		if err != nil {
			return err
		}
		for _, r := range records {
			s := r.GetString("sent")
			if s == "" {
				s = r.GetString("created")
			}
			if len(s) >= 10 {
				r.Set("date", s[:10])
			}
			if err = app.Save(r); err != nil {
				return err
			}
		}
		if err = deduplicate(app, c, []string{"user", "habit", "date"}); err != nil {
			return err
		}
		c.Indexes = append(c.Indexes, "CREATE UNIQUE INDEX idx_habit_day ON habit_completions (user,habit,date)")
		if err = app.Save(c); err != nil {
			return err
		}
		c, err = app.FindCollectionByNameOrId("transcriptions")
		if err != nil {
			return err
		}
		if f := c.Fields.GetByName("transcription"); f != nil {
			if ef, ok := f.(*core.EditorField); ok {
				ef.Required = false
			}
		}
		c.Fields.Add(&core.SelectField{Name: "status", Values: []string{"queued", "transcribing", "cleaning", "completed", "failed", "canceled"}, MaxSelect: 1}, &core.TextField{Name: "language"}, &core.TextField{Name: "raw_text"}, &core.TextField{Name: "cleaned_text"}, &core.JSONField{Name: "segments"}, &core.TextField{Name: "cleanup_outcome"}, &core.TextField{Name: "error"}, &core.TextField{Name: "anchor_id"}, &core.TextField{Name: "request_key"}, &core.BoolField{Name: "applied"}, &core.NumberField{Name: "attempts", OnlyInt: true}, &core.RelationField{Name: "goal", CollectionId: ids["goals"], MaxSelect: 1}, &core.RelationField{Name: "habit", CollectionId: ids["habits"], MaxSelect: 1})
		c.CreateRule = nil
		c.UpdateRule = nil
		c.DeleteRule = nil
		c.Indexes = append(c.Indexes, "CREATE UNIQUE INDEX idx_transcription_request ON transcriptions (user,request_key) WHERE request_key != ''")
		if err = app.Save(c); err != nil {
			return err
		}
		// FTS is derived and can be rebuilt without changing primary records.
		_, err = app.DB().NewQuery("CREATE VIRTUAL TABLE IF NOT EXISTS content_search USING fts5(user UNINDEXED, kind UNINDEXED, entity_id UNINDEXED, placement_id UNINDEXED, title, body, tokenize='unicode61')").Execute()
		return err
	}, nil)
}

func deduplicate(app core.App, c *core.Collection, keys []string) error {
	records, err := app.FindAllRecords(c)
	if err != nil {
		return err
	}
	seen := map[string]*core.Record{}
	for _, r := range records {
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, r.GetString(k))
		}
		key := strings.Join(parts, "\x00")
		if old := seen[key]; old != nil {
			if n := r.GetString("notes"); n != "" {
				old.Set("notes", old.GetString("notes")+"\n"+n)
			}
			if r.GetBool("is_completed") {
				old.Set("is_completed", true)
			}
			if err := app.Save(old); err != nil {
				return err
			}
			app.Logger().Warn("Merged duplicate record", "collection", c.Name, "id", r.Id, "into", old.Id)
			if err := app.Delete(r); err != nil {
				return fmt.Errorf("merge %s: %w", c.Name, err)
			}
		} else {
			seen[key] = r
		}
	}
	return nil
}
