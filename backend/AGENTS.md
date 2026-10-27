# Working in backend/

A **modular monolith** — one binary (`serve | migrate | claim`), one Postgres — laid out
like `my_fidani/myfidani_go_backend` (its `MONOLITH-ARCHITECTURE.md` is the
reference; this file is the short version, plus where we differ).

## Layers and import direction

```
api → internal/module → internal/storage → pkg
```

One way only. A reverse import is a review rejection.

| Layer | Knows | Does NOT know |
|---|---|---|
| `api/` (handler, dto, middleware) | gin, HTTP status, swagger, `model`, `apperr` | SQL, squirrel, table names |
| `internal/module/<name>/` | business rules, `repo` interfaces, `apperr` | gin, `*gin.Context`, HTTP status, SQL |
| `internal/storage/` (`repo`, `postgres`, `models`) | SQL, squirrel, tables | business rules, HTTP |
| `pkg/` (`db`, `logger`) | its own technical job | the domain, `internal/` |

`internal/app` is the only place the graph is wired. `cmd/app/main.go` holds no
logic: config → logger → app → run.

## Non-negotiable rules

- Every SELECT filters `deleted_at IS NULL` (`postgres.NotDeleted`). Deletes are soft.
- Filter and sort names go through a registry (`diagramFilterable`, `diagramSortable`);
  unknown fields/types/values are a 400, never SQL. Search escapes `% _ \`.
- Every list: `search, filters, sort, page, limit`, returns `{items, count}`, orders by
  the requested key **plus a unique tiebreak** (`guid`), limit clamped to `config.MaxLimit`.
- `Update`/`Delete` match on `guid AND deleted_at IS NULL`; `RowsAffected()==0` → NotFound.
- A write spanning more than one table runs in `storage.Tx(ctx, fn)`.
- Every error leaves storage as an `apperr` (`HandleDatabaseError`); a driver error never
  reaches a client. 5xx bodies are always "Internal Server Error".
- Handlers pass `c.Request.Context()`; never `context.Background()` in a request chain.
- PUT replaces the whole mutable representation; PATCH uses pointer fields so an omitted
  field differs from an explicit `""`/`{}`. Both run the same service validation.
- No `omitempty` on responses (`""` means "none"). JSON is snake_case.
- Every handler has a full swagger block; run `make swag_init` after changing one.
- An applied migration is never edited — add a new pair with a working `down`.
  Table → FK → index → trigger go in separate files.
- No secret in code or in a config default. Request bodies are capped (`config.MaxBodySize`).
- Hand-written Go follows the `myfidani-go-code-style` skill (grouped `var` blocks,
  validate first, early returns, no one-use helpers).

## Adding a resource (in order)

migration → `internal/model` → `storage/models` → `storage/repo` interface →
`storage/postgres` impl → `storage.go` (**four** places: interface method, struct
field, `newStorage`, getter) → service in the module (+ `module.go`) →
`api/dto` → `api/handler/<module>_<resource>.go` → `router.go` → `make swag_init` →
tests → `make verify`.

## History (diagram versions)

- Snapshots hold `dbml` + `positions` only (never the name). They are immutable, soft deleted,
  scoped by diagram guid on **every** read (a version is never reachable through another
  diagram's key) and hidden with their diagram (`versionFrom` joins the live diagram).
- Automatic rule (`diagrams.captureIfDue`, in the save's own transaction): a save that
  *changes the DBML* first snapshots the state it is about to overwrite, at most once per
  `config.VersionInterval` and never when the newest snapshot already holds that DBML or the
  DBML is empty. Position-only saves, renames and no-op saves never snapshot.
- Restore snapshots the current state first (label "Before restore"), so it can be undone;
  history is capped at `config.MaxVersions` per diagram (`Prune`, oldest first).
- Time comes from `Deps.Now` (tests move it by hand) and is passed to the repo as
  `VersionCreate.CreatedAt`; the API shows seconds, the column keeps full precision.
- Layout reaches jsonb only through `postgres.encodePositions` (nil -> `{}`): the CHECK
  constraint rejects JSON `null`.

## Auth, ownership and sharing

- **Modules:** `core` owns accounts (users, sessions); `diagrams` owns diagrams, folders, versions and shares and
  reaches users only through its own `UserGateway` interface (`Deps.Users`, implemented by `core`).
- **Tokens:** a short-lived HS256 access JWT (`sub` = user guid, `fam` = login family) and an opaque refresh token
  stored only as a sha256 hash. Refresh **rotates**: the old token is revoked by a single conditional UPDATE
  (`SessionI.Revoke` returns whether *this* call did it), so of two concurrent refreshes only one wins; a replayed
  token revokes the whole family (outside the failed transaction, so it sticks). `serve` refuses to start with a
  `JWT_SECRET` shorter than 32 chars. Login runs bcrypt against a dummy hash for unknown emails (no timing oracle) and
  returns one message for "no such email" and "wrong password".
- **Authorization lives in the service, never the handler.** Every service method that takes a diagram or folder guid
  first calls `requireDiagram` / `requireFolder` (`internal/module/diagrams/access.go`) with the minimum role: no access
  at all = **404** (existence is not revealed), some but too little = **403**. A new method that skips it is a bug;
  `authz_test.go` is the matrix to extend.
- **Roles** `owner > editor > viewer`. Role on a folder = best of owning it/any ancestor and a share on it/any
  ancestor; on a diagram = owner, direct share, or its folder chain. The rule is implemented twice — SQL (recursive CTEs
  in `postgres/diagrams_access.go`) and the in-memory fake — and **the same scenario runs against both**
  (`TestAuthzInMemory` / `TestAuthzPostgres`); change one, change both.
- **Lists** (`Find`) take the actor and filter in SQL through the `accessPrefix` CTE (`facc` = folders the actor can
  reach with their rank). Parents/folders the caller cannot see are returned as `""`; public-link tokens only to
  owner-rank callers.
- **Folders** nest (`parent_guid`, max `config.MaxFolderDepth`), moves reject cycles (`IsInside`) and over-deep trees
  (`Depth` + `Height`); deleting a non-empty folder is a 409 (nothing is silently moved or hidden).
- **Shares** (`folder_members`, `diagram_members`) are hard-deleted: a revoked share is not history. The owner is
  `owner_guid`, never a row. Only owner-rank may list/change shares; anyone may remove their *own* row (leave).
- **Concurrency:** `diagrams.revision` bumps when the DBML changes; `base_revision` makes a save conditional *in the
  same UPDATE statement* (409 `REVISION_CONFLICT` otherwise). Positions are last-write-wins.
- **Rate limiting:** `RateLimit` (in-memory, per client IP) on register/login/refresh/password. The client IP is only
  trustworthy because the engine trusts no proxy header unless `TRUSTED_PROXIES` lists it — set that behind a proxy.
- **Old data:** rows created before accounts have `owner_guid IS NULL` and are unreachable until
  `make claim email=you@x.co` (`app claim -email=...`) gives them to a registered user.
- **Sign-up** is open unless `REGISTRATION_ENABLED=false`. There is no email verification, password reset or
  account deletion yet.

## Personal access tokens and MCP

- **Tokens** (`core.TokenService`, table `access_tokens`): `dbd_pat_` + 256 random bits; only the sha256 is stored,
  the secret is returned once at creation. 90 days by default (max 365, no "never"), max 20 active per user, scope
  `read` or `write`. `Authenticate` gives one identical error for unknown, revoked, expired and malformed secrets and
  records `last_used_at` at most once a minute (a failure to record never fails the request).
- **Middleware** (`api/middleware/auth.go`): a bearer starting with `dbd_pat_` is a token, anything else a JWT. A read
  token is refused on every non-GET (`/mcp` decides per tool instead). `RequireSession` guards account management
  (`PATCH /me`, password, and `/me/tokens*`): a token must never be able to mint its own replacement. A new account-level
  route belongs behind it.
- **MCP** (`api/mcp`): stateless Streamable-HTTP JSON-RPC on `POST /mcp` (no /v1 prefix), same credentials as the API.
  It is an HTTP adapter like a handler: each tool decodes strictly (`bind` rejects unknown fields so a misspelt
  `base_revision` is reported, not dropped), calls ONE service method as the caller, and returns the result as JSON text.
  **It must never touch storage or do its own permission checks** — authorization stays in the services, so a tool can
  do exactly what the same person can do over REST. Tools marked `Write` are refused for read tokens; destructive ones
  carry `destructiveHint`. A foreign browser `Origin` is refused in production (DNS-rebinding guard).
- **Adding a feature** means adding its tool: put it in the matching `api/mcp/tools_*.go`, extend
  `docs/API-CONTRACT.md` (tool table) and `api/mcp_test.go` (the read-token test loops over every write tool, so a
  forgotten `Write: true` fails it).
- nginx must forward `/mcp` (see `frontend/nginx.conf`).

## Where we differ from my_fidani

- **No tenant (`company_id`):** ownership + shares replace it. There is no permission-menu system (`RequireMenu`); the
  three roles above are the whole model.
- No Redis cache, S3, cron, SMTP; the matching config and deps are simply absent.
- Swagger uses concrete `dto.DiagramList`/`FolderList`/`VersionList`/`MemberList` because swag cannot resolve the
  generic `model.List[...]` from another package. The JSON is identical.
- Limits live in `config/constants.go` and are enforced in the service.

## Tests

- `make verify` — build, vet, `go test -short -race` (no database needed).
- `make test` — also runs everything that needs Postgres. `pgtest.Open(t, "<name>")` creates and migrates a
  database `dbdiagram_test_<name>` per test package on first use and truncates it per test, so packages can run in
  parallel; it skips when Postgres is unreachable or with `-short`.
- Service/handler tests use the in-memory `internal/storage/storagetest`; SQL behaviour (filters, sort, pagination,
  soft delete, FK, Tx rollback, roles, tree queries, session rotation) is covered by the Postgres suites.
- `internal/module/testkit` wires both modules over a storage with a clock the test moves by hand.

## One-off

`scripts/import_legacy.sql` copied the pre-refactor prototype rows (bigint ids) into the `dbdiagram` schema. It has been
run on the dev database; the old tables were renamed to `public.legacy_*`, not dropped — drop them once you are sure.
