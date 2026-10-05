package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Principal distinguishes human API calls from restricted MCP calls.
type Principal struct {
	UserID string
	Source AccessSource
}
type AccessSource string

const (
	Human AccessSource = "human"
	Agent AccessSource = "agent"
)

var ErrPermission = errors.New("agents cannot write journals")
var ErrUnavailable = errors.New("record unavailable")

func authorize(ctx context.Context, p Principal, kind string, write bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.UserID == "" || (p.Source != Human && p.Source != Agent) {
		return ErrUnavailable
	}
	if _, ok := collections[kind]; !ok {
		return ErrUnavailable
	}
	if write && p.Source == Agent && kind == "journal" {
		return ErrPermission
	}
	return nil
}
func entity(app core.App, p Principal, kind, id string) (*core.Record, error) {
	r, err := owned(app, collections[kind], id, p.UserID)
	if err != nil {
		return nil, ErrUnavailable
	}
	return r, nil
}
func (s *Server) Get(ctx context.Context, p Principal, kind, id string) (map[string]any, error) {
	if err := authorize(ctx, p, kind, false); err != nil {
		return nil, err
	}
	r, err := entity(s.App, p, kind, id)
	if err != nil {
		return nil, err
	}
	out := DTO(s.App, r)
	files := []map[string]any{}
	seen := map[string]bool{}
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		if f, err := owned(s.App, "files", id, p.UserID); err == nil {
			files = append(files, map[string]any{"id": f.Id, "name": f.GetString("name"), "mimetype": f.GetString("mimetype"), "size": f.GetInt("size")})
		}
	}
	d := ReadDocument(r)
	var walk func(Node)
	walk = func(n Node) {
		if id, ok := n["fileId"].(string); ok {
			add(id)
		}
		for _, c := range nodeChildren(n) {
			walk(c)
		}
	}
	for _, n := range append(d.Value, d.Media...) {
		walk(n)
	}
	for _, id := range d.AudioFileIDs {
		add(id)
	}
	out["attachments"] = files
	return out, nil
}

type ListOptions struct {
	Page           int    `json:"page,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Status         string `json:"status,omitempty"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Tag            string `json:"tag,omitempty"`
	Goal           string `json:"goal,omitempty"`
	IncludeDeleted bool   `json:"includeDeleted,omitempty"`
	Compact        bool   `json:"-"`
}
type PageResult struct {
	Items   []map[string]any `json:"items"`
	Page    int              `json:"page"`
	HasMore bool             `json:"hasMore"`
}

