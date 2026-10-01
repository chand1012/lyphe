package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_2942065320")
		if err != nil {
			return err
		}

		// add field
		if err := collection.Fields.AddMarshaledJSONAt(5, []byte(`{
			"help": "",
			"hidden": false,
			"id": "select1274211008",
			"maxSelect": 0,
			"name": "select",
			"presentable": false,
			"required": true,
			"system": false,
			"type": "select",
			"values": [
				"decision",
				"llm",
				"agent",
				"tools",
				"task",
				"journal",
				"habit",
				"other"
			]
		}`)); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_2942065320")
		if err != nil {
			return err
		}

		// remove field
		collection.Fields.RemoveById("select1274211008")

		return app.Save(collection)
	})
}
