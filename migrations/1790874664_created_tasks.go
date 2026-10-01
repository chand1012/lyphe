package migrations

import (
	"encoding/json/v2"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
			"createRule": null,
			"deleteRule": null,
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
					"autogeneratePattern": "",
					"help": "title of the task",
					"hidden": false,
					"id": "text724990059",
					"max": 0,
					"min": 0,
					"name": "title",
					"pattern": "",
					"presentable": true,
					"primaryKey": false,
					"required": true,
					"system": false,
					"type": "text"
				},
				{
					"convertURLs": false,
					"help": "description of the task",
					"hidden": false,
					"id": "editor1843675174",
					"maxSize": 0,
					"name": "description",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "editor"
				},
				{
					"help": "",
					"hidden": false,
					"id": "select1655102503",
					"maxSelect": 0,
					"name": "priority",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "select",
					"values": [
						"low",
						"medium",
						"high",
						"urgent",
						"critical"
					]
				},
				{
					"help": "",
					"hidden": false,
					"id": "select2063623452",
					"maxSelect": 0,
					"name": "status",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "select",
					"values": [
						"todo",
						"in_progress",
						"done",
						"in_review",
						"blocked",
						"paused"
					]
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
					"cascadeDelete": false,
					"collectionId": "pbc_207790276",
					"help": "",
					"hidden": false,
					"id": "relation4242336558",
					"maxSelect": 0,
					"minSelect": 0,
					"name": "goal",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "relation"
				},
				{
					"help": "When to wait until",
					"hidden": false,
					"id": "date3746841700",
					"max": "",
					"min": "",
					"name": "wait_until",
					"presentable": false,
					"required": false,
					"system": false,
					"type": "date"
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
			"id": "pbc_2602490748",
			"indexes": [],
			"listRule": null,
			"name": "tasks",
			"system": false,
			"type": "base",
			"updateRule": null,
			"viewRule": null
		}`

		collection := &core.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("pbc_2602490748")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
