package backend

import (
	"fmt"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"strings"
	"time"
)

func indexEntity(app core.App, r *core.Record) error {
	kind, _ := entityKind(r.Collection().Name)
	if _, err := app.DB().NewQuery("DELETE FROM content_search WHERE kind={:k} AND entity_id={:id}").Bind(dbx.Params{"k": kind, "id": r.Id}).Execute(); err != nil {
		return err
	}
	if r.GetString("deleted_at") != "" {
		return nil
	}
	title := r.GetString("title")
	if title == "" {
		title = r.GetString("goal")
	}
	body := r.GetString("content_text")
	if body == "" {
		d := ReadDocument(r)
		raw, _, _, err := ValidateDocument(app, d, r.GetString("user"))
		if err == nil {
			body = raw
		}
	}
	for _, id := range r.GetStringSlice("tags") {
		if t, err := app.FindRecordById("tags", id); err == nil {
			body += "\n" + t.GetString("name")
		}
	}
	if id := r.GetString("folder"); id != "" {
		if f, err := app.FindRecordById("folders", id); err == nil {
			body += "\n" + f.GetString("name")
		}
	}
	params := dbx.Params{"u": r.GetString("user"), "k": kind, "id": r.Id, "p": "", "t": title, "b": body}
	if _, err := app.DB().NewQuery("INSERT INTO content_search (user,kind,entity_id,placement_id,title,body) VALUES ({:u},{:k},{:id},{:p},{:t},{:b})").Bind(params).Execute(); err != nil {
		return err
	}
	placements, err := app.FindRecordsByFilter("entity_files", "source_"+kind+"={:id}", "", 0, 0, dbx.Params{"id": r.Id})
	if err != nil {
		return err
	}
	for _, placement := range placements {
		f, err := app.FindRecordById("files", placement.GetString("file"))
		if err != nil {
			continue
		}
		params["p"] = placement.GetString("placement_id")
		params["t"] = f.GetString("name")
		params["b"] = f.GetString("name")
		jobs, err := app.FindRecordsByFilter("transcriptions", "file={:f} && user={:u}", "", 0, 0, dbx.Params{"f": f.Id, "u": r.GetString("user")})
		if err != nil {
			return err
		}
		for _, job := range jobs {
			params["b"] = fmt.Sprint(params["b"]) + "\n" + job.GetString("raw_text") + "\n" + job.GetString("cleaned_text")
		}
		if _, err = app.DB().NewQuery("INSERT INTO content_search (user,kind,entity_id,placement_id,title,body) VALUES ({:u},{:k},{:id},{:p},{:t},{:b})").Bind(params).Execute(); err != nil {
			return err
		}
	}
	return nil
}
func (s *Server) rebuildSearch() error {
	return s.App.RunInTransaction(func(app core.App) error {
		if _, err := app.DB().NewQuery("DELETE FROM content_search").Execute(); err != nil {
			return err
		}
		for _, table := range collections {
			rows, err := app.FindAllRecords(table)
			if err != nil {
				return err
			}
			for _, r := range rows {
				if err = indexEntity(app, r); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
func (s *Server) search(e *core.RequestEvent) error {
	q := strings.TrimSpace(e.Request.URL.Query().Get("q"))
	if len(q) > 300 {
		return e.BadRequestError("Search is too long", nil)
	}
	type Hit struct {
		Kind        string `json:"kind" db:"kind"`
		ID          string `json:"id" db:"entity_id"`
		PlacementID string `json:"placementId,omitempty" db:"placement_id"`
		Title       string `json:"title" db:"title"`
		Snippet     string `json:"snippet" db:"snippet"`
	}
	hits := []Hit{}
	params := dbx.Params{"u": e.Auth.Id}
	query := "SELECT kind,entity_id,placement_id,title,substr(body,1,180) AS snippet FROM content_search WHERE user={:u} LIMIT 50"
	if q != "" {
		tokens := strings.Fields(q)
		quoted := []string{}
		for _, t := range tokens {
			quoted = append(quoted, "\""+strings.ReplaceAll(t, "\"", "\"\"")+"\"*")
		}
		params["q"] = strings.Join(quoted, " AND ")
		query = "SELECT kind,entity_id,placement_id,title,snippet(content_search,5,'','', '…',24) AS snippet FROM content_search WHERE content_search MATCH {:q} AND user={:u} ORDER BY rank LIMIT 50"
	}
	if err := e.App.DB().NewQuery(query).Bind(params).All(&hits); err != nil {
		return response(e, err)
	}
	return e.JSON(200, hits)
}
func (s *Server) cleanup() error {
	return s.App.RunInTransaction(func(app core.App) error {
		cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour).Format("2006-01-02 15:04:05.000Z")
		for kind, table := range collections {
			rows, err := app.FindRecordsByFilter(table, "deleted_at!='' && deleted_at < {:d}", "", 0, 0, dbx.Params{"d": cutoff})
			if err != nil {
				return err
			}
			for _, r := range rows {
				// Keep tombstones referenced by live documents so later edits remain valid.
				incoming, err := app.FindRecordsByFilter("document_references", "target_"+kind+"={:id}", "", 1, 0, dbx.Params{"id": r.Id})
				if err != nil {
					return err
				}
				if len(incoming) > 0 {
					continue
				}
				if _, err := app.DB().NewQuery("DELETE FROM content_search WHERE kind={:k} AND entity_id={:id}").Bind(dbx.Params{"k": kind, "id": r.Id}).Execute(); err != nil {
					return err
				}
				for _, derived := range []string{"entity_files", "document_references"} {
					linked, err := app.FindRecordsByFilter(derived, "source_"+kind+"={:id}", "", 0, 0, dbx.Params{"id": r.Id})
					if err != nil {
						return err
					}
					for _, link := range linked {
						if err = app.Delete(link); err != nil {
							return err
						}
					}
				}
				if err = app.Delete(r); err != nil {
					return err
				}
			}
		}
		files, err := app.FindRecordsByFilter("files", "created < {:d}", "", 0, 0, dbx.Params{"d": time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02 15:04:05.000Z")})
		if err != nil {
			return err
		}
		for _, f := range files {
			links, err := app.FindRecordsByFilter("entity_files", "file={:id}", "", 1, 0, dbx.Params{"id": f.Id})
			if err != nil {
				return err
			}
			if len(links) > 0 {
				continue
			}
			jobs, err := app.FindRecordsByFilter("transcriptions", "file={:id}", "", 1, 0, dbx.Params{"id": f.Id})
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				if err = app.Delete(f); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// IndexEntity refreshes derived search rows in the caller's transaction.
func IndexEntity(app core.App, r *core.Record) error { return indexEntity(app, r) }
