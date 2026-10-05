package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("tokens")
		if err != nil {
			return err
		}
		c.Fields.Add(&core.TextField{Name: "purpose"})
		c.Fields.GetByName("hash").SetHidden(true)
		c.CreateRule = nil
		c.UpdateRule = nil
		c.DeleteRule = nil
		c.ListRule = nil
		c.ViewRule = nil
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("tokens")
		if err != nil {
			return err
		}
		// Never restore direct credential writes on rollback.
		c.Fields.RemoveByName("purpose")
		return app.Save(c)
	})
}
