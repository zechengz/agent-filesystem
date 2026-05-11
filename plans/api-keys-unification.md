# API Keys Unification

Status: implementation pass landed
Owner: rowan
Created: 2026-05-10
Updated: 2026-05-11

## Goal

Collapse three token surfaces (MCP volume, MCP control-plane, CLI mount) plus
upcoming workspace tokens into one central **API Keys** page. Rename "Token"
→ "API Key" across the UI. Per-object pages get read-only summary panels that
deep-link into the central page.

## Decisions

- Single central page; per-object summaries are read-only.
- Sidebar label: `MCP` → **`API Keys`** (icon: `KeyIcon`).
- Route: add `/api-keys`, keep `/mcp` as alias.
- "Token" → "API Key" in chrome. "Bearer" / "Token" stays in code snippets
  and in the wire-protocol `Authorization: Bearer …`.
- Capability ladder is **unified**: `Read`, `Read+Write`, `Read+Write+Checkpoints`,
  `Admin`. Type column (MCP / CLI / Workspace) disambiguates.
- Control-plane keys live in the same table — **yellow badge + confirm step**
  on create (preserves current safety gate).
- Inline onboarding flows (`/connect-cli`, onboarding drawer) **keep**
  creating keys inline. Resulting key also lands on `/api-keys`.
- Per-key detail pane hosts "How to use" with **MCP / CLI / SDK** snippet tabs
  (replaces the global `MCP_COMMANDS` drawer on the keys page).
- Per-key audit log (IP / useragent / lastUsedAt): nice-to-have, **not in
  scope** here — needs backend storage.

## Information architecture

### Sidebar

| Position | Today | New |
|---|---|---|
| 4 | MCP | **API Keys** |

Other items unchanged. `/mcp` → 301/alias to `/api-keys`.

### `/api-keys` page

```
[All] [Volume] [Workspace] [Control plane] [CLI]   [search…]   [+ Create API key]
```

Table columns: Name · Type · Scope · Capability · Created · Last used · Expires.

Row click → right-pane detail with metadata + "How to use" tabs (MCP / CLI /
SDK). Revoke lives in the detail pane.

### Create dialog (one flow, replaces `CreateMCPAccessDialog` +
`LocalMCPAccessDialog` + `WorkspaceTokensSection` form)

```
1. Access type:  ○ MCP (file tools)   ○ CLI mount   ○ Workspace (future)
2. Scope:        ○ Volume … selector    ○ Workspace … selector    ○ Control plane
                 (invalid combos disabled; e.g. CLI + Control plane n/a)
3. Capability:   Read / Read+Write / Read+Write+Checkpoints / Admin
                 (ladder gated by access+scope)
4. Name + Expiry
   [Control plane only → red confirm checkbox]
```

Pre-fill: deep-link from per-object summary sets Access + Scope; user picks
Capability + Name.

### Per-object summary panel

Appears on Volume Settings tab and Workspace Settings tab (replaces today's
`WorkspaceTokensSection` Tokens tab). Read-only. Shape:

```
API keys for this {volume|workspace}
N active · last used 12m ago
 • Codex laptop          MCP    Read+Write+Checkpoints
 • CI runner             CLI    Read+Write
[Create key for this {volume|workspace}]  [Open API Keys →]
```

Click row → `/api-keys?selected=<id>`.

## Component changes

- `ui/src/layout/navigation-items.ts` — relabel, change icon to `KeyIcon`,
  add `/api-keys` route.
- `ui/src/routes/mcp.tsx` → repurpose as `/api-keys` (or copy + alias) — page
  title, subtitle, button labels.
- `ui/src/foundation/tables/access-tokens-table.tsx`:
  - Add **Type** column (MCP / CLI / Workspace / Control plane).
  - Unify capability formatter — drop `mount-` prefix in display.
  - Filter buttons by Type and Scope.
  - Strings: "access token" → "API key".
- `ui/src/features/agents/CreateMCPAccessDialog.tsx` → rename to
  `CreateAPIKeyDialog`. Add Access-type step. Handle CLI branch
  (currently in `WorkspaceTokensSection`).
- `ui/src/features/agents/LocalMCPAccessDialog.tsx` — fold into main dialog
  as an "ephemeral / local" option, or delete if the merged flow covers it.
- `ui/src/features/agents/profiles/WorkspaceTokensSection.tsx` — delete; its
  creation logic moves into the unified dialog. Workspace profile **Tokens
  tab** in `AgentProfilePage.tsx` is replaced by the read-only summary panel
  on Settings tab.
- `ui/src/routes/workspace-studio/-settings-tab.tsx` — keep "Authorized
  tokens" section; tighten to the summary shape above; add **Create key for
  this volume** button that deep-links into central dialog.
- `ui/src/foundation/hooks/use-afs.ts` — expose a combined
  `useAllAPIKeys()` that merges `useAllMCPAccessTokens` +
  `useControlPlaneTokens` + `useCLIAccessTokens` for the unified list.
- `ui/src/foundation/types/afs.ts` — add a discriminated `APIKey` union
  (or a `kind: "mcp" | "cli" | "control-plane"` tag on a common shape) so
  the table can render rows uniformly.
- `ui/src/components/access-token-empty-state.tsx` — copy update; rename
  file.
- `ui/src/components/onboarding-drawer.tsx`, `mcp-connection-panel.tsx`,
  `connect-cli.tsx` — copy update; inline create flows still work but use
  unified dialog component.