func pagination(page, limit int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 100
	}
	if page < 1 || page > 1000000 || limit < 1 || limit > 100 {
		return 0, 0, fmt.Errorf("invalid pagination")
	}
	return page, limit, nil
}
func (s *Server) List(ctx context.Context, p Principal, kind string, o ListOptions) (PageResult, error) {
	out := PageResult{Items: []map[string]any{}}
	if err := authorize(ctx, p, kind, false); err != nil {
		return out, err
	}
	page, limit, err := pagination(o.Page, o.Limit)
	if err != nil {
		return out, err
	}
	out.Page = page
	filter := "user={:u}"
	params := dbx.Params{"u": p.UserID}
	if !o.IncludeDeleted {
		filter += " && deleted_at=''"
	}
	c, err := s.App.FindCollectionByNameOrId(collections[kind])
	if err != nil {
		return out, err
	}
	add := func(field, key, value string) error {
		if value == "" {
			return nil
		}
		if c.Fields.GetByName(field) == nil {
			return fmt.Errorf("filter %s does not apply to %s", field, kind)
		}
		filter += " && " + field + "={:" + key + "}"
		params[key] = value
		return nil
	}
	if err = add("status", "status", o.Status); err != nil {
		return out, err
	}
	if err = add("goal", "goal", o.Goal); err != nil {
		return out, err
	}
	if o.Tag != "" {
		filter += " && tags.name ?= {:tag}"
		params["tag"] = o.Tag
	}
	dateField := "created"
	if kind == "journal" {
		dateField = "date"
	}
	if kind == "task" {
		dateField = "due_on"
	}
	for _, v := range []string{o.From, o.To} {
		if v != "" {
			if _, err = time.Parse("2006-01-02", v); err != nil {
				return out, fmt.Errorf("invalid date filter")
			}
		}
	}
	if o.From != "" && o.To != "" && o.From > o.To {
		return out, fmt.Errorf("invalid date range")
	}
	if o.From != "" {
		filter += " && " + dateField + ">={:from}"
		params["from"] = o.From
	}
	if o.To != "" {
		if dateField == "created" {
			filter += " && created<{:to}"
			d, _ := time.Parse("2006-01-02", o.To)
			params["to"] = d.AddDate(0, 0, 1).Format("2006-01-02")
		} else {
			filter += " && " + dateField + "<={:to}"
			params["to"] = o.To
		}
	}
	rows, err := s.App.FindRecordsByFilter(c, filter, "-created,-id", limit+1, (page-1)*limit, params)
	if err != nil {
		return out, err
	}
	out.HasMore = len(rows) > limit
	if out.HasMore {
		rows = rows[:limit]
	}
	for _, r := range rows {
		item := DTO(s.App, r)
		if o.Compact {
			delete(item, "content")
			delete(item, "content_text")
			delete(item, "description")
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func PlainDocument(text string) Document {
	nodes := []Node{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		nodes = append(nodes, Node{"type": "p", "children": []any{Node{"text": line}}})
	}
	return Document{Version: 1, Value: nodes, AudioFileIDs: []string{}}
}
func accountDate(app core.App, user string) string {
	zone := "UTC"
	rows, err := app.FindRecordsByFilter("settings", "user={:u}", "", 1, 0, dbx.Params{"u": user})
	if err == nil && len(rows) > 0 && rows[0].GetString("timezone") != "" {
		zone = rows[0].GetString("timezone")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func (s *Server) Create(ctx context.Context, p Principal, kind string, input map[string]any, document *Document) (map[string]any, error) {
	if err := authorize(ctx, p, kind, true); err != nil {
		return nil, err
	}
	table := collections[kind]
	var created *core.Record
	err := s.transaction(ctx, func(app core.App) error {

		c, err := app.FindCollectionByNameOrId(table)
		if err != nil {
			return err
		}
		r := core.NewRecord(c)
		r.Set("user", p.UserID)
		r.Set("title", "Untitled")
		r.Set("status", map[string]string{"tasks": "todo", "goals": "active"}[table])
		r.Set("cadence", "daily")
		r.Set("date", accountDate(app, p.UserID))
		r.Set("content", Document{Version: 1, Value: []Node{{"type": "p", "children": []any{Node{"text": ""}}}}, AudioFileIDs: []string{}})
		if err = s.applyPrincipalPatch(app, r, input, p); err != nil {
			return err
		}
		if err = app.Save(r); err != nil {
			return err
		}
		created = r
		if document != nil {
			return SaveDocument(app, r, *document, 0)
		}
		return indexEntity(app, r)
	})
	if err != nil {
		return nil, err
	}
	return DTO(s.App, created), nil
}

func (s *Server) Update(ctx context.Context, p Principal, kind, id string, revision int, patch map[string]any) (map[string]any, error) {
	if err := authorize(ctx, p, kind, true); err != nil {
		return nil, err
	}
	var r *core.Record
	err := s.transaction(ctx, func(app core.App) error {

		fresh, err := entity(app, p, kind, id)
		if err != nil {
			return err
		}
		if fresh.GetString("deleted_at") != "" {
			return fmt.Errorf("record is deleted")
		}
		if fresh.GetInt("revision") != revision {
			return ErrConflict
		}
		if err = s.applyPrincipalPatch(app, fresh, patch, p); err != nil {
			return err
		}
		fresh.Set("revision", fresh.GetInt("revision")+1)
		if err = app.Save(fresh); err != nil {
			return err
		}
		r = fresh
		if err := indexEntity(app, fresh); err != nil {
			return err
		}
		if _, changed := patch["tags"]; changed {
			return pruneUnusedTags(app, p.UserID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return DTO(s.App, r), nil
}
func (s *Server) applyPrincipalPatch(app core.App, r *core.Record, patch map[string]any, p Principal) error {
	if p.Source == Agent {
		allowed := map[string]bool{"title": true, "status": true, "progress": true, "cadence": true, "priority": true, "effort": true, "due_on": true, "goal": true, "position": true, "wait_until": true, "hour": true, "minute": true, "day": true, "tags": true}
		for key := range patch {
			if !allowed[key] || r.Collection().Fields.GetByName(key) == nil {
				return fmt.Errorf("unsupported field %q", key)
			}
		}
	}
	return s.applyPatch(app, r, patch, p.UserID)
}
func (s *Server) WriteDocument(ctx context.Context, p Principal, kind, id string, revision int, d Document, appendText *string) (map[string]any, error) {
	if err := authorize(ctx, p, kind, true); err != nil {
		return nil, err
	}
	var r *core.Record
	err := s.transaction(ctx, func(app core.App) error {
		var err error
		r, err = entity(app, p, kind, id)
		if err != nil {
			return err
		}
		if r.GetString("deleted_at") != "" {
			return fmt.Errorf("record is deleted")
		}
		if appendText != nil {
			d = ReadDocument(r)
			d.Value = append(d.Value, PlainDocument(*appendText).Value...)
		}
		return SaveDocument(app, r, d, revision)
	})
	if err != nil {
		return nil, err
	}
	return DTO(s.App, r), nil
}

func (s *Server) SetDeleted(ctx context.Context, p Principal, kind, id string, restore bool, revision *int) (map[string]any, error) {
	if err := authorize(ctx, p, kind, true); err != nil {
		return nil, err
	}
	if p.Source == Agent && revision == nil {
		return nil, fmt.Errorf("baseRevision required")
	}
	var r *core.Record
	err := s.transaction(ctx, func(app core.App) error {

		var err error
		r, err = entity(app, p, kind, id)
		if err != nil {
			return err
		}
		if revision != nil && r.GetInt("revision") != *revision {
			return ErrConflict
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
		return nil, err
	}
	return DTO(s.App, r), nil
}

func (s *Server) Duplicate(ctx context.Context, p Principal, kind, id string, revision *int) (map[string]any, error) {
	if err := authorize(ctx, p, kind, true); err != nil {
		return nil, err
	}
	if p.Source == Agent && revision == nil {
		return nil, fmt.Errorf("baseRevision required")
	}
	var copy *core.Record
	err := s.transaction(ctx, func(app core.App) error {

		r, err := entity(app, p, kind, id)
		if err != nil {
			return err
		}
		if revision != nil && r.GetInt("revision") != *revision {
			return ErrConflict
		}

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
		return nil, err
	}
	return DTO(s.App, copy), nil
}

func (s *Server) Relationships(ctx context.Context, p Principal, kind, id string) ([]map[string]any, error) {
	if err := authorize(ctx, p, kind, false); err != nil {
		return nil, err
	}
	r, err := entity(s.App, p, kind, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.App.FindRecordsByFilter("document_references", "user={:u} && (source_"+kind+"={:id} || target_"+kind+"={:id})", "", 0, 0, dbx.Params{"u": p.UserID, "id": r.Id})
	if err != nil {
		return nil, err
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
		candidates, err := s.App.FindRecordsByFilter(table, "user={:u} && deleted_at=''", "", 0, 0, dbx.Params{"u": p.UserID})
		if err != nil {
			return nil, err
		}
		for _, source := range candidates {
			for _, field := range source.Collection().Fields {
				relation, ok := field.(*core.RelationField)
				if !ok {
					continue
				}
				for targetKind, targetTable := range collections {
					collection, _ := s.App.FindCollectionByNameOrId(targetTable)
					if relation.CollectionId != collection.Id {
						continue
					}
					for _, targetID := range source.GetStringSlice(relation.Name) {
						if (sourceKind == kind && source.Id == r.Id) || (targetKind == kind && targetID == r.Id) {
							if target, err := owned(s.App, targetTable, targetID, p.UserID); err == nil && target.GetString("deleted_at") == "" {
								add(sourceKind, source.Id, targetKind, targetID, true)
							}
						}
					}
				}
			}
		}
	}
	return edges, nil
}

type TaskOrder struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Position int    `json:"position"`
	Revision int    `json:"revision"`
}

func (s *Server) Reorder(ctx context.Context, p Principal, tasks []TaskOrder) ([]any, error) {
	if err := authorize(ctx, p, "task", true); err != nil {
		return nil, err
	}
	if len(tasks) > 1000 {
		return nil, fmt.Errorf("too many tasks")
	}
	items := []any{}
	err := s.transaction(ctx, func(app core.App) error {

		seen := map[string]bool{}
		for _, item := range tasks {
			if seen[item.ID] {
				return fmt.Errorf("duplicate task")
			}
			seen[item.ID] = true
			r, err := entity(app, p, "task", item.ID)
			if err != nil {
				return err
			}
			if r.GetString("deleted_at") != "" {
				return fmt.Errorf("record is deleted")
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
	return items, err
}

type HabitDayInput struct {
	Completed *bool   `json:"completed,omitempty"`
	Notes     *string `json:"notes,omitempty"`
}

func (s *Server) SetHabitDay(ctx context.Context, p Principal, id, date string, input HabitDayInput) (*core.Record, error) {
	if err := authorize(ctx, p, "habit", true); err != nil {
		return nil, err
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, fmt.Errorf("invalid date")
	}
	var row *core.Record
	err := s.transaction(ctx, func(app core.App) error {

		habit, err := entity(app, p, "habit", id)
		if err != nil {
			return err
		}
		if habit.GetString("deleted_at") != "" {
			return fmt.Errorf("habit is deleted")
		}

		rows, err := app.FindRecordsByFilter("habit_completions", "user={:u} && habit={:h} && date={:d}", "", 1, 0, dbx.Params{"u": p.UserID, "h": habit.Id, "d": date})
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			row = rows[0]
		} else {
			c, _ := app.FindCollectionByNameOrId("habit_completions")
			row = core.NewRecord(c)
			row.Set("user", p.UserID)
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
	return row, err
}
func (s *Server) HabitHistory(ctx context.Context, p Principal, id, from, to string, page, limit int) (PageResult, error) {
	out := PageResult{Items: []map[string]any{}}
	if err := authorize(ctx, p, "habit", false); err != nil {
		return out, err
	}
	if _, err := entity(s.App, p, "habit", id); err != nil {
		return out, err
	}
	page, limit, err := pagination(page, limit)
	if err != nil {
		return out, err
	}
	out.Page = page
	filter := "user={:u} && habit={:h}"
	params := dbx.Params{"u": p.UserID, "h": id}
	for key, v := range map[string]string{"from": from, "to": to} {
		if v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return out, fmt.Errorf("invalid date")
			}
			op := ">="
			if key == "to" {
				op = "<="
			}
			filter += " && date" + op + "{:" + key + "}"
			params[key] = v
		}
	}
	if from != "" && to != "" && from > to {
		return out, fmt.Errorf("invalid date range")
	}
	rows, err := s.App.FindRecordsByFilter("habit_completions", filter, "-date,-id", limit+1, (page-1)*limit, params)
	if err != nil {
		return out, err
	}
	out.HasMore = len(rows) > limit
	if out.HasMore {
		rows = rows[:limit]
	}
	for _, row := range rows {
		out.Items = append(out.Items, row.PublicExport())
	}
	return out, nil
}

// ReplaceText changes body paragraphs while retaining all attachment-bearing blocks.
func (s *Server) ReplaceText(ctx context.Context, p Principal, kind, id string, revision int, text string) (map[string]any, error) {
	if err := authorize(ctx, p, kind, true); err != nil {
		return nil, err
	}
	var r *core.Record
	err := s.transaction(ctx, func(app core.App) error {
		var err error
		r, err = entity(app, p, kind, id)
		if err != nil {
			return err
		}
		if r.GetString("deleted_at") != "" {
			return fmt.Errorf("record is deleted")
		}
		d := ReadDocument(r)
		body := PlainDocument(text).Value
		var hasFile func(Node) bool
		hasFile = func(n Node) bool {
			if _, ok := n["fileId"]; ok {
				return true
			}
			for _, child := range nodeChildren(n) {
				if hasFile(child) {
					return true
				}
			}
			return false
		}
		for _, n := range d.Value {
			if hasFile(n) {
				body = append(body, n)
			}
		}
		d.Value = body
		return SaveDocument(app, r, d, revision)
	})
	if err != nil {
		return nil, err
	}
	return DTO(s.App, r), nil
}
func nodeChildren(n Node) []Node {
	out := []Node{}
	switch children := n["children"].(type) {
	case []any:
		for _, child := range children {
			switch c := child.(type) {
			case Node:
				out = append(out, c)
			case map[string]any:
				out = append(out, Node(c))
			}
		}
	case []Node:
		out = children
	}
	return out
}
func documentText(d Document) string {
	var b strings.Builder
	var walk func(Node)
	walk = func(n Node) {
		if t, ok := n["text"].(string); ok {
			b.WriteString(t)
		}
		for _, c := range nodeChildren(n) {
			walk(c)
		}
	}
	for _, n := range d.Value {
		walk(n)
		b.WriteByte('\n')
	}
	return b.String()
}

// Cancellation before commit rolls the entire operation back.
func (s *Server) transaction(ctx context.Context, fn func(core.App) error) error {
	return s.App.RunInTransaction(func(app core.App) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(app); err != nil {
			return err
		}
		return ctx.Err()
	})
}
