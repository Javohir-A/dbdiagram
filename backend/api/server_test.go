package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/api"
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/app"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/testkit"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/storagetest"
	"gitlab.com/udevs/dbdiagram/backend/pkg/jwt"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type client struct {
	t     *testing.T
	h     http.Handler
	token string
	guid  string
	// headers are extra request headers (for example X-Forwarded-For).
	headers map[string]string
}

func newHandler(t *testing.T, mutate func(*config.Config)) http.Handler {
	t.Helper()

	var (
		cfg = testkit.Config()
		log = logger.New("error", "test")
	)
	if mutate != nil {
		mutate(cfg)
	}

	st := storagetest.New()
	c := core.New(core.Deps{Storage: st, Log: log, Cfg: cfg})
	a := &app.App{Cfg: cfg, Log: log, Core: c, Diagrams: diagrams.New(diagrams.Deps{Storage: st, Log: log, Cfg: cfg, Users: c.User})}

	return api.New(&api.Options{Cfg: cfg, Log: log, App: a}).Handler()
}

// newClient is a signed-in person (alice) on a fresh server.
func newClient(t *testing.T) *client {
	t.Helper()

	return newPerson(t, newHandler(t, nil), "alice")
}

// newPerson registers name@example.test through the real endpoint and returns a
// client carrying their access token.
func newPerson(t *testing.T, h http.Handler, name string) *client {
	t.Helper()

	anon := &client{t: t, h: h}
	status, body, _ := anon.do("POST", "/v1/auth/register", map[string]any{"email": name + "@example.test", "name": name, "password": "correct horse 9"})
	require.Equal(t, 200, status, "%v", body)

	return &client{
		t: t, h: h,
		token: body["access_token"].(string),
		guid:  body["user"].(map[string]any)["guid"].(string),
	}
}

// do sends a request; a string body is sent as-is, anything else as JSON.
func (c *client) do(method, path string, body any) (int, map[string]any, http.Header) {
	c.t.Helper()

	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		require.NoError(c.t, err)
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)

	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)

	return rec.Code, out, rec.Header()
}

func errorCode(t *testing.T, body map[string]any) string {
	t.Helper()

	e, ok := body["error"].(map[string]any)
	require.True(t, ok, "body must be the {error:{code,message}} envelope: %v", body)

	return e["code"].(string)
}

func TestCreateAndRead(t *testing.T) {
	c := newClient(t)

	status, d, _ := c.do("POST", "/v1/diagrams", map[string]any{"name": "Shop", "dbml": "Table a {id int}"})
	require.Equal(t, 200, status)
	for _, key := range []string{"guid", "name", "dbml", "positions", "folder_id", "share_token", "created_at", "updated_at"} {
		require.Contains(t, d, key, "every field is always present: no omitempty")
	}
	require.Equal(t, "", d["folder_id"])
	require.Equal(t, map[string]any{}, d["positions"])

	status, got, _ := c.do("GET", "/v1/diagrams/"+d["guid"].(string), nil)
	require.Equal(t, 200, status)
	require.Equal(t, "Shop", got["name"])
}

func TestValidationAndErrors(t *testing.T) {
	c := newClient(t)

	status, body, _ := c.do("POST", "/v1/diagrams", map[string]any{"dbml": "x"})
	require.Equal(t, 400, status)
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))

	status, body, _ = c.do("POST", "/v1/diagrams", "{not json")
	require.Equal(t, 400, status)
	require.Equal(t, "INVALID_JSON", errorCode(t, body))

	status, body, _ = c.do("POST", "/v1/diagrams", map[string]any{"name": "x", "positions": []int{1}})
	require.Equal(t, 400, status, "positions must be an object")
	require.Equal(t, "INVALID_JSON", errorCode(t, body))

	// Over the transport cap: the body is never fully read.
	status, body, _ = c.do("POST", "/v1/diagrams", `{"name":"x","dbml":"`+strings.Repeat("a", config.MaxBodySize)+`"}`)
	require.Equal(t, 413, status)
	require.Equal(t, "PAYLOAD_TOO_LARGE", errorCode(t, body))

	// Under the transport cap but over the DBML cap: the service rejects it.
	status, body, _ = c.do("POST", "/v1/diagrams", `{"name":"x","dbml":"`+strings.Repeat("a", config.MaxDBMLSize+10)+`"}`)
	require.Equal(t, 413, status)
	require.Equal(t, "PAYLOAD_TOO_LARGE", errorCode(t, body))

	status, body, _ = c.do("GET", "/v1/diagrams/missing", nil)
	require.Equal(t, 404, status)
	require.Equal(t, "NOT_FOUND", errorCode(t, body))

	status, body, _ = c.do("GET", "/v1/nope", nil)
	require.Equal(t, 404, status)
	require.Equal(t, "NOT_FOUND", errorCode(t, body), "an unknown route answers in the same envelope")

	for _, q := range []string{"page=0", "page=x", "limit=0", "filters=notjson", "sort=%7B%7D"} {
		status, body, _ = c.do("GET", "/v1/diagrams?"+q, nil)
		require.Equal(t, 400, status, q)
		require.Equal(t, "BAD_REQUEST", errorCode(t, body), q)
	}
}