- `MCP_COMMANDS` drawer on `/mcp` — moved into per-key detail "How to use"
  tabs.

## Phases

### Phase 1 — Foundation

- [x] Add `APIKey` union type + `useAllAPIKeys()` hook merging MCP +
      control-plane + CLI rows.
- [x] Unify capability formatter; new ladder labels (`Read`,
      `Read+Write`, `Read+Write+Checkpoints`, `Admin`).
- [x] Add Type column + scope filter chips to `APIKeysTable`.
- [x] Backend: `ListAllCLIAccessTokens` catalog + manager method,
      `RevokeCLIAccessToken`, `GET/DELETE /v1/cli-tokens(/:id)` routes
      (CLI rows were not previously listable).

### Phase 2 — Unified create dialog

- [x] `CreateAPIKeyDialog` with Access-type (MCP / CLI mount) + Scope
      (Volume / Control plane) steps. Old name kept as alias.
- [x] Control-plane confirm checkbox gating submit.
- [x] Pre-fill from per-object deep-link search params (workspaceId,
      databaseId).
- [ ] `LocalMCPAccessDialog` — kept as separate "Local stdio MCP" entry
      point for now; not folded.

### Phase 3 — Rename + nav

- [x] Sidebar label → "API Keys", icon → `KeyIcon`.
- [x] Route `/api-keys` added; `/mcp` still resolves to the same page.
- [x] Page title, subtitle, button copy, empty state copy: "token" →
      "API key" in user-facing surfaces.

### Phase 4 — Per-object summary panels

- [x] Workspace profile **Tokens tab** removed; Settings tab now renders
      `APIKeysSummaryPanel` with Create + Open shortcut.
- [x] Volume Settings "Agent access" section replaced with
      `APIKeysSummaryPanel`; deep-links to `/api-keys` pre-filtered.

### Phase 5 — Detail pane + snippet tabs

- [x] Per-key detail dialog metadata + Revoke.
- [x] "How to use" tabs (MCP / CLI / SDK) inside the detail dialog.

### Phase 6 — Cleanup + verify

- [x] String sweeps for "access token", "MCP token", "mount token"
      → "API key" in user-facing surfaces.
- [x] Removed `WorkspaceTokensSection.tsx` + its test.
- [x] `npm run build` clean.
- [x] `npm run lint` clean.
- [x] `npm test` — only pre-existing unrelated failure in
      `public-agent-documents.test.ts` (untouched).
- [x] `go build ./...` clean.
- [x] `go test ./internal/controlplane/...` passes (token tests).
- [x] Browser smoke: page renders at `/api-keys` and `/mcp`, sidebar shows
      "API Keys" with key icon, unified create dialog opens, Access type
      toggles disable Control plane for CLI, Control plane scope shows
      yellow confirm checkbox, submit disabled until acknowledged.

## Out of scope

- Backend prefix changes (`afs_mcp_*`, `afs_cli_*` etc. stay).
- Audit log fields (IP, useragent) — backend work.
- Workspace (composition) keys backend — picks up the same UI when the
  volume-workspace plan lands them.

## Verification

- [ ] Each key type creatable from central page.
- [ ] Each key type creatable from its object page (deep-link).
- [ ] Inline onboarding still mints a key and that key appears on
      `/api-keys`.
- [ ] Control-plane create requires confirm; badge shows yellow.
- [ ] Volume Settings + Workspace Settings panels list correct keys.
- [ ] Old `/mcp` URL still resolves.

## Open questions / follow-ups

- Keep `/mcp` as alias indefinitely or set a sunset (e.g. 60d)?
- `LocalMCPAccessDialog` — kept as a separate "Local stdio" surface.
  Still TBD whether to fold into the main dialog or remove if hosted MCP
  is the only real product path.
- Per-key audit log (last-used IP, useragent) — backend doesn't store
  these yet. Worth a follow-up plan if we want the detail dialog to
  surface them.
- Workspace composition tokens (per the volume-workspace plan) — the
  unified table will pick them up as a 4th `kind` once the backend mints
  them.

## Implementation Review — 2026-05-11

Landed the full unification:

- Backend: `GET /v1/cli-tokens` + `DELETE /v1/cli-tokens/:id` so the
  central table can list and revoke CLI keys (catalog +
  `DatabaseManager.ListAllCLIAccessTokens` + `RevokeCLIAccessToken`).
- Frontend types: `APIKey` discriminated union (`kind: "mcp" | "cli"`)
  in `foundation/types/afs.ts`; `useAllAPIKeys()` merges MCP +
  control-plane + CLI rows in one hook.
- `APIKeysTable` (export name) replaces the old `AccessTokensTable`
  shape. New `Type` column, scope filter chips
  (All / Volume MCP / Control plane / CLI mount), unified capability
  ladder labels. Detail dialog includes "How to use this key" tabs
  (MCP / CLI / SDK).
- `CreateAPIKeyDialog` collapses MCP + CLI creation behind one form
  with Access-type and Scope steps; control-plane scope requires
  explicit confirm.
- New `APIKeysSummaryPanel` rendered on Volume Settings tab and Agent
  Workspace Settings tab (replacing the old per-object Tokens tab and
  the embedded MCP token mini-table on the volume Settings panel).
- New nav entry: `API Keys` with key icon at `/api-keys`. `/mcp`
  resolves to the same page for back-compat.
- Old `WorkspaceTokensSection.tsx` and its test deleted.
- All build/lint/test runs green except the one pre-existing
  `public-agent-documents.test.ts` failure that pre-dates this change.
