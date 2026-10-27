package api_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/config"
)

// withToken makes a personal access token for c through the real endpoint and
// returns a client that uses it instead of the login session.
func withToken(t *testing.T, c *client, scope string) *client {
	t.Helper()

	status, body, _ := c.do("POST", "/v1/me/tokens", map[string]any{"name": "test " + scope, "scope": scope})
	require.Equal(t, 200, status, "%v", body)
	require.True(t, strings.HasPrefix(body["token"].(string), "dbd_pat_"))

	return &client{t: t, h: c.h, token: body["token"].(string), guid: c.guid}
}

func TestPersonalAccessTokensOverHTTP(t *testing.T) {
	h := newHandler(t, nil)
	ann := newPerson(t, h, "ann")

	status, made, _ := ann.do("POST", "/v1/me/tokens", map[string]any{"name": "ci", "scope": "read", "expires_in_days": 30})
	require.Equal(t, 200, status)
	require.Contains(t, made, "token", "the secret is in the create response")

	status, list, _ := ann.do("GET", "/v1/me/tokens", nil)
	require.Equal(t, 200, status)
	require.Equal(t, float64(1), list["count"])
	item := list["items"].([]any)[0].(map[string]any)
	require.NotContains(t, item, "token", "the secret is never listed")
	require.Equal(t, made["prefix"], item["prefix"])
	require.Equal(t, "read", item["scope"])

	status, body, _ := ann.do("POST", "/v1/me/tokens", map[string]any{"name": "x", "scope": "root"})
	require.Equal(t, 400, status)
	require.Equal(t, "BAD_REQUEST", errorCode(t, body))

	read := &client{t: t, h: h, token: made["token"].(string)}
	status, me, _ := read.do("GET", "/v1/me", nil)
	require.Equal(t, 200, status, "a token authenticates like a login")
	require.Equal(t, "ann", me["name"])

	// A read token cannot change anything, even with the right role.
	status, body, _ = read.do("POST", "/v1/diagrams", map[string]any{"name": "nope"})
	require.Equal(t, 403, status)
	require.Equal(t, "FORBIDDEN", errorCode(t, body))
	status, _, _ = read.do("GET", "/v1/diagrams", nil)
	require.Equal(t, 200, status)

	// Account management needs a real session, for any scope.
	write := withToken(t, ann, "write")
	for _, r := range []struct{ method, path string }{
		{"PATCH", "/v1/me"}, {"PUT", "/v1/me/password"}, {"GET", "/v1/me/tokens"}, {"POST", "/v1/me/tokens"}, {"DELETE", "/v1/me/tokens/" + made["guid"].(string)},
	} {
		status, body, _ := write.do(r.method, r.path, map[string]any{"name": "x", "scope": "write"})
		require.Equal(t, 403, status, "%s %s: a token must not manage tokens or the account", r.method, r.path)
		require.Equal(t, "FORBIDDEN", errorCode(t, body))
	}
	status, d, _ := write.do("POST", "/v1/diagrams", map[string]any{"name": "by token", "dbml": "Table a {id int}"})
	require.Equal(t, 200, status, "a write token can do everything else")
	require.Equal(t, ann.guid, d["owner_id"])

	// Revocation is immediate; a stranger cannot revoke it.
	bob := newPerson(t, h, "bob")
	status, _, _ = bob.do("DELETE", "/v1/me/tokens/"+made["guid"].(string), nil)
	require.Equal(t, 404, status)
	status, _, _ = ann.do("DELETE", "/v1/me/tokens/"+made["guid"].(string), nil)
	require.Equal(t, 200, status)
	status, body, _ = read.do("GET", "/v1/me", nil)
	require.Equal(t, 401, status)
	require.Equal(t, "UNAUTHORIZED", errorCode(t, body))

	for name, token := range map[string]string{"unknown": "dbd_pat_" + strings.Repeat("A", 43), "truncated": "dbd_pat_", "wrong kind": "ghp_abc"} {
		status, _, _ := (&client{t: t, h: h, token: token}).do("GET", "/v1/me", nil)
		require.Equal(t, 401, status, name)
	}
}

// ---- MCP ---------------------------------------------------------------------

type rpc struct {
	status int
	body   map[string]any
}

