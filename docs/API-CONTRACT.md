# dbdiagram API contract (v1)

Shared by `backend/` and `frontend/`. Mirrors the my_fidani conventions:
snake_case JSON, no `omitempty` (empty string = "none"), `{ items, count }`
lists, `{ error: { code, message } }` errors. Multi-user: every route except the public ones needs a bearer token (see **Auth & access**).

Base URL: `${VITE_API_BASE_URL}/v1` (backend default `http://localhost:8080`).

## Models

```jsonc
// Diagram (full) — GET one, create/update responses
{ "guid": "uuid", "name": "", "dbml": "", "positions": { "<tableId>": { "x": 0, "y": 0 } },
  "folder_id": "",      // "" = not in a folder, OR in a folder the caller cannot see
  "share_token": "",    // "" = no public link (only owner-rank callers ever see the token)
  "owner_id": "uuid", "owner_name": "",
  "role": "owner",      // the CALLER'S effective role: "owner" | "editor" | "viewer"
  "revision": 1,        // bumps whenever the dbml changes (see Concurrency)
  "updated_by_name": "",
  "created_at": "RFC3339", "updated_at": "RFC3339" }

// DiagramListItem — list only (no dbml / positions: they are large)
{ "guid", "name", "folder_id", "share_token", "owner_id", "owner_name", "role", "created_at", "updated_at" }

// Folder (flat; the client builds the tree)
{ "guid", "name",
  "parent_id": "",      // "" = top level, OR the parent is a folder the caller cannot see
  "owner_id", "owner_name", "role": "owner|editor|viewer",
  "created_at", "updated_at" }

// SharedDiagram — public, read-only
{ "name", "dbml", "positions" }

// List<T>
{ "items": [T], "count": 0 }   // count = total under the filter, not page size
```

Limits: `name` 1..120 chars, `dbml` <= 1 MiB, `positions` must be an object of
`{x, y}` numbers (unknown shapes are a 400).

## Errors

HTTP status + body `{ "error": { "code": "BAD_REQUEST", "message": "..." } }`.
Codes: `BAD_REQUEST` 400, `INVALID_JSON` 400, `NOT_FOUND` 404,
`ALREADY_EXISTS` 409, `CONFLICT` 409, `PAYLOAD_TOO_LARGE` 413, `UNAUTHORIZED` 401, `FORBIDDEN` 403, `REVISION_CONFLICT` 409, `RATE_LIMITED` 429 (with `Retry-After`), `UNAVAILABLE` 503, `INTERNAL` 500
(message is always "Internal Server Error"; the cause is only in the server log).

## Auth & access

Single-page-app friendly: JSON tokens, no cookies. Send `Authorization: Bearer <access_token>`
on every request except: `GET /v1/health`, `POST /v1/auth/register|login|refresh`,
`GET /v1/shared/:key`. A missing/expired/invalid access token is **401 `UNAUTHORIZED`**; the
client then calls refresh once and retries, and on failure goes to the login page.

```jsonc
// User
{ "guid": "uuid", "email": "a@b.co", "name": "Ann", "created_at": "RFC3339" }
// AuthResponse
{ "user": User, "access_token": "jwt", "refresh_token": "opaque", "expires_in": 900 }   // seconds
```

| Method | Path | Body | Returns |
|---|---|---|---|
| POST | `/v1/auth/register` | `{email, name, password}` (password 8..72 bytes) | `AuthResponse` (409 `ALREADY_EXISTS` if the email is taken; 403 `FORBIDDEN` if sign-up is disabled) |
| POST | `/v1/auth/login` | `{email, password}` | `AuthResponse` (401 `UNAUTHORIZED` "invalid email or password", never says which) |
| POST | `/v1/auth/refresh` | `{refresh_token}` | `AuthResponse` — the refresh token is **rotated**: the old one stops working; reusing an old one revokes the whole session family |
| POST | `/v1/auth/logout` | `{refresh_token}` | `{}` |
| GET | `/v1/me` | | `User` |
| PATCH | `/v1/me` | `{name}` | `User` |
| PUT | `/v1/me/password` | `{current_password, new_password}` | `{}` (other sessions are revoked) |

Emails are trimmed and lower-cased. Auth routes are rate limited per IP (429 `RATE_LIMITED`).

### Roles and inheritance

`owner` > `editor` > `viewer`. A caller's role on a **folder** is the highest of: owning it or
any ancestor folder (`owner`); a share on it or any ancestor (`editor`/`viewer`). On a
**diagram**: owning it (`owner`), a direct share, or any of the above on its folder chain.
Folders nest up to 8 levels.

| Action | Needs |
|---|---|
| read a diagram/folder, its versions, its revision | viewer |
| edit dbml/positions/name, create/restore/delete versions, create a diagram or subfolder inside a folder, rename a folder | editor |
| delete, move, change the public link, manage members (list / add / change / remove others) | owner |
| a member removing **themselves** (leave) | any direct member |

