package backend

import (
	"errors"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct{ App core.App }

var ErrConflict = errors.New("document changed in another session")

func Register(app core.App) *Server {
	s := &Server{App: app}
	names := []string{"journal", "goals", "tasks", "habits", "folders", "tags", "files", "settings", "habit_completions"}
	guard := func(e *core.RecordRequestEvent) error {
		if e.Auth == nil {
			return e.UnauthorizedError("Sign in required", nil)
		}
		if !e.Record.IsNew() && e.Record.Original().GetString("user") != e.Auth.Id {
			return e.ForbiddenError("Record unavailable", nil)
		}
		e.Record.Set("user", e.Auth.Id)
		if err := validateRelations(e.App, e.Record, e.Auth.Id); err != nil {
			return e.BadRequestError(err.Error(), nil)
		}
		if _, ok := entityKind(e.Record.Collection().Name); ok {
			if !e.Record.IsNew() {
				return e.ForbiddenError("Use the application save API", nil)
			}
			e.Record.Set("revision", 0)
			e.Record.Set("content_text", "")
			e.Record.Set("deleted_at", "")
			e.Record.Set("content", Document{Version: 1, Value: []Node{{"type": "p", "children": []any{Node{"text": ""}}}}, AudioFileIDs: []string{}})
		}
		return e.Next()
	}
	app.OnRecordCreateRequest(names...).BindFunc(guard)
	app.OnRecordUpdateRequest(names...).BindFunc(guard)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		r := e.Router.Group("/api")
		r.Bind(apis.RequireAuth("users"))
		r.GET("/entities/{kind}", s.list)
		r.GET("/entities/{kind}/{id}", s.get)
		r.POST("/entities/{kind}", s.create)
		r.PATCH("/entities/{kind}/{id}", s.patch)
		r.PUT("/entities/{kind}/{id}/document", s.document)
		r.POST("/entities/{kind}/{id}/delete", s.remove)
		r.POST("/entities/{kind}/{id}/restore", s.restore)
		r.POST("/entities/{kind}/{id}/duplicate", s.duplicate)
		r.GET("/entities/{kind}/{id}/relationships", s.relationships)
		r.POST("/tasks/reorder", s.reorder)
		r.PUT("/habits/{id}/days/{date}", s.habitDay)
		r.POST("/files/upload", s.upload)
		r.GET("/search", s.search)
		r.GET("/preferences", s.preferences)
		r.PATCH("/preferences", s.updatePreferences)
		if err := s.rebuildSearch(); err != nil {
			return err
		}
		app.Cron().MustAdd("cleanup", "15 * * * *", func() {
			if err := s.cleanup(); err != nil {
				app.Logger().Error("Cleanup failed", "error", err)
			}
		})
		return e.Next()
	})
	return s
}
func entityKind(table string) (string, bool) {
	for k, v := range collections {
		if v == table {
			return k, true
		}
	}
	return "", false
}
func validateRelations(app core.App, r *core.Record, user string) error {
	for _, f := range r.Collection().Fields {
		if rel, ok := f.(*core.RelationField); ok && rel.Name != "user" {
			for _, id := range r.GetStringSlice(rel.Name) {
				target, err := app.FindRecordById(rel.CollectionId, id)
				if err != nil || target.GetString("user") != user {
					return fmt.Errorf("invalid %s relationship", rel.Name)
				}
			}
		}
	}
	return nil
}
func (s *Server) record(e *core.RequestEvent) (*core.Record, error) {
	table, ok := collections[e.Request.PathValue("kind")]
	if !ok {
		return nil, e.NotFoundError("Unknown entity", nil)
	}
	r, err := owned(e.App, table, e.Request.PathValue("id"), e.Auth.Id)
	if err != nil {
		return nil, e.NotFoundError("Record unavailable", nil)
	}
	return r, nil
}
func response(e *core.RequestEvent, err error) error {
	if errors.Is(err, ErrConflict) {
		return e.JSON(409, map[string]any{"code": "revision_conflict", "message": err.Error()})
	}
	if err != nil {
		return e.BadRequestError(err.Error(), nil)
	}
	return nil
}
func DTO(app core.App, r *core.Record) map[string]any {
	kind, _ := entityKind(r.Collection().Name)
	out := r.PublicExport()
	out["kind"] = kind
	out["content"] = ReadDocument(r)
	out["title"] = r.GetString("title")
	if kind == "goal" && out["title"] == "" {
		out["title"] = r.GetString("goal")
	}
	tagNames := []string{}
	for _, id := range r.GetStringSlice("tags") {
		if t, err := app.FindRecordById("tags", id); err == nil {
			tagNames = append(tagNames, t.GetString("name"))
		}
	}
	out["tags"] = tagNames
	out["folderId"] = r.GetString("folder")
	out["folder"] = "Unfiled"
	if id := r.GetString("folder"); id != "" {
		if f, err := app.FindRecordById("folders", id); err == nil {
			out["folder"] = f.GetString("name")
		}
	}
	return out
}
func (s *Server) list(e *core.RequestEvent) error {
	table, ok := collections[e.Request.PathValue("kind")]
	if !ok {
		return e.NotFoundError("Unknown entity", nil)
	}
	page, _ := strconv.Atoi(e.Request.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	rows, err := e.App.FindRecordsByFilter(table, "user={:u} && deleted_at=''", "-created", 100, (page-1)*100, dbx.Params{"u": e.Auth.Id})
	if err != nil {
		return response(e, err)
	}
	items := []any{}
	for _, r := range rows {
		items = append(items, DTO(e.App, r))
	}
	return e.JSON(200, map[string]any{"items": items, "page": page, "hasMore": len(rows) == 100})
}
func (s *Server) get(e *core.RequestEvent) error {
	r, err := s.record(e)
	if err != nil {
		return err
	}
	return e.JSON(200, DTO(e.App, r))
}

type Mutation struct {
	BaseRevision int            `json:"baseRevision"`
	Patch        map[string]any `json:"patch"`
	Document     Document       `json:"document"`
}

func (s *Server) create(e *core.RequestEvent) error {
	table, ok := collections[e.Request.PathValue("kind")]
	if !ok {
		return e.NotFoundError("Unknown entity", nil)
	}
	var input map[string]any
	if err := e.BindBody(&input); err != nil {
		return response(e, err)
	}
	var created *core.Record
	err := e.App.RunInTransaction(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId(table)
		if err != nil {
			return err
		}
		r := core.NewRecord(c)
		r.Set("user", e.Auth.Id)
		r.Set("title", "Untitled")
		r.Set("status", map[string]string{"tasks": "todo", "goals": "active"}[table])
		r.Set("cadence", "daily")
		r.Set("date", time.Now().Format("2006-01-02"))
		r.Set("content", Document{Version: 1, Value: []Node{{"type": "p", "children": []any{Node{"text": ""}}}}, AudioFileIDs: []string{}})
		if err = s.applyPatch(app, r, input, e.Auth.Id); err != nil {
			return err
		}
		if err = app.Save(r); err != nil {
			return err
		}
		created = r
		return indexEntity(app, r)
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(201, DTO(e.App, created))
}
func (s *Server) applyPatch(app core.App, r *core.Record, patch map[string]any, user string) error {
	allowed := map[string]bool{"title": true, "status": true, "progress": true, "cadence": true, "priority": true, "effort": true, "date": true, "due_on": true, "goal": true, "goals": true, "tasks": true, "habits": true, "position": true}
	for k, v := range patch {
		if allowed[k] && r.Collection().Fields.GetByName(k) != nil {
			r.Set(k, v)
		}
		if k == "tags" {
			ids := []string{}
			items, ok := v.([]any)
			if !ok {
				return fmt.Errorf("invalid tags")
			}
			for _, n := range items {
				name, ok := n.(string)
				if !ok || len(strings.TrimSpace(name)) == 0 {
					return fmt.Errorf("invalid tag")
				}
				tag, err := namedRecord(app, "tags", user, strings.TrimSpace(name))
				if err != nil {
					return err
				}
				ids = append(ids, tag.Id)
			}
			r.Set("tags", ids)
		}
		if k == "folder" {
			name, _ := v.(string)
			if name == "" || name == "Unfiled" {
				r.Set("folder", "")
			} else {
				folder, err := namedRecord(app, "folders", user, name)
				if err != nil {
					return err
				}
				r.Set("folder", folder.Id)
			}
		}
	}
	for _, name := range []string{"date", "due_on"} {
		if v := r.GetString(name); v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return fmt.Errorf("invalid %s", name)
			}
		}
	}
	return validateRelations(app, r, user)
}
func namedRecord(app core.App, table, user, name string) (*core.Record, error) {
	rows, err := app.FindRecordsByFilter(table, "user={:u} && name={:n}", "", 1, 0, dbx.Params{"u": user, "n": name})
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows[0], nil
	}
	c, err := app.FindCollectionByNameOrId(table)
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(c)
	r.Set("user", user)
	r.Set("name", name)
	err = app.Save(r)
	return r, err
}
func (s *Server) patch(e *core.RequestEvent) error {
	r, err := s.record(e)
	if err != nil {
		return err
	}
	var input Mutation
	if err = e.BindBody(&input); err != nil {
		return response(e, err)
	}
	err = e.App.RunInTransaction(func(app core.App) error {
		fresh, err := app.FindRecordById(r.Collection(), r.Id)
		if err != nil {
			return err
		}
		if fresh.GetInt("revision") != input.BaseRevision {
			return ErrConflict
		}
		if err = s.applyPatch(app, fresh, input.Patch, e.Auth.Id); err != nil {
			return err
		}
		fresh.Set("revision", fresh.GetInt("revision")+1)
		if err = app.Save(fresh); err != nil {
			return err
		}
		r = fresh
		return indexEntity(app, fresh)
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, DTO(e.App, r))
}
func SaveDocument(app core.App, r *core.Record, d Document, revision int) error {
	if r.GetInt("revision") != revision {
		return ErrConflict
	}
	text, refs, files, err := ValidateDocument(app, d, r.GetString("user"))
	if err != nil {
		return err
	}
	r.Set("content", d)
	r.Set("content_text", text)
	r.Set("revision", revision+1)
	if err = app.Save(r); err != nil {
		return err
	}
	kind, _ := entityKind(r.Collection().Name)
	for _, table := range []string{"document_references", "entity_files"} {
		rows, err := app.FindRecordsByFilter(table, "source_"+kind+"={:id}", "", 0, 0, dbx.Params{"id": r.Id})
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err = app.Delete(row); err != nil {
				return err
			}
		}
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		key := ref.Kind + ref.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		c, _ := app.FindCollectionByNameOrId("document_references")
		row := core.NewRecord(c)
		row.Set("user", r.GetString("user"))
		row.Set("source_"+kind, r.Id)
		row.Set("target_"+ref.Kind, ref.ID)
		if err = app.Save(row); err != nil {
			return err
		}
	}
	for _, f := range files {
		c, _ := app.FindCollectionByNameOrId("entity_files")
		row := core.NewRecord(c)
		row.Set("user", r.GetString("user"))
		row.Set("source_"+kind, r.Id)
		row.Set("file", f.File)
		row.Set("placement_id", f.ID)
		row.Set("location", f.Location)
		if err = app.Save(row); err != nil {
			return err
		}
	}
	jobs, err := app.FindRecordsByFilter("transcriptions", kind+"={:id} && (status='queued' || status='transcribing' || status='cleaning')", "", 0, 0, dbx.Params{"id": r.Id})
	if err != nil {
		return err
	}
	for _, job := range jobs {
		attached := false
		for _, f := range files {
			if f.File == job.GetString("file") {
				attached = true
				break
			}
		}
		if !attached {
			job.Set("status", "canceled")
			if err := app.Save(job); err != nil {
				return err
			}
		}
	}
	return indexEntity(app, r)
}
func (s *Server) document(e *core.RequestEvent) error {
	r, err := s.record(e)
	if err != nil {
		return err
	}
	var input Mutation
	if err = e.BindBody(&input); err != nil {
		return response(e, err)
	}
	err = e.App.RunInTransaction(func(app core.App) error {
		fresh, err := app.FindRecordById(r.Collection(), r.Id)
		if err != nil {
			return err
		}
		if fresh.GetString("deleted_at") != "" {
			return fmt.Errorf("record is deleted")
		}
		if err = SaveDocument(app, fresh, input.Document, input.BaseRevision); err != nil {
			return err
		}
		r = fresh
		return nil
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, DTO(e.App, r))
}
func (s *Server) deleted(e *core.RequestEvent, restore bool) error {
	r, err := s.record(e)
	if err != nil {
		return err
	}
	err = e.App.RunInTransaction(func(app core.App) error {
		r, err = app.FindRecordById(r.Collection(), r.Id)
		if err != nil {
			return err
		}
		value := ""
		if !restore {
			value = time.Now().UTC().Format(time.RFC3339)
		}
		if !restore {
			kind, _ := entityKind(r.Collection().Name)
			jobs, err := app.FindRecordsByFilter("transcriptions", kind+"={:id} && (status='queued' || status='transcribing' || status='cleaning')", "", 0, 0, dbx.Params{"id": r.Id})
			if err != nil {
				return err
			}
			for _, job := range jobs {
				job.Set("status", "canceled")
				if err := app.Save(job); err != nil {
					return err
				}
			}
		}
		r.Set("deleted_at", value)
		r.Set("revision", r.GetInt("revision")+1)
		if err = app.Save(r); err != nil {
			return err
		}
		return indexEntity(app, r)
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, DTO(e.App, r))
}
func (s *Server) remove(e *core.RequestEvent) error  { return s.deleted(e, false) }
func (s *Server) restore(e *core.RequestEvent) error { return s.deleted(e, true) }
func (s *Server) duplicate(e *core.RequestEvent) error {
	r, err := s.record(e)
	if err != nil {
		return err
	}
	var copy *core.Record
	err = e.App.RunInTransaction(func(app core.App) error {
		copy = core.NewRecord(r.Collection())
		for _, f := range r.Collection().Fields {
			if f.GetName() != "id" && f.GetName() != "created" && f.GetName() != "updated" {
				copy.Set(f.GetName(), r.Get(f.GetName()))
			}
		}
		copy.Set("title", r.GetString("title")+" (copy)")
		copy.Set("revision", 0)
		copy.Set("deleted_at", "")
		if err := app.Save(copy); err != nil {
			return err
		}
		d := ReadDocument(r)
		renamePlacements(d.Value)
		renamePlacements(d.Media)
		return SaveDocument(app, copy, d, 0)
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(201, DTO(e.App, copy))
}
func renamePlacements(nodes []Node) {
	for _, n := range nodes {
		if _, ok := n["placementId"]; ok {
			n["placementId"] = fmt.Sprintf("%d-%s", time.Now().UnixNano(), n["placementId"])
		}
		if children, ok := n["children"].([]any); ok {
			for _, child := range children {
				if m, ok := child.(map[string]any); ok {
					renamePlacements([]Node{Node(m)})
				}
			}
		}
	}
}
func (s *Server) relationships(e *core.RequestEvent) error {
	r, err := s.record(e)
	if err != nil {
		return err
	}
	kind, _ := entityKind(r.Collection().Name)
	rows, err := e.App.FindRecordsByFilter("document_references", "user={:u} && (source_"+kind+"={:id} || target_"+kind+"={:id})", "", 0, 0, dbx.Params{"u": e.Auth.Id, "id": r.Id})
	if err != nil {
		return response(e, err)
	}
	edges := []map[string]any{}
	seen := map[string]bool{}
	add := func(sourceKind, sourceID, targetKind, targetID string, manual bool) {
		key := sourceKind + ":" + sourceID + ":" + targetKind + ":" + targetID
		if seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, map[string]any{"source_" + sourceKind: sourceID, "target_" + targetKind: targetID, "manual": manual})
	}
	for _, row := range rows {
		for sourceKind := range collections {
			for targetKind := range collections {
				if sourceID, targetID := row.GetString("source_"+sourceKind), row.GetString("target_"+targetKind); sourceID != "" && targetID != "" {
					add(sourceKind, sourceID, targetKind, targetID, false)
				}
			}
		}
	}
	for sourceKind, table := range collections {
		candidates, err := e.App.FindRecordsByFilter(table, "user={:u} && deleted_at=''", "", 0, 0, dbx.Params{"u": e.Auth.Id})
		if err != nil {
			return response(e, err)
		}
		for _, source := range candidates {
			for _, field := range source.Collection().Fields {
				relation, ok := field.(*core.RelationField)
				if !ok {
					continue
				}
				for targetKind, targetTable := range collections {
					collection, _ := e.App.FindCollectionByNameOrId(targetTable)
					if relation.CollectionId != collection.Id {
						continue
					}
					for _, targetID := range source.GetStringSlice(relation.Name) {
						if (sourceKind == kind && source.Id == r.Id) || (targetKind == kind && targetID == r.Id) {
							if target, err := owned(e.App, targetTable, targetID, e.Auth.Id); err == nil && target.GetString("deleted_at") == "" {
								add(sourceKind, source.Id, targetKind, targetID, true)
							}
						}
					}
				}
			}
		}
	}
	return e.JSON(200, edges)
}
func (s *Server) reorder(e *core.RequestEvent) error {
	var input struct {
		Tasks []struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Position int    `json:"position"`
			Revision int    `json:"revision"`
		} `json:"tasks"`
	}
	if err := e.BindBody(&input); err != nil {
		return response(e, err)
	}
	if len(input.Tasks) > 1000 {
		return response(e, fmt.Errorf("too many tasks"))
	}
	items := []any{}
	err := e.App.RunInTransaction(func(app core.App) error {
		seen := map[string]bool{}
		for _, item := range input.Tasks {
			if seen[item.ID] {
				return fmt.Errorf("duplicate task")
			}
			seen[item.ID] = true
			r, err := owned(app, "tasks", item.ID, e.Auth.Id)
			if err != nil {
				return err
			}
			if r.GetInt("revision") != item.Revision {
				return ErrConflict
			}
			r.Set("status", item.Status)
			r.Set("position", item.Position)
			r.Set("revision", item.Revision+1)
			if err = app.Save(r); err != nil {
				return err
			}
			items = append(items, DTO(app, r))
			if err = indexEntity(app, r); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, items)
}
func (s *Server) habitDay(e *core.RequestEvent) error {
	habit, err := owned(e.App, "habits", e.Request.PathValue("id"), e.Auth.Id)
	if err != nil {
		return e.NotFoundError("Habit unavailable", nil)
	}
	date := e.Request.PathValue("date")
	if _, err = time.Parse("2006-01-02", date); err != nil {
		return response(e, err)
	}
	var input struct {
		Completed *bool   `json:"completed"`
		Notes     *string `json:"notes"`
	}
	if err = e.BindBody(&input); err != nil {
		return response(e, err)
	}
	var row *core.Record
	err = e.App.RunInTransaction(func(app core.App) error {
		rows, err := app.FindRecordsByFilter("habit_completions", "user={:u} && habit={:h} && date={:d}", "", 1, 0, dbx.Params{"u": e.Auth.Id, "h": habit.Id, "d": date})
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			row = rows[0]
		} else {
			c, _ := app.FindCollectionByNameOrId("habit_completions")
			row = core.NewRecord(c)
			row.Set("user", e.Auth.Id)
			row.Set("habit", habit.Id)
			row.Set("date", date)
		}
		if input.Completed != nil {
			row.Set("is_completed", *input.Completed)
		}
		if input.Notes != nil {
			row.Set("notes", htmlEscape(*input.Notes))
		}
		return app.Save(row)
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, row)
}
func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
func (s *Server) upload(e *core.RequestEvent) error {
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, 26*1024*1024)
	files, err := e.FindUploadedFiles("file")
	if err != nil || len(files) != 1 {
		return e.BadRequestError("Upload one file at a time (25 MiB maximum)", nil)
	}
	file := files[0]
	c, err := e.App.FindCollectionByNameOrId("files")
	if err != nil {
		return response(e, err)
	}
	r := core.NewRecord(c)
	r.Set("file", file)
	r.Set("user", e.Auth.Id)
	r.Set("name", file.OriginalName)
	reader, err := file.Reader.Open()
	if err != nil {
		return response(e, err)
	}
	mime, err := mimetype.DetectReader(reader)
	reader.Close()
	if err != nil {
		return response(e, err)
	}
	r.Set("mimetype", mime.String())
	r.Set("size", file.Size)
	if err = e.App.Save(r); err != nil {
		return response(e, err)
	}
	return e.JSON(201, r)
}
func Preferences(app core.App, user string) (*core.Record, error) {
	rows, err := app.FindRecordsByFilter("settings", "user={:u}", "", 1, 0, dbx.Params{"u": user})
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows[0], nil
	}
	c, err := app.FindCollectionByNameOrId("settings")
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(c)
	r.Set("user", user)
	r.Set("enable_ai", true)
	r.Set("enable_transcription", true)
	r.Set("autocomplete", true)
	r.Set("cleanup", true)
	r.Set("timezone", "UTC")
	err = app.Save(r)
	return r, err
}
func (s *Server) preferences(e *core.RequestEvent) error {
	var r *core.Record
	err := e.App.RunInTransaction(func(app core.App) error { var err error; r, err = Preferences(app, e.Auth.Id); return err })
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, r)
}
func (s *Server) updatePreferences(e *core.RequestEvent) error {
	var input map[string]any
	if err := e.BindBody(&input); err != nil {
		return response(e, err)
	}
	var r *core.Record
	err := e.App.RunInTransaction(func(app core.App) error {
		var err error
		r, err = Preferences(app, e.Auth.Id)
		if err != nil {
			return err
		}
		for _, k := range []string{"enable_ai", "enable_transcription", "autocomplete", "cleanup", "timezone"} {
			if v, ok := input[k]; ok {
				r.Set(k, v)
			}
		}
		if _, err = time.LoadLocation(r.GetString("timezone")); err != nil {
			return fmt.Errorf("invalid timezone")
		}
		return app.Save(r)
	})
	if err != nil {
		return response(e, err)
	}
	return e.JSON(200, r)
}