func TestPutReplacesPatchKeeps(t *testing.T) {
	c := newClient(t)

	_, d, _ := c.do("POST", "/v1/diagrams", map[string]any{
		"name": "A", "dbml": "Table a {id int}", "positions": map[string]any{"a": map[string]any{"x": 1, "y": 2}},
	})
	path := "/v1/diagrams/" + d["guid"].(string)

	status, got, _ := c.do("PATCH", path, map[string]any{"name": "B"})
	require.Equal(t, 200, status)
	require.Equal(t, "B", got["name"])
	require.Equal(t, "Table a {id int}", got["dbml"], "PATCH keeps what it does not mention")
	require.NotEmpty(t, got["positions"])

	status, got, _ = c.do("PATCH", path, map[string]any{"dbml": ""})
	require.Equal(t, 200, status)
	require.Equal(t, "", got["dbml"], "an explicit empty string is a value, not an omission")

	status, got, _ = c.do("PUT", path, map[string]any{"name": "C"})
	require.Equal(t, 200, status)
	require.Equal(t, "C", got["name"])
	require.Equal(t, map[string]any{}, got["positions"], "PUT replaces: omitted positions become empty")
}

func TestShareFolderAndSharedView(t *testing.T) {
	c := newClient(t)

	_, d, _ := c.do("POST", "/v1/diagrams", map[string]any{"name": "A", "dbml": "Table a {id int}"})
	path := "/v1/diagrams/" + d["guid"].(string)

	status, body, _ := c.do("PUT", path+"/share", map[string]any{})
	require.Equal(t, 400, status, "enabled is required")
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))

	status, on, _ := c.do("PUT", path+"/share", map[string]any{"enabled": true})
	require.Equal(t, 200, status)
	token := on["share_token"].(string)
	require.NotEmpty(t, token)

	status, shared, _ := c.do("GET", "/v1/shared/"+token, nil)
	require.Equal(t, 200, status)
	require.Len(t, shared, 3, "a public link reveals name, dbml and positions only")
	require.Equal(t, "A", shared["name"])

	status, _, _ = c.do("PUT", path+"/share", map[string]any{"enabled": false})
	require.Equal(t, 200, status)
	status, _, _ = c.do("GET", "/v1/shared/"+token, nil)
	require.Equal(t, 404, status)

	status, body, _ = c.do("PUT", path+"/folder", map[string]any{"folder_id": "00000000-0000-0000-0000-000000000000"})
	require.Equal(t, 400, status)
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))

	_, f, _ := c.do("POST", "/v1/folders", map[string]any{"name": "Work"})
	status, moved, _ := c.do("PUT", path+"/folder", map[string]any{"folder_id": f["guid"]})
	require.Equal(t, 200, status)
	require.Equal(t, f["guid"], moved["folder_id"])

	status, body, _ = c.do("DELETE", "/v1/folders/"+f["guid"].(string), nil)
	require.Equal(t, 409, status, "a folder that still holds a diagram is not deleted")
	require.Equal(t, "CONFLICT", errorCode(t, body))

	status, _, _ = c.do("PUT", path+"/folder", map[string]any{"folder_id": ""})
	require.Equal(t, 200, status)
	status, _, _ = c.do("DELETE", "/v1/folders/"+f["guid"].(string), nil)
	require.Equal(t, 200, status)
	_, after, _ := c.do("GET", path, nil)
	require.Equal(t, "", after["folder_id"])

	status, _, _ = c.do("DELETE", path, nil)
	require.Equal(t, 200, status)
	status, _, _ = c.do("GET", path, nil)
	require.Equal(t, 404, status)
}

