package main

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/osutils"
	"github.com/spf13/cobra"

	"github.com/chand1012/lyphe/internal/version"
	_ "github.com/chand1012/lyphe/migrations"
)

// Static export of the frontend (frontend: `bun run build` -> build/client).
const defaultStaticPath = "frontend/build/client"

func main() {
	app := pocketbase.New()

	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version.Info())
		},
	})

	var staticPath string
	app.RootCmd.PersistentFlags().StringVar(
		&staticPath,
		"staticPath",
		defaultStaticPath,
		"Directory to serve from the base URL (empty to disable)",
	)

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		// enable auto creation of migration files when making collection changes in the Dashboard
		// (the IsProbablyGoRun check is to enable it only during development)
		Automigrate: osutils.IsProbablyGoRun(),
	})

	// Serve the built SPA from the base URL. The mux prefers the more specific
	// /api and /_ routes, so the catch-all wildcard doesn't shadow them.
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			if _, err := os.Stat(staticPath); err == nil && !e.Router.HasRoute("GET", "/{path...}") {
				// indexFallback = true -> index.html for unknown paths (SPA routes)
				e.Router.GET("/{path...}", apis.Static(os.DirFS(staticPath), true))
			}

			return e.Next()
		},
		Priority: 999, // last, so app routes registered earlier win
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
