package backend

import (
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Include soft-deleted items: their tags must survive the undo/restore window.
// Once an item is permanently deleted, its final unused tags can be removed.
func pruneUnusedTags(app core.App, user string) error {
	conditions := []string{}
	params := dbx.Params{}
	if user != "" {
		conditions = append(conditions, "t.user={:user}")
		params["user"] = user
	}
	for _, table := range collections {
		conditions = append(conditions, "NOT EXISTS (SELECT 1 FROM "+table+" AS entity, json_each(entity.tags) AS tag WHERE tag.value=t.id)")
	}
	var unused []struct {
		ID string `db:"id"`
	}
	if err := app.DB().NewQuery("SELECT t.id FROM tags AS t WHERE " + strings.Join(conditions, " AND ")).Bind(params).All(&unused); err != nil {
		return err
	}
	for _, row := range unused {
		tag, err := app.FindRecordById("tags", row.ID)
		if err != nil {
			return err
		}
		if err := app.Delete(tag); err != nil {
			return err
		}
	}
	return nil
}