func TestCORSPreflightAllowsPatch(t *testing.T) {
	c := newClient(t)

	req := httptest.NewRequest("OPTIONS", "/v1/diagrams/x", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)

	require.Equal(t, 204, rec.Code)
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	require.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestVersionRoutes(t *testing.T) {
	c := newClient(t)

	_, d, _ := c.do("POST", "/v1/diagrams", map[string]any{"name": "A", "dbml": "Table a {id int}"})
	path := "/v1/diagrams/" + d["guid"].(string) + "/versions"

	status, v, _ := c.do("POST", path, nil)
	require.Equal(t, 200, status, "a POST with no body at all is a valid unlabelled snapshot")
	require.Equal(t, config.ManualVersionLabel, v["label"])
	require.Equal(t, d["guid"], v["diagram_id"])
	require.NotContains(t, v, "dbml", "list-style items never carry the large columns")

	status, labelled, _ := c.do("POST", path, map[string]any{"label": "release"})
	require.Equal(t, 200, status)
	require.Equal(t, "release", labelled["label"])

	status, body, _ := c.do("POST", path, map[string]any{"label": strings.Repeat("x", config.MaxLabelLength+1)})
	require.Equal(t, 400, status)
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))

	status, list, _ := c.do("GET", path, nil)
	require.Equal(t, 200, status)
	require.Equal(t, float64(2), list["count"])

	status, one, _ := c.do("GET", path+"/"+labelled["guid"].(string), nil)
	require.Equal(t, 200, status)
	require.Equal(t, "Table a {id int}", one["dbml"])
	require.Contains(t, one, "positions")

	// Change the diagram, then restore the labelled snapshot.
	c.do("PATCH", "/v1/diagrams/"+d["guid"].(string), map[string]any{"dbml": "changed"})
	status, restored, _ := c.do("POST", path+"/"+labelled["guid"].(string)+"/restore", nil)
	require.Equal(t, 200, status)
	require.Equal(t, "Table a {id int}", restored["dbml"])
	require.Equal(t, "A", restored["name"])

	status, _, _ = c.do("DELETE", path+"/"+labelled["guid"].(string), nil)
	require.Equal(t, 200, status)
	status, body, _ = c.do("GET", path+"/"+labelled["guid"].(string), nil)
	require.Equal(t, 404, status)
	require.Equal(t, "NOT_FOUND", errorCode(t, body))

	status, _, _ = c.do("GET", "/v1/diagrams/missing/versions", nil)
	require.Equal(t, 404, status)
}

// ---- auth, access and sharing over HTTP -------------------------------------

