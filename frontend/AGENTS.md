# Working in the frontend

React 19 + TypeScript + Vite. The folder layout follows `my_fidani/myfidani_react_admin`,
trimmed to what this app needs. API shapes live in `../docs/API-CONTRACT.md`.

## Layers and import direction

```
api, services, types, lib  →  stores, hooks  →  features, layout, components  →  app
```

Imports only point left-to-right as written; a reverse import is a review rejection.

| Folder | Holds | Must not |
|---|---|---|
| `config/` | `env.ts` — the only reader of `import.meta.env` | import anything else |
| `types/` | `api.ts` — wire types, snake_case, straight from the contract | contain logic |
| `api/` | axios client, `ApiError`, list-query serializer | know any endpoint |
| `services/<domain>/` | `<name>.service.ts`: one typed function per endpoint, takes an `AbortSignal` for reads | touch stores or React |
| `lib/` | pure helpers: DBML parse/export, layout, save queue, export, debounce | import React state or services |
| `stores/` | zustand stores (auth, diagram, folder, history, ui, toast). All server writes go through them. `authStore` is the only one that may reset the others | render anything |
| `hooks/` | `useAsync`, `useParsedDbml` | hold app state |
| `components/ui/` | small reusable controls (Button, Dialog, Menu, Toaster, ...) | import features or stores (except Toaster) |
| `features/<area>/` | screens and their parts (`auth`, `editor`, `diagrams`, `sharing`, `history`, `shared`) | import another feature's internals (the shared view reuses the editor's canvas/editor components on purpose) |
| `layout/` | app shell frames; content comes in as props | import features |
| `app/` | router, root component, route errors | hold logic |
| `styles/` | `tokens.css` + one file per area, all imported from `index.css` | — |

## Rules

- `@/` is the alias for `src/`. No deep relative imports across folders.
- Every rejected request is an `ApiError` (`api/errors.ts`). Show failures with
  `notifyError(e, 'Context')`; never leave a promise unhandled.
- Autosave: `diagramStore` sends `PATCH` with only the changed fields through a
  serial save queue (`lib/saveQueue.ts`). Do not call the service directly from
  components. Never mark a diagram dirty from a render effect.
- Default table positions are computed (`lib/layout.ts`), never written back
  until the user changes something.
- The diagram's identity is `guid`. Empty string means "none" (`folder_id`, `share_token`).
- Heavy modules load on demand: Monaco (`features/editor/monaco.ts`), `@dbml/parse`,
  `@dbml/core` (SQL import/export only), `jspdf`, `html-to-image`. Do not import
  them at module top level outside their lazy chunk. Monaco is bundled locally — no CDN.
- History (`features/history`, `stores/historyStore.ts`): versions snapshot the *saved* state, so
  flush autosave before creating or restoring one. A previewed version is shown by the editor
  *instead of* the diagram (the stored diagram is never touched), read only. A server-side
  replacement (restore) goes through `applyServerDiagram`, never through `setDbml`, so it is not
  saved back. `DbmlEditor` ignores `onChange` calls that merely echo the `value` prop: Monaco
  reports programmatic text swaps as changes, and without that guard previewing a version
  overwrote the real diagram.
- **Auth** (`stores/authStore.ts`, `api/http.ts`, `app/guards.tsx`): the access token is in memory only; the
  refresh token is in `localStorage` (`auth.refresh`) — readable by XSS, hence short access tokens and a
  rotating refresh token. `installAuth` attaches the bearer, and on a 401 refreshes **once for all
  concurrent requests** (`singleFlight`) and replays the request once; a rejected refresh ends the session,
  a network error does not. The refresh itself runs under a Web Lock and re-reads the stored token, so two
  tabs never rotate the same token. Public endpoints pass `publicRequest` (`_skipAuth`). Sign-out and expiry
  call `resetAllStores()`: one user's data must never be visible to the next — add a `reset()` to any new
  store and call it there. Only same-site `next` paths are followed (`lib/safeNext.ts`).
- **Roles are UX, not security.** `lib/roles.ts` (`canEdit`, `canManage`) decides what is *offered*; the server
  decides what is *allowed*. Never hide a failure: a refused action shows the server's message.
- **Folder tree** (`lib/tree.ts`, `features/diagrams`): built client-side from the flat lists. A parent the
  server hides arrives as `""` and makes the node a top-level "Shared with me" root; cycles are broken, never
  looped. Moves and shares re-read both lists (access is inherited, so patching rows is not enough). Every
  drag-and-drop move has a keyboard alternative ("Move to…").
- **Several editors** (`lib/conflict.ts`, `diagramStore`, `features/editor/useRevisionPoll.ts`): a save that
  includes the DBML sends `base_revision`; a 409 `REVISION_CONFLICT` pauses autosave and asks. Only a *DBML*
  save may adopt the returned revision (a positions-only save could otherwise hide someone else's text
  change). The poll never replaces unsaved edits and is paused while a history preview is open.
- Dialogs rendered inside the sidebar are `inert` while it is collapsed; anything the toolbar can open
  (the share dialog) is mounted outside the sidebar (`ShareHost` via `AppShell`'s `overlays`).
- No `window.alert/confirm/prompt`; use `ConfirmDialog` / `PromptDialog`.
- Interactive things are real `<button>`/links with an accessible name.

## Access tokens (PATs) and MCP

- `features/auth/AccessTokensDialog.tsx` (lazy, from the user menu) lists/creates/revokes tokens and shows the MCP
  connection snippets; helpers live in `lib/tokens.ts`, state in `stores/tokenStore.ts`.
- The secret (`CreatedAccessToken.token`) exists **only** in `tokenStore.revealed`: it is never written to
  localStorage, a URL or a log, it is cleared on dismiss, when the dialog unmounts and on sign-out
  (`resetAllStores`). `AccessToken` (the listed form) has no secret field. Do not add one.
- A PAT can't manage tokens, so this UI only ever runs with a real login session.

## Commands

```sh
npm run dev         # vite
npm run typecheck   # tsc -b
npm run lint        # oxlint
npm test            # vitest (pure lib code)
npm run build       # tsc -b && vite build
```

Run `typecheck`, `lint`, `test` before finishing a change.
