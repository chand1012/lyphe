package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_1358207143")
		if err != nil {
			return err
		}

		// add field
		if err := collection.Fields.AddMarshaledJSONAt(7, []byte(`{
			"cascadeDelete": false,
			"collectionId": "pbc_1219621782",
			"help": "",
			"hidden": false,
			"id": "relation1874629670",
			"maxSelect": 10,
			"minSelect": 0,
			"name": "tags",
			"presentable": false,
			"required": false,
			"system": false,
			"type": "relation"
		}`)); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_1358207143")
		if err != nil {
			return err
		}

		// remove field
		collection.Fields.RemoveById("relation1874629670")

		return app.Save(collection)
	})
}
