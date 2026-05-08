# Search Semantic Phase

Status: completed
Owner: Codex
Created: 2026-05-07
Updated: 2026-05-07

## Goal

Make `afs query --semantic` run real vector-ranked retrieval with globally
configured, default-on embeddings, while preserving keyword fallback behavior.

## Scope

- Add a real embedding provider for query/chunk vectors.
- Persist chunk vectors on the existing query chunk HASH projection.
- Add Redis Search vector KNN over chunk HASHes with path filters.
- Route semantic-only requests through the vector path.
- Keep hybrid/default query stable and fallback-friendly.
- Update focused tests and docs.

## Checklist

- [x] Inspect current query/index contracts.
- [x] Implement embedding/vector indexing.
- [x] Wire service and CLI semantic behavior.
- [x] Add regression tests.
- [x] Update docs.
- [x] Run targeted validation.
- [x] Move embedding model/status to global default-on behavior.
- [x] Remove UI/docs guidance that treats embeddings as per-workspace.
- [x] Re-run validation after global behavior update.

## In Flight

None.

## Remaining

None.

## Decisions / Blockers

- Use root `plans/`, not top-level `tasks/`, per current repo guidance.
- Scope vector RediSearch indexes and binary embedding HASH fields by
  provider/model/dimension so model changes cannot collide.
- Embedding provider/model are global environment/runtime settings, not
  workspace settings. Semantic embeddings are considered on by default; missing
  provider credentials should degrade to unavailable status without hard
  failure.

## Verification

- `go test ./internal/queryembedding ./internal/queryvector ./internal/queryindex`
- `go test ./internal/controlplane -run 'Query|WorkspaceConfig' -count=1`
- `go test ./cmd/afs -run 'Query|WorkspaceConfig' -count=1`
- `go test ./cmd/afs -count=1`
- `go test ./internal/controlplane -count=1`
- `cd ui && npm run build`
- `make commands`
- `make test`

## Review

Implemented semantic-only query with global, default-on embedding provider
resolution. Added an OpenAI embedding provider
(`openai:text-embedding-3-small` by default),
QMD-style query/document embedding formatting, chunk
embedding persistence, provider-scoped Redis vector KNN with direct
vector-ranking fallback,
CLI/service routing, tests, and docs. Local deterministic embeddings are test
only; product behavior requires a real provider. Follow-up update made
provider/model resolution global and default-on, removed public per-workspace
embedding controls, and reports missing provider credentials through query
index status without hard-failing semantic query commands. Removed stale
top-level workspace-scoped embedding status fields so provider/model appear
only under the global `embeddings` status object.