A caller with **no** access gets **404** (existence is not revealed); with some access but too little,
**403 `FORBIDDEN`**. Moving a diagram/folder needs `owner` on it and `editor` on the destination.

### Members

```jsonc
// Member
{ "user_id": "uuid", "email": "", "name": "", "role": "owner|editor|viewer", "created_at": "RFC3339" }
```

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/v1/folders/:key/members` | | `List<Member>` — the owner first (role `owner`), then direct shares |
| PUT | `/v1/folders/:key/members` | `{email, role: "editor"\|"viewer"}` | `Member` (add or change; 404 `NOT_FOUND` "no user with that email"; 400 for the owner / yourself) |
| DELETE | `/v1/folders/:key/members/:user` | | `{}` |
| GET / PUT / DELETE | `/v1/diagrams/:key/members[/:user]` | same | same, for one diagram |

Direct shares only: access inherited from an ancestor folder is changed on that folder.

### Concurrency (several editors)

`Diagram.revision` bumps whenever the **dbml** changes (not for positions or the name). `PATCH`/`PUT`
accept `base_revision` (int): if `dbml` is in the body and `base_revision` differs from the stored
revision, nothing is written and the answer is **409 `REVISION_CONFLICT`** — the client must reload
or resend with the new revision. Without `base_revision` the write wins (used by restore etc.).
Positions are last-write-wins.

`GET /v1/diagrams/:key/revision` → `{ "revision": 1, "updated_at": "RFC3339", "updated_by_name": "" }`
(tiny; the editor polls it every few seconds to notice other people's edits).

### Existing data

Diagrams/folders created before auth have no owner and are invisible until claimed:
`make claim email=you@x.co` (backend) assigns every unowned diagram and folder to that user.

## Diagrams

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/v1/diagrams?search&filters&sort&page&limit` | | `List<DiagramListItem>` |
| POST | `/v1/diagrams` | `{name, dbml, positions, folder_id?}` (`folder_id` needs `editor` on it) | `Diagram` (caller becomes owner) |
| GET | `/v1/diagrams/:key` | | `Diagram` |
| PUT | `/v1/diagrams/:key` | `{name, dbml, positions, base_revision?}` — `name` required; omitted `dbml`/`positions` become empty | `Diagram` |
| PATCH | `/v1/diagrams/:key` | any of `{name?, dbml?, positions?}` plus `base_revision?` | `Diagram` |
| PUT | `/v1/diagrams/:key/folder` | `{folder_id}` (`""` removes; unknown folder -> 400) | `Diagram` |
| PUT | `/v1/diagrams/:key/share` | `{enabled: bool}` | `Diagram` (`share_token` set / `""`) |
| DELETE | `/v1/diagrams/:key` | | `{}` |
| GET | `/v1/shared/:key` | | `SharedDiagram` (`:key` = share token) |

List contract: `search` (ILIKE on name), `filters` = JSON array
`[{field,type,value}]` (fields: `folder_id`, `owner_id`, `name`, `created_at`, `updated_at`;
types: `eq neq search gt gte lt lte`), `sort` = JSON array `[{field,type}]`
(`type` = `asc|desc`; fields: `name created_at updated_at`; default
`updated_at desc`), `page` (>=1, default 1), `limit` (default 10, max 1000,
larger is clamped). Unknown field/type => 400.

## Folders

Folders nest (`parent_id`). The list is flat and contains every folder the caller can reach.

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/v1/folders?search&sort&page&limit` | | `List<Folder>` (default sort `name asc`; the client asks `limit=1000`) |
| POST | `/v1/folders` | `{name, parent_id?}` (`parent_id` needs `editor` on it) | `Folder` |
| GET | `/v1/folders/:key` | | `Folder` |
| PUT | `/v1/folders/:key` | `{name}` | `Folder` |
| PATCH | `/v1/folders/:key` | `{name?}` | `Folder` |
| PUT | `/v1/folders/:key/parent` | `{parent_id}` (`""` = top level) | `Folder` — 400 when it would create a cycle or exceed the depth limit |
| DELETE | `/v1/folders/:key` | | `{}` — 409 `CONFLICT` "folder is not empty" while it holds folders or diagrams |

## Versions (history)

Snapshots of a diagram's name-less content (`dbml` + `positions`), newest first.
The server takes them automatically: when a save **changes the DBML**, the state
*before* that save is snapshotted if the newest snapshot is >= 5 minutes old (and
differs from it). Dragging tables (positions only) never creates one. At most
100 are kept per diagram (oldest dropped). Restoring snapshots the current state
first, so a restore can itself be undone.

```jsonc
// VersionItem — list / create responses (no dbml: it is large)
{ "guid": "uuid", "diagram_id": "uuid", "label": "", "created_at": "RFC3339" }   // label "" = automatic; a manual save without a label is "Manual save"
// Version — GET one
{ "guid", "diagram_id", "label", "dbml": "", "positions": { }, "created_at" }
```

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/v1/diagrams/:key/versions?page&limit` | | `List<VersionItem>` (newest first) |
| POST | `/v1/diagrams/:key/versions` | `{label?}` (<= 120 chars) | `VersionItem` — a manual snapshot of the saved state |
| GET | `/v1/diagrams/:key/versions/:version` | | `Version` |
| POST | `/v1/diagrams/:key/versions/:version/restore` | | `Diagram` — dbml + positions replaced, name kept |
| DELETE | `/v1/diagrams/:key/versions/:version` | | `{}` |

