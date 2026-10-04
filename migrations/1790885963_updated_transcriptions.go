package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_3590480204")
		if err != nil {
			return err
		}

		// remove field
		collection.Fields.RemoveById("file2359244304")

		// add field
		if err := collection.Fields.AddMarshaledJSONAt(1, []byte(`{
			"cascadeDelete": false,
			"collectionId": "pbc_3446931122",
			"help": "",
			"hidden": false,
			"id": "relation2359244304",
			"maxSelect": 0,
			"minSelect": 0,
			"name": "file",
			"presentable": false,
			"required": true,
			"system": false,
			"type": "relation"
		}`)); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_3590480204")
		if err != nil {
			return err
		}

		// add field
		if err := collection.Fields.AddMarshaledJSONAt(1, []byte(`{
			"help": "",
			"hidden": false,
			"id": "file2359244304",
			"maxSelect": 0,
			"maxSize": 26214400,
			"mimeTypes": [
				"audio/ogg",
				"audio/mpeg",
				"audio/wav",
				"audio/x-m4a",
				"audio/aac"
			],
			"name": "file",
			"presentable": false,
			"protected": true,
			"required": true,
			"system": false,
			"thumbs": [],
			"type": "file"
		}`)); err != nil {
			return err
		}

		// remove field
		collection.Fields.RemoveById("relation2359244304")

		return app.Save(collection)
	})
}
