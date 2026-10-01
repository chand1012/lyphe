package migrations

import (
	"encoding/json/v2"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
			"createRule": "@request.auth.id = user.id",
			"deleteRule": "@request.auth.id = user.id",
			"fields": [
				{
					"autogeneratePattern": "[a-z0-9]{15}",
					"help": "",
					"hidden": false,
					"id": "text3208210256",
					"max": 15,
					"min": 15,
					"name": "id",
					"pattern": "^[a-z0-9]+$",
					"presentable": false,
					"primaryKey": true,
					"required": true,
					"system": true,
					"type": "text"
				},
				{
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
				},
				{
					"cascadeDelete": false,
					"collectionId": "_pb_users_auth_",
					"help": "",
					"hidden": false,
					"id": "relation2375276105",
					"maxSelect": 0,
					"minSelect": 0,
					"name": "user",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "relation"
				},
				{
					"convertURLs": false,
					"help": "",
					"hidden": false,
					"id": "editor849144196",
					"maxSize": 0,
					"name": "transcription",
					"presentable": false,
					"required": true,
					"system": false,
					"type": "editor"
				},
				{
					"cascadeDelete": false,
					"collectionId": "pbc_1358207143",
					"help": "",
					"hidden": false,
					"id": "relation3249006413",
					"maxSelect": 0,
					"minSelect": 0,
					"name": "journal",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "relation"
				},
				{
					"cascadeDelete": false,
					"collectionId": "pbc_2602490748",
					"help": "",
					"hidden": false,
					"id": "relation1384045349",
					"maxSelect": 0,
					"minSelect": 0,
					"name": "task",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "relation"
				},
				{
					"hidden": false,
					"id": "autodate2990389176",
					"name": "created",
					"onCreate": true,
					"onUpdate": false,
					"presentable": false,
					"system": false,
					"type": "autodate"
				},
				{
					"hidden": false,
					"id": "autodate3332085495",
					"name": "updated",
					"onCreate": true,
					"onUpdate": true,
					"presentable": false,
					"system": false,
					"type": "autodate"
				}
			],
			"id": "pbc_3590480204",
			"indexes": [],
			"listRule": "@request.auth.id = user.id",
			"name": "transcriptions",
			"system": false,
			"type": "base",
			"updateRule": "@request.auth.id = user.id",
			"viewRule": "@request.auth.id = user.id"
		}`

		collection := &core.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_3590480204")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