A version that does not belong to `:key` is a 404, never another diagram's data. Reading versions needs `viewer`; creating, restoring and deleting need `editor`.
Unsaved edits are not in any snapshot: the client must flush its autosave first.

## Personal access tokens (PATs)

Long-lived credentials for scripts and AI agents (the MCP endpoint below). Created only from a
logged-in browser session, never with another PAT.

```jsonc
// AccessToken — list / create responses
{ "guid": "uuid", "name": "claude-desktop", "prefix": "dbd_pat_AbCd", "scope": "read|write",
  "expires_at": "RFC3339", "last_used_at": "RFC3339|\"\"", "created_at": "RFC3339" }
// CreatedAccessToken — create response only: the secret is shown ONCE and never stored
{ ...AccessToken, "token": "dbd_pat_<43 random chars>" }
```

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/v1/me/tokens` | | `List<AccessToken>` (active only: not revoked, not expired) |
| POST | `/v1/me/tokens` | `{name (1..60), scope: "read"\|"write", expires_in_days? (1..365, default 90)}` | `CreatedAccessToken` (400 once the user has 20 active tokens) |
| DELETE | `/v1/me/tokens/:key` | | `{}` (404 for someone else's token) |

Use it exactly like an access JWT: `Authorization: Bearer dbd_pat_...`. Differences:
- `read` tokens may only make safe requests (`GET`) and call read-only MCP tools; `write` tokens may do everything
  a user can do **except** account management: `/v1/me` (PATCH), `/v1/me/password` and `/v1/me/tokens*` need a real
  login session (403 `FORBIDDEN` for a PAT).
- A revoked, expired or unknown token is 401 `UNAUTHORIZED`. Only the sha256 is stored. `last_used_at` is updated at most once a minute.
- Changing the password does not revoke PATs (revoke them in the UI); deleting a token takes effect immediately.

## MCP endpoint (Model Context Protocol)

`POST /mcp` — MCP **Streamable HTTP**, stateless JSON-RPC 2.0 (protocol version `2025-06-18`; older `2025-03-26` and
`2024-11-05` clients are accepted). Same bearer rules as above (PAT or session JWT). Methods: `initialize`,
`notifications/initialized`, `ping`, `tools/list`, `tools/call`. `GET /mcp` → 405. A browser `Origin` that is not in
`CORS_ALLOWED_ORIGINS` is rejected (DNS-rebinding guard); non-browser clients send none.
Tool errors are returned as `isError: true` results with a message safe to show the model; protocol errors use JSON-RPC codes.

Every tool wraps the same service call as the REST API and therefore the same owner/editor/viewer checks. Tools:

| Tool | Needs | What it does |
|---|---|---|
| `whoami` | read | the account behind the token |
| `list_diagrams` | read | `search?, folder_id?, page?, limit?` -> diagrams (no dbml) |
| `get_diagram` | read | `diagram_id` -> name, dbml, revision, role, folder, owner |
| `describe_schema` | read | `diagram_id` -> parsed tables/columns/refs summary (from the DBML) |
| `create_diagram` | write | `name, dbml?, folder_id?` |
| `update_diagram` | write | `diagram_id, name?, dbml?, base_revision?` (conflict -> tool error telling to re-read) |
| `delete_diagram` | write | `diagram_id` |
| `move_diagram` | write | `diagram_id, folder_id` (`""` = top level) |
| `set_public_link` | write | `diagram_id, enabled` -> returns the public URL path when on |
| `list_folders` | read | `search?` -> flat folders with `parent_id` and `role` |
| `create_folder` / `rename_folder` / `move_folder` / `delete_folder` | write | like REST |
| `list_members` | read | `resource: "diagram"\|"folder", id` (owner role needed) |
| `share` | write | `resource, id, email, role: "editor"\|"viewer"` |
| `unshare` | write | `resource, id, user_id` |
| `list_versions` / `get_version` | read | history of a diagram |
| `create_version` / `restore_version` / `delete_version` | write | like REST |
| `validate_dbml` | read | `dbml` -> `{valid, errors:[{line,column,message}]}` |
| `export_sql` | read | `diagram_id` or `dbml`, `dialect: "postgres"\|"mysql"` -> SQL text |

Read-only tools carry the MCP annotation `readOnlyHint: true`; destructive ones `destructiveHint: true`.
A `read` token calling a write tool gets a tool error. Importing SQL and PNG/PDF export are browser features and are not exposed.

## Misc

`GET /v1/health` -> `{ "status": "ok" }`. Swagger UI at `/swagger/index.html`.