func TestPrivateRoutesRejectAnonymousCallers(t *testing.T) {
	h := newHandler(t, nil)
	anon := &client{t: t, h: h}
	key := "00000000-0000-0000-0000-000000000000"

	for _, r := range []struct{ method, path string }{
		{"GET", "/v1/me"}, {"PATCH", "/v1/me"}, {"PUT", "/v1/me/password"},
		{"GET", "/v1/diagrams"}, {"POST", "/v1/diagrams"}, {"GET", "/v1/diagrams/" + key}, {"PUT", "/v1/diagrams/" + key},
		{"PATCH", "/v1/diagrams/" + key}, {"DELETE", "/v1/diagrams/" + key}, {"GET", "/v1/diagrams/" + key + "/revision"},
		{"PUT", "/v1/diagrams/" + key + "/folder"}, {"PUT", "/v1/diagrams/" + key + "/share"},
		{"GET", "/v1/diagrams/" + key + "/members"}, {"PUT", "/v1/diagrams/" + key + "/members"}, {"DELETE", "/v1/diagrams/" + key + "/members/" + key},
		{"GET", "/v1/diagrams/" + key + "/versions"}, {"POST", "/v1/diagrams/" + key + "/versions"},
		{"GET", "/v1/diagrams/" + key + "/versions/" + key}, {"POST", "/v1/diagrams/" + key + "/versions/" + key + "/restore"},
		{"DELETE", "/v1/diagrams/" + key + "/versions/" + key},
		{"GET", "/v1/folders"}, {"POST", "/v1/folders"}, {"GET", "/v1/folders/" + key}, {"PUT", "/v1/folders/" + key},
		{"PATCH", "/v1/folders/" + key}, {"DELETE", "/v1/folders/" + key}, {"PUT", "/v1/folders/" + key + "/parent"},
		{"GET", "/v1/folders/" + key + "/members"}, {"PUT", "/v1/folders/" + key + "/members"}, {"DELETE", "/v1/folders/" + key + "/members/" + key},
	} {
		status, body, _ := anon.do(r.method, r.path, map[string]any{})
		require.Equal(t, 401, status, "%s %s must need a token", r.method, r.path)
		require.Equal(t, "UNAUTHORIZED", errorCode(t, body))
	}

	for name, token := range map[string]string{
		"garbage":   "not.a.jwt",
		"alg none":  "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4IiwiZXhwIjo5OTk5OTk5OTk5fQ.",
		"wrong key": signedWith(t, "another-secret-another-secret-another-1"),
	} {
		bad := &client{t: t, h: h, token: token}
		status, _, _ := bad.do("GET", "/v1/diagrams", nil)
		require.Equal(t, 401, status, name)
	}

	// The open routes stay open.
	status, _, _ := anon.do("GET", "/v1/shared/nope", nil)
	require.Equal(t, 404, status, "a public link is reachable without an account (and unknown tokens are just 404)")
}

func signedWith(t *testing.T, secret string) string {
	t.Helper()

	token, err := jwt.Sign(secret, "00000000-0000-0000-0000-0000000000a1", "", time.Hour, time.Now())
	require.NoError(t, err)

	return token
}

func TestAuthFlowOverHTTP(t *testing.T) {
	h := newHandler(t, nil)
	anon := &client{t: t, h: h}
	creds := map[string]any{"email": "Ann@Example.test", "name": "Ann", "password": "correct horse 9"}

	status, reg, _ := anon.do("POST", "/v1/auth/register", creds)
	require.Equal(t, 200, status)
	require.Equal(t, "ann@example.test", reg["user"].(map[string]any)["email"])
	require.NotContains(t, reg["user"], "password_hash", "a hash never leaves the server")
	require.NotContains(t, reg["user"], "password")

	status, body, _ := anon.do("POST", "/v1/auth/register", creds)
	require.Equal(t, 409, status)
	require.Equal(t, "ALREADY_EXISTS", errorCode(t, body))

	status, body, _ = anon.do("POST", "/v1/auth/register", map[string]any{"email": "x@example.test", "name": "X", "password": "short"})
	require.Equal(t, 400, status)
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))

	status, body, _ = anon.do("POST", "/v1/auth/login", map[string]any{"email": "ann@example.test", "password": "nope nope nope"})
	require.Equal(t, 401, status)
	require.Equal(t, "UNAUTHORIZED", errorCode(t, body))

	status, login, _ := anon.do("POST", "/v1/auth/login", map[string]any{"email": "ann@example.test", "password": "correct horse 9"})
	require.Equal(t, 200, status)

	me := &client{t: t, h: h, token: login["access_token"].(string)}
	status, who, _ := me.do("GET", "/v1/me", nil)
	require.Equal(t, 200, status)
	require.Equal(t, "Ann", who["name"])

	status, renamed, _ := me.do("PATCH", "/v1/me", map[string]any{"name": "Anna"})
	require.Equal(t, 200, status)
	require.Equal(t, "Anna", renamed["name"])

	status, next, _ := anon.do("POST", "/v1/auth/refresh", map[string]any{"refresh_token": login["refresh_token"]})
	require.Equal(t, 200, status)
	status, body, _ = anon.do("POST", "/v1/auth/refresh", map[string]any{"refresh_token": login["refresh_token"]})
	require.Equal(t, 401, status, "a rotated refresh token cannot be used again")
	require.Equal(t, "UNAUTHORIZED", errorCode(t, body))
	status, _, _ = anon.do("POST", "/v1/auth/refresh", map[string]any{"refresh_token": next["refresh_token"]})
	require.Equal(t, 401, status, "and the replay killed the whole login")

	status, _, _ = anon.do("POST", "/v1/auth/logout", map[string]any{"refresh_token": "anything"})
	require.Equal(t, 200, status, "logout never fails for the caller")

	status, body, _ = me.do("PUT", "/v1/me/password", map[string]any{"current_password": "wrong", "new_password": "brand new pass 1"})
	require.Equal(t, 403, status)
	require.Equal(t, "FORBIDDEN", errorCode(t, body))
}