func call(t *testing.T, c *client, method string, params any, id any) rpc {
	t.Helper()

	req := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		req["id"] = id
	}
	if params != nil {
		req["params"] = params
	}

	status, body, _ := c.do("POST", "/mcp", req)

	return rpc{status, body}
}

// tool calls a tool and returns (isError, text).
func tool(t *testing.T, c *client, name string, args map[string]any) (bool, string) {
	t.Helper()

	r := call(t, c, "tools/call", map[string]any{"name": name, "arguments": args}, 1)
	require.Equal(t, 200, r.status)
	require.Nil(t, r.body["error"], "tool problems are results, not protocol errors: %v", r.body["error"])

	res := r.body["result"].(map[string]any)
	isErr, _ := res["isError"].(bool)

	return isErr, res["content"].([]any)[0].(map[string]any)["text"].(string)
}

func toolJSON(t *testing.T, c *client, name string, args map[string]any) map[string]any {
	t.Helper()

	isErr, text := tool(t, c, name, args)
	require.False(t, isErr, "%s failed: %s", name, text)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out))

	return out
}

func TestMCPProtocol(t *testing.T) {
	h := newHandler(t, nil)
	ann := newPerson(t, h, "ann")
	agent := withToken(t, ann, "write")

	t.Run("needs credentials", func(t *testing.T) {
		status, _, _ := (&client{t: t, h: h}).do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
		require.Equal(t, 401, status)
		status, _, hdr := agent.do("GET", "/mcp", nil)
		require.Equal(t, 405, status)
		require.Equal(t, "POST", hdr.Get("Allow"))
	})

	t.Run("initialize negotiates the protocol version", func(t *testing.T) {
		r := call(t, agent, "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}}, 7)
		require.Equal(t, 200, r.status)
		require.Equal(t, float64(7), r.body["id"])
		res := r.body["result"].(map[string]any)
		require.Equal(t, "2025-03-26", res["protocolVersion"], "an older version we support is echoed")
		require.Contains(t, res["capabilities"], "tools")
		require.NotEmpty(t, res["instructions"])

		r = call(t, agent, "initialize", map[string]any{"protocolVersion": "1999-01-01"}, "abc")
		require.Equal(t, "abc", r.body["id"], "string ids are echoed as given")
		require.Equal(t, "2025-06-18", r.body["result"].(map[string]any)["protocolVersion"], "an unknown one gets our latest")
	})

	t.Run("notifications get no body", func(t *testing.T) {
		r := call(t, agent, "notifications/initialized", nil, nil)
		require.Equal(t, 202, r.status)
		require.Nil(t, r.body)
	})

	t.Run("errors use JSON-RPC codes", func(t *testing.T) {
		r := call(t, agent, "no/such/method", nil, 1)
		require.Equal(t, float64(-32601), r.body["error"].(map[string]any)["code"])

		r = call(t, agent, "tools/call", map[string]any{"name": "no_such_tool"}, 2)
		require.Equal(t, float64(-32602), r.body["error"].(map[string]any)["code"])

		status, body, _ := agent.do("POST", "/mcp", "{not json")
		require.Equal(t, 200, status)
		require.Equal(t, float64(-32700), body["error"].(map[string]any)["code"])

		status, body, _ = agent.do("POST", "/mcp", map[string]any{"id": 1, "method": "ping"})
		require.Equal(t, 200, status)
		require.Equal(t, float64(-32600), body["error"].(map[string]any)["code"], "jsonrpc must be 2.0")
	})

	t.Run("tools/list describes every tool with annotations", func(t *testing.T) {
		r := call(t, agent, "tools/list", nil, 3)
		tools := r.body["result"].(map[string]any)["tools"].([]any)
		require.Len(t, tools, 24)

		seen := map[string]map[string]any{}
		for _, x := range tools {
			tool := x.(map[string]any)
			name := tool["name"].(string)
			require.NotContains(t, seen, name, "duplicate tool name")
			seen[name] = tool
			require.NotEmpty(t, tool["description"], name)
			require.Equal(t, "object", tool["inputSchema"].(map[string]any)["type"], name)
			require.Contains(t, tool["annotations"], "readOnlyHint", name)
		}

		for _, name := range []string{"whoami", "list_diagrams", "get_diagram", "create_diagram", "update_diagram", "delete_diagram", "move_diagram", "set_public_link",
			"list_folders", "create_folder", "rename_folder", "move_folder", "delete_folder", "list_members", "share", "unshare",
			"list_versions", "get_version", "create_version", "restore_version", "delete_version", "validate_dbml", "describe_schema", "export_sql"} {
			require.Contains(t, seen, name)
		}
		require.Equal(t, true, seen["get_diagram"]["annotations"].(map[string]any)["readOnlyHint"])
		require.Equal(t, false, seen["update_diagram"]["annotations"].(map[string]any)["readOnlyHint"])
		require.Equal(t, true, seen["delete_diagram"]["annotations"].(map[string]any)["destructiveHint"])
	})

	t.Run("batches", func(t *testing.T) {
		status, body, _ := agent.do("POST", "/mcp", `[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"},{"jsonrpc":"2.0","id":2,"method":"ping"}]`)
		require.Equal(t, 200, status)
		require.Nil(t, body, "a batch answers with an array, which the harness leaves nil")
	})
}

func TestMCPToolsFollowTheSameAccessRulesAsTheAPI(t *testing.T) {
	h := newHandler(t, nil)
	olga, vic, sam := newPerson(t, h, "olga"), newPerson(t, h, "vic"), newPerson(t, h, "sam")
	agent, viewerAgent, strangerAgent := withToken(t, olga, "write"), withToken(t, vic, "write"), withToken(t, sam, "write")

	who := toolJSON(t, agent, "whoami", nil)
	require.Equal(t, "olga@example.test", who["email"])
	require.Equal(t, false, who["read_only"])

	folder := toolJSON(t, agent, "create_folder", map[string]any{"name": "Team"})
	sub := toolJSON(t, agent, "create_folder", map[string]any{"name": "Sub", "parent_id": folder["id"]})
	require.Equal(t, folder["id"], sub["parent_id"])

	d := toolJSON(t, agent, "create_diagram", map[string]any{"name": "Shop", "dbml": "Table a {id int}", "folder_id": sub["id"]})
	require.Equal(t, "owner", d["your_role"])
	require.Equal(t, float64(1), d["revision"])
	require.NotContains(t, d, "positions", "canvas geometry is not shown to models")

	list := toolJSON(t, agent, "list_diagrams", map[string]any{"search": "sho"})
	require.Equal(t, float64(1), list["total"])
	require.NotContains(t, list["diagrams"].([]any)[0], "dbml", "lists never carry the source")

	t.Run("sharing through a tool grants real access", func(t *testing.T) {
		m := toolJSON(t, agent, "share", map[string]any{"resource": "folder", "id": folder["id"], "email": "vic@example.test", "role": "viewer"})
		require.Equal(t, "viewer", m["role"])

		got := toolJSON(t, viewerAgent, "get_diagram", map[string]any{"diagram_id": d["id"]})
		require.Equal(t, "viewer", got["your_role"], "inherited through the folder")
		require.Equal(t, "Table a {id int}", got["dbml"])

		isErr, text := tool(t, viewerAgent, "update_diagram", map[string]any{"diagram_id": d["id"], "name": "hacked"})
		require.True(t, isErr)
		require.Contains(t, text, "editor", "the same 403 rule: a viewer cannot edit")

		isErr, text = tool(t, strangerAgent, "get_diagram", map[string]any{"diagram_id": d["id"]})
		require.True(t, isErr)
		require.Contains(t, text, "not found", "a stranger cannot tell it exists")
		require.Equal(t, float64(0), toolJSON(t, strangerAgent, "list_diagrams", nil)["total"])

		members := toolJSON(t, agent, "list_members", map[string]any{"resource": "folder", "id": folder["id"]})
		require.Len(t, members["members"], 2)
		isErr, _ = tool(t, viewerAgent, "list_members", map[string]any{"resource": "folder", "id": folder["id"]})
		require.True(t, isErr)
	})

	t.Run("a read-only token cannot call any write tool", func(t *testing.T) {
		readAgent := withToken(t, olga, "read")
		require.Equal(t, true, toolJSON(t, readAgent, "whoami", nil)["read_only"])
		require.Equal(t, "Shop", toolJSON(t, readAgent, "get_diagram", map[string]any{"diagram_id": d["id"]})["name"])

		r := call(t, readAgent, "tools/list", nil, 1)
		for _, x := range r.body["result"].(map[string]any)["tools"].([]any) {
			tl := x.(map[string]any)
			if tl["annotations"].(map[string]any)["readOnlyHint"] == true {
				continue
			}
			isErr, text := tool(t, readAgent, tl["name"].(string), map[string]any{"diagram_id": d["id"], "name": "x", "folder_id": "", "resource": "diagram", "id": d["id"], "email": "a@b.co", "role": "viewer", "user_id": "x", "version_id": "x", "enabled": true})
			require.True(t, isErr, "%s must be refused for a read token", tl["name"])
			require.Contains(t, text, "read-only", tl["name"])
		}
		require.Equal(t, "Table a {id int}", toolJSON(t, agent, "get_diagram", map[string]any{"diagram_id": d["id"]})["dbml"], "nothing changed")
	})

	t.Run("edits are conflict-checked and say how to recover", func(t *testing.T) {
		ok := toolJSON(t, agent, "update_diagram", map[string]any{"diagram_id": d["id"], "dbml": "Table b {id int}", "base_revision": 1})
		require.Equal(t, float64(2), ok["revision"])

		isErr, text := tool(t, agent, "update_diagram", map[string]any{"diagram_id": d["id"], "dbml": "Table c {id int}", "base_revision": 1})
		require.True(t, isErr)
		require.Contains(t, text, "someone else")
		require.Contains(t, text, "get_diagram", "the model is told what to do next")

		isErr, text = tool(t, agent, "update_diagram", map[string]any{"diagram_id": d["id"]})
		require.True(t, isErr)
		require.Contains(t, text, "name and/or dbml")
	})

	t.Run("bad arguments are reported, never silently ignored", func(t *testing.T) {
		isErr, text := tool(t, agent, "update_diagram", map[string]any{"diagram_id": d["id"], "dbml": "x", "base_revison": 2})
		require.True(t, isErr)
		require.Contains(t, text, "base_revison", "a misspelt base_revision must not quietly drop the conflict protection")

		isErr, text = tool(t, agent, "share", map[string]any{"resource": "team", "id": "x", "email": "a@b.co", "role": "viewer"})
		require.True(t, isErr)
		require.Contains(t, text, "diagram")

		isErr, _ = tool(t, agent, "set_public_link", map[string]any{"diagram_id": d["id"]})
		require.True(t, isErr, "enabled is required, a missing boolean is not false")
	})

	t.Run("versions, public link and cleanup", func(t *testing.T) {
		link := toolJSON(t, agent, "set_public_link", map[string]any{"diagram_id": d["id"], "enabled": true})
		require.Equal(t, true, link["public_link_enabled"])
		require.True(t, strings.HasPrefix(link["public_link_path"].(string), "/s/"))
		require.Empty(t, toolJSON(t, viewerAgent, "get_diagram", map[string]any{"diagram_id": d["id"]})["public_link_path"], "only owners see the link")

		v := toolJSON(t, agent, "create_version", map[string]any{"diagram_id": d["id"], "label": "before launch"})
		require.Equal(t, "before launch", v["label"])
		vs := toolJSON(t, agent, "list_versions", map[string]any{"diagram_id": d["id"]})
		require.GreaterOrEqual(t, vs["total"], float64(1))
		got := toolJSON(t, agent, "get_version", map[string]any{"diagram_id": d["id"], "version_id": v["guid"]})
		require.Equal(t, "Table b {id int}", got["dbml"])

		toolJSON(t, agent, "update_diagram", map[string]any{"diagram_id": d["id"], "dbml": "Table z {id int}"})
		restored := toolJSON(t, agent, "restore_version", map[string]any{"diagram_id": d["id"], "version_id": v["guid"]})
		require.Equal(t, "Table b {id int}", restored["dbml"])
		toolJSON(t, agent, "delete_version", map[string]any{"diagram_id": d["id"], "version_id": v["guid"]})

		isErr, text := tool(t, agent, "delete_folder", map[string]any{"folder_id": folder["id"]})
		require.True(t, isErr, "a non-empty folder is refused")
		require.Contains(t, text, "not empty")

		toolJSON(t, agent, "move_diagram", map[string]any{"diagram_id": d["id"], "folder_id": ""})
		toolJSON(t, agent, "unshare", map[string]any{"resource": "folder", "id": folder["id"], "user_id": vic.guid})
		toolJSON(t, agent, "move_folder", map[string]any{"folder_id": sub["id"], "parent_id": ""})
		toolJSON(t, agent, "rename_folder", map[string]any{"folder_id": sub["id"], "name": "Sub2"})
		toolJSON(t, agent, "delete_folder", map[string]any{"folder_id": sub["id"]})
		toolJSON(t, agent, "delete_folder", map[string]any{"folder_id": folder["id"]})
		toolJSON(t, agent, "delete_diagram", map[string]any{"diagram_id": d["id"]})
		require.Equal(t, float64(0), toolJSON(t, agent, "list_diagrams", nil)["total"])
	})
}

func TestMCPRejectsForeignBrowserOrigins(t *testing.T) {
	h := newHandler(t, nil) // develop, no CORS list: all origins allowed
	agent := withToken(t, newPerson(t, h, "ann"), "read")
	agent.headers = map[string]string{"Origin": "https://evil.example"}
	status, _, _ := agent.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	require.Equal(t, 200, status, "outside production an unlisted CORS config allows any origin")

	prod := newHandler(t, func(c *config.Config) {
		c.Environment = "production"
		c.CORSAllowedOrigins = []string{"https://app.example.test"}
	})
	agent = withToken(t, newPerson(t, prod, "ann"), "read")
	agent.headers = map[string]string{"Origin": "https://evil.example"}
	status, _, _ = agent.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	require.Equal(t, 403, status, "DNS-rebinding guard")

	agent.headers = map[string]string{"Origin": "https://app.example.test"}
	status, _, _ = agent.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	require.Equal(t, 200, status)

	agent.headers = nil
	status, _, _ = agent.do("POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	require.Equal(t, 200, status, "non-browser agents send no Origin")
}

func TestMCPDBMLTools(t *testing.T) {
	h := newHandler(t, nil)
	ann := newPerson(t, h, "ann")
	agent := withToken(t, ann, "read") // all three are read-only tools

	good := "Table users {\n  id int [pk, increment]\n  email varchar [not null, unique]\n}\nTable posts {\n  id int [pk]\n  user_id int\n}\nRef: posts.user_id > users.id"

	v := toolJSON(t, agent, "validate_dbml", map[string]any{"dbml": good})
	require.Equal(t, true, v["valid"])
	require.Equal(t, float64(2), v["tables"])

	v = toolJSON(t, agent, "validate_dbml", map[string]any{"dbml": "Table a {\n  id int [pk\n}\nRef: b.x > nope.id"})
	require.Equal(t, false, v["valid"])
	require.Greater(t, v["error_count"], float64(1), "all errors in one pass")
	first := v["errors"].([]any)[0].(map[string]any)
	require.NotZero(t, first["line"], "errors carry positions")
	require.NotEmpty(t, first["message"])

	made := toolJSON(t, withToken(t, ann, "write"), "create_diagram", map[string]any{"name": "S", "dbml": good})
	desc := toolJSON(t, agent, "describe_schema", map[string]any{"diagram_id": made["id"]})
	require.Contains(t, desc["summary"], "users(")
	require.Contains(t, desc["summary"], "posts.user_id > users.id")

	sql := toolJSON(t, agent, "export_sql", map[string]any{"diagram_id": made["id"], "dialect": "postgres"})
	require.Contains(t, sql["sql"], "CREATE TABLE")
	require.Contains(t, sql["sql"], "FOREIGN KEY")
	my := toolJSON(t, agent, "export_sql", map[string]any{"dbml": good, "dialect": "mysql"})
	require.Contains(t, my["sql"], "AUTO_INCREMENT")

	for name, args := range map[string]map[string]any{
		"both sources": {"diagram_id": made["id"], "dbml": good, "dialect": "postgres"},
		"no source":    {"dialect": "postgres"},
		"bad dialect":  {"dbml": good, "dialect": "oracle"},
		"broken dbml":  {"dbml": "Table {", "dialect": "postgres"},
	} {
		isErr, _ := tool(t, agent, "export_sql", args)
		require.True(t, isErr, name)
	}

	// Reading by id is access-checked like everywhere else.
	stranger := withToken(t, newPerson(t, h, "sam"), "read")
	isErr, text := tool(t, stranger, "describe_schema", map[string]any{"diagram_id": made["id"]})
	require.True(t, isErr)
	require.Contains(t, text, "not found")
}
