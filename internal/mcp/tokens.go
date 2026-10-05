package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const tokenPrefix = "lyphe_mcp_"

type Credential struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Expiration string `json:"expiration"`
	Created    string `json:"created"`
}

func credential(r *core.Record) Credential {
	return Credential{ID: r.Id, Name: r.GetString("name"), Expiration: r.GetDateTime("expiration").Time().UTC().Format(time.RFC3339), Created: r.GetDateTime("created").Time().UTC().Format(time.RFC3339)}
}
func tokenHash(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
func IssueToken(app core.App, user, name string, expiry time.Time) (Credential, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return Credential{}, "", fmt.Errorf("name must be between 1 and 200 bytes")
	}
	if !expiry.After(time.Now()) {
		return Credential{}, "", fmt.Errorf("expiration must be in the future")
	}
	c, err := app.FindCollectionByNameOrId("tokens")
	if err != nil {
		return Credential{}, "", err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return Credential{}, "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(secret)
	r := core.NewRecord(c)
	r.Set("user", user)
	r.Set("name", name)
	r.Set("purpose", "mcp")
	r.Set("hash", tokenHash(encoded))
	r.Set("expiration", expiry.UTC().Format(time.RFC3339))
	if err = app.Save(r); err != nil {
		return Credential{}, "", err
	}
	return credential(r), tokenPrefix + r.Id + "." + encoded, nil
}
func authenticate(app core.App, header string) (identity, error) {
	denied := fmt.Errorf("invalid MCP credential")
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !strings.HasPrefix(parts[1], tokenPrefix) {
		return identity{}, denied
	}
	id, secret, ok := strings.Cut(strings.TrimPrefix(parts[1], tokenPrefix), ".")
	if !ok || len(id) != 15 || len(secret) != 43 {
		return identity{}, denied
	}
	r, err := app.FindRecordById("tokens", id)
	if err != nil || r.GetString("purpose") != "mcp" || r.GetDateTime("expiration").Time().IsZero() || !r.GetDateTime("expiration").Time().After(time.Now()) {
		return identity{}, denied
	}
	if subtle.ConstantTimeCompare([]byte(tokenHash(secret)), []byte(r.GetString("hash"))) != 1 {
		return identity{}, denied
	}
	user, err := app.FindRecordById("users", r.GetString("user"))
	if err != nil {
		return identity{}, denied
	}
	return identity{UserID: user.Id, CredentialID: r.Id}, nil
}
func (s *Server) listTokens(e *core.RequestEvent) error {
	e.Response.Header().Set("Cache-Control", "no-store")
	rows, err := e.App.FindRecordsByFilter("tokens", "user={:u} && purpose='mcp'", "-created", 0, 0, dbx.Params{"u": e.Auth.Id})
	if err != nil {
		return e.InternalServerError("Credentials unavailable", nil)
	}
	items := []Credential{}
	for _, r := range rows {
		items = append(items, credential(r))
	}
	return e.JSON(200, map[string]any{"items": items})
}
func (s *Server) createToken(e *core.RequestEvent) error {
	var input struct {
		Name       string `json:"name"`
		Expiration string `json:"expiration"`
	}
	if err := e.BindBody(&input); err != nil {
		return e.BadRequestError("Invalid credential request", nil)
	}
	expiry := time.Now().AddDate(0, 0, 90)
	if input.Expiration != "" {
		var err error
		expiry, err = time.Parse(time.RFC3339, input.Expiration)
		if err != nil {
			return e.BadRequestError("Use an RFC3339 expiration", nil)
		}
	}
	item, secret, err := IssueToken(e.App, e.Auth.Id, input.Name, expiry)
	if err != nil {
		return e.BadRequestError("Could not create credential", err)
	}
	e.Response.Header().Set("Cache-Control", "no-store")
	return e.JSON(201, map[string]any{"credential": item, "token": secret})
}
func (s *Server) deleteToken(e *core.RequestEvent) error {
	r, err := e.App.FindRecordById("tokens", e.Request.PathValue("id"))
	if err != nil || r.GetString("user") != e.Auth.Id || r.GetString("purpose") != "mcp" {
		return e.NotFoundError("Credential unavailable", nil)
	}
	if err = e.App.Delete(r); err != nil {
		return e.InternalServerError("Could not revoke credential", nil)
	}
	return e.NoContent(204)
}