func TestSignUpCanBeClosed(t *testing.T) {
	h := newHandler(t, func(c *config.Config) { c.RegistrationEnabled = false })
	anon := &client{t: t, h: h}

	status, body, _ := anon.do("POST", "/v1/auth/register", map[string]any{"email": "a@example.test", "name": "A", "password": "correct horse 9"})
	require.Equal(t, 403, status)
	require.Equal(t, "FORBIDDEN", errorCode(t, body))
}

func TestAuthRoutesAreRateLimitedPerRealClient(t *testing.T) {
	h := newHandler(t, func(c *config.Config) { c.AuthRateLimitPerMin = 3 })
	attacker := &client{t: t, h: h, headers: map[string]string{"X-Forwarded-For": "1.1.1.1"}}

	var last int
	var hdr http.Header
	for i := 0; i < 5; i++ {
		// A forged header per attempt must not buy a fresh allowance.
		attacker.headers["X-Forwarded-For"] = fmt.Sprintf("10.0.0.%d", i)
		last, _, hdr = attacker.do("POST", "/v1/auth/login", map[string]any{"email": "a@example.test", "password": "guess"})
	}

	require.Equal(t, 429, last)
	require.NotEmpty(t, hdr.Get("Retry-After"))
}

func TestSharingOverHTTP(t *testing.T) {
	h := newHandler(t, nil)
	olga, ed, vic, sam := newPerson(t, h, "olga"), newPerson(t, h, "ed"), newPerson(t, h, "vic"), newPerson(t, h, "sam")

	_, folder, _ := olga.do("POST", "/v1/folders", map[string]any{"name": "Team"})
	fpath := "/v1/folders/" + folder["guid"].(string)
	status, sub, _ := olga.do("POST", "/v1/folders", map[string]any{"name": "Sub", "parent_id": folder["guid"]})
	require.Equal(t, 200, status)
	require.Equal(t, folder["guid"], sub["parent_id"], "folders nest")
	require.Equal(t, "owner", sub["role"])

	_, d, _ := olga.do("POST", "/v1/diagrams", map[string]any{"name": "Shop", "dbml": "Table a {id int}", "folder_id": sub["guid"]})
	dpath := "/v1/diagrams/" + d["guid"].(string)
	require.Equal(t, olga.guid, d["owner_id"])
	require.Equal(t, "olga", d["owner_name"])
	require.Equal(t, float64(1), d["revision"])

	// Sharing: owner only, by email, role editor/viewer.
	status, _, _ = ed.do("PUT", fpath+"/members", map[string]any{"email": "sam@example.test", "role": "viewer"})
	require.Equal(t, 404, status, "a stranger cannot even tell the folder exists")
	status, m, _ := olga.do("PUT", fpath+"/members", map[string]any{"email": "ED@example.test", "role": "editor"})
	require.Equal(t, 200, status)
	require.Equal(t, "editor", m["role"])
	require.Equal(t, ed.guid, m["user_id"])
	status, _, _ = olga.do("PUT", fpath+"/members", map[string]any{"email": "vic@example.test", "role": "viewer"})
	require.Equal(t, 200, status)

	status, body, _ := olga.do("PUT", fpath+"/members", map[string]any{"email": "nobody@example.test", "role": "viewer"})
	require.Equal(t, 404, status)
	require.Equal(t, "NOT_FOUND", errorCode(t, body))
	status, _, _ = olga.do("PUT", fpath+"/members", map[string]any{"email": "sam@example.test", "role": "owner"})
	require.Equal(t, 400, status)
	status, _, _ = olga.do("PUT", fpath+"/members", map[string]any{"email": "sam@example.test"})
	require.Equal(t, 400, status, "role is required")

	status, list, _ := olga.do("GET", fpath+"/members", nil)
	require.Equal(t, 200, status)
	require.Equal(t, float64(3), list["count"])
	require.Equal(t, "owner", list["items"].([]any)[0].(map[string]any)["role"])
	status, body, _ = ed.do("GET", fpath+"/members", nil)
	require.Equal(t, 403, status, "an editor may not see who else has access")
	require.Equal(t, "FORBIDDEN", errorCode(t, body))

	// The inherited roles, as the callers see them.
	for who, role := range map[*client]string{olga: "owner", ed: "editor", vic: "viewer"} {
		status, got, _ := who.do("GET", dpath, nil)
		require.Equal(t, 200, status)
		require.Equal(t, role, got["role"])
		require.Equal(t, "", got["share_token"])
	}
	status, _, _ = sam.do("GET", dpath, nil)
	require.Equal(t, 404, status)

	status, _, _ = vic.do("PATCH", dpath, map[string]any{"name": "nope"})
	require.Equal(t, 403, status)
	status, body, _ = vic.do("DELETE", dpath, nil)
	require.Equal(t, 403, status)
	require.Equal(t, "FORBIDDEN", errorCode(t, body))
	status, _, _ = ed.do("PATCH", dpath, map[string]any{"name": "Shop v2"})
	require.Equal(t, 200, status)

	// Lists are per caller.
	for who, n := range map[*client]float64{olga: 1, ed: 1, vic: 1, sam: 0} {
		_, list, _ := who.do("GET", "/v1/diagrams?limit=1000", nil)
		require.Equal(t, n, list["count"])
		_, flist, _ := who.do("GET", "/v1/folders?limit=1000", nil)
		require.Equal(t, n*2, flist["count"], "the folder and its subfolder")
	}

	// Leaving, and revoking.
	status, _, _ = vic.do("DELETE", fpath+"/members/"+vic.guid, nil)
	require.Equal(t, 200, status, "a member can leave")
	status, _, _ = vic.do("GET", dpath, nil)
	require.Equal(t, 404, status)
	status, _, _ = olga.do("DELETE", fpath+"/members/"+ed.guid, nil)
	require.Equal(t, 200, status)
	status, _, _ = ed.do("GET", dpath, nil)
	require.Equal(t, 404, status)

	// Moves.
	status, body, _ = olga.do("PUT", fpath+"/parent", map[string]any{"parent_id": sub["guid"]})
	require.Equal(t, 400, status, "no cycles")
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))
	status, _, _ = olga.do("PUT", "/v1/folders/"+sub["guid"].(string)+"/parent", map[string]any{"parent_id": ""})
	require.Equal(t, 200, status)
	status, _, _ = olga.do("DELETE", fpath, nil)
	require.Equal(t, 200, status, "the parent is empty now")
}

