// Package mcp exposes a restricted agent interface to existing Lyphe data.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chand1012/lyphe/internal/backend"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type Server struct {
	App     core.App
	Backend *backend.Server
	Handler http.Handler
}
type identity struct{ UserID, CredentialID string }
type identityKey struct{}

func Register(app core.App, b *backend.Server) *Server {
	s := New(app, b)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/mcp", apis.WrapStdHandler(s.Handler))
		e.Router.GET("/mcp", apis.WrapStdHandler(s.Handler))
		e.Router.DELETE("/mcp", apis.WrapStdHandler(s.Handler))
		r := e.Router.Group("/api/mcp/tokens")
		r.Bind(apis.RequireAuth("users"))
		r.GET("", s.listTokens)
		r.POST("", s.createToken)
		r.DELETE("/{id}", s.deleteToken)
		return e.Next()
	})
	return s
}
func New(app core.App, b *backend.Server) *Server {
	s := &Server{App: app, Backend: b}
	server := sdk.NewServer(&sdk.Implementation{Name: "lyphe", Version: "1.0.0"}, nil)
	s.tools(server)
	transport := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 2 << 20, PropagateRequestCancellation: true})
	s.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !validOrigin(r) {
			http.Error(w, "Origin denied", http.StatusForbidden)
			return
		}
		who, err := authenticate(app, r.Header.Get("Authorization"))
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer realm="lyphe-mcp"`)
			http.Error(w, "MCP credential required", http.StatusUnauthorized)
			return
		}
		transport.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, who)))
	})
	return s
}
func validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && strings.EqualFold(u.Host, r.Host)
}
func principal(ctx context.Context) (backend.Principal, identity) {
	who, _ := ctx.Value(identityKey{}).(identity)
	return backend.Principal{UserID: who.UserID, Source: backend.Agent}, who
}
func errorCode(err error) string {
	switch {
	case errors.Is(err, backend.ErrPermission):
		return "permission_denied"
	case errors.Is(err, backend.ErrUnavailable):
		return "not_found"
	case errors.Is(err, backend.ErrConflict):
		return "revision_conflict"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	default:
		return "validation_error"
	}
}
func result(value any, err error) *sdk.CallToolResult {
	failed := err != nil
	if failed {
		value = map[string]any{"code": errorCode(err), "message": err.Error()}
	}
	data, _ := json.Marshal(value)
	return &sdk.CallToolResult{IsError: failed, StructuredContent: value, Content: []sdk.Content{&sdk.TextContent{Text: string(data)}}}
}
func (s *Server) log(ctx context.Context, name, id string, started time.Time, err error) {
	_, who := principal(ctx)
	code := "ok"
	if err != nil {
		code = errorCode(err)
	}
	s.App.Logger().Info("MCP tool", "tool", name, "credential", who.CredentialID, "entity", id, "result", code, "duration", time.Since(started).String())
}
