# dbdiagram — frontend

DBML editor with a live ER diagram. React 19, TypeScript, Vite, zustand,
React Flow, Monaco. Talks to the Go backend in `../backend` (see
`../docs/API-CONTRACT.md`).

```sh
cp .env.example .env     # VITE_API_BASE_URL, default http://localhost:8080
npm install
npm run dev
```

Scripts: `dev`, `build`, `typecheck`, `lint` (oxlint), `test` (vitest), `preview`.

## Layout

```
src/
  app/        router, App, route errors
  config/     env.ts
  api/        axios client, ApiError, list query
  types/      api.ts (wire types)
  services/   auth.service.ts, diagrams/{diagrams,folders,members,versions,shared}.service.ts
  stores/     authStore, diagramStore (+ save queue, conflicts), folderStore, historyStore, uiStore, toastStore
  features/   auth/ (login, register, account), editor/ (page, Monaco, canvas, conflict, poll), diagrams/ (folder tree), sharing/ (share dialog), history/, shared/ (public view)
  components/ ui/ (Button, Dialog, Menu, ...), ErrorBoundary
  layout/     AppShell, Sidebar, Topbar
  hooks/      useAsync, useParsedDbml
  lib/        dbml, layout, saveQueue, tree, roles, conflict, safeNext, singleFlight, export, relativeTime, debounce, cn
  styles/     tokens + per-area CSS
```

Routes: `/login`, `/register`; `/` and `/d/:key` (editor, needs sign-in); `/s/:key` (public read-only view, no sign-in).

Conventions and layer rules: see `AGENTS.md`.

## History

The **History** button (next to Share) opens a drawer of snapshots, newest first. Click one to preview it
read only (banner: *Restore this version* / *Back to current*), **Save version** takes a labelled snapshot,
× deletes one, Esc closes the drawer and leaves the preview. Restoring keeps the current content as a new
version server-side. Not shown on the public share page.

## Accounts, folders and sharing

- **Sign in / register** (`/login`, `/register`). `?next=` returns you to the page you wanted, but only
  same-site paths are honoured (`lib/safeNext.ts`). The user menu at the bottom of the sidebar has
  **Account** (name, password) and **Log out**.
- **Token storage.** The access token lives in memory only and is short-lived. The refresh token is in
  `localStorage` (`auth.refresh`) so a reload keeps you signed in — which means any script running on this
  origin (an XSS bug) could read it. That is the trade-off: the refresh token is **rotated on every use**
  (a replayed one ends the session server-side), and refreshing is single-flight and cross-tab locked.
- **Folder tree.** *My diagrams* and *Shared with me*, nested folders, role badges (`viewer`/`editor`)
  on what others own. Row menu (`⋯`): new diagram/subfolder, rename, **Share…**, **Move to…**, delete — what
  is offered depends on your role. Drag a row onto a folder to move it; a refused move shows the server's reason.
  A folder can only be deleted when empty.
- **Share dialog.** Add people by email as *Can edit* / *Can view*; access to a folder covers everything
  inside it. A diagram's public read-only link is managed in the same dialog. Non-owners can only **Leave**.
- **Viewer** = read-only editor, no history writes, no import. This is UI only; the server enforces roles.
- **Several editors.** Saves carry the revision you loaded; if someone else changed the DBML first you get a
  dialog (*Load their version* / *Keep editing* / *Overwrite*). While nothing is unsaved the editor polls every
  5 s (visible tab only) and swaps in other people's changes with a "Updated by …" toast.
- **Access tokens & MCP.** *User menu → Access tokens & MCP…*: create read or read-and-write tokens (expiry
  7–365 days, default 90), see when each was last used, revoke them. A new token's secret is shown **once**
  (copy it before hiding it; it is kept only in memory and dropped on hide or sign-out). The same dialog gives
  the MCP URL and ready-to-paste Claude Code / JSON snippets. Treat tokens like passwords: they act as you
  within their scope, and unlike a login they are not ended by a password change — revoke them here.