func TestRevisionConflictOverHTTP(t *testing.T) {
	c := newClient(t)
	_, d, _ := c.do("POST", "/v1/diagrams", map[string]any{"name": "A", "dbml": "v1"})
	path := "/v1/diagrams/" + d["guid"].(string)

	status, saved, _ := c.do("PATCH", path, map[string]any{"dbml": "v2", "base_revision": 1})
	require.Equal(t, 200, status)
	require.Equal(t, float64(2), saved["revision"])

	status, body, _ := c.do("PATCH", path, map[string]any{"dbml": "v3", "base_revision": 1})
	require.Equal(t, 409, status)
	require.Equal(t, "REVISION_CONFLICT", errorCode(t, body))

	status, body, _ = c.do("PUT", path, map[string]any{"name": "A", "dbml": "v3", "base_revision": 1})
	require.Equal(t, 409, status, "PUT honours it too")
	require.Equal(t, "REVISION_CONFLICT", errorCode(t, body))

	status, got, _ := c.do("GET", path, nil)
	require.Equal(t, 200, status)
	require.Equal(t, "v2", got["dbml"], "the refused saves wrote nothing")

	status, rev, _ := c.do("GET", path+"/revision", nil)
	require.Equal(t, 200, status)
	require.Equal(t, float64(2), rev["revision"])
	require.Equal(t, "alice", rev["updated_by_name"])
}
