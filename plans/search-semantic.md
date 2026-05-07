# Search Semantic Phase

Status: in progress
Owner: Codex
Created: 2026-05-07
Updated: 2026-05-07

## Goal

Make `afs query --semantic` run real vector-ranked retrieval when workspace
embeddings are enabled, while preserving keyword fallback behavior.

## Scope

- Add a deterministic embedding engine for query/chunk vectors.
- Persist chunk vectors on the existing query chunk HASH projection.
- Add Redis Search vector KNN over chunk HASHes with path filters.
- Route semantic-only requests through the vector path.
- Keep hybrid/default query stable and fallback-friendly.
- Update focused tests and docs.

## Checklist

- [ ] Inspect current query/index contracts.
- [ ] Implement embedding/vector indexing.
- [ ] Wire service and CLI semantic behavior.
- [ ] Add regression tests.
- [ ] Update docs.
- [ ] Run targeted validation.

## In Flight

Inspecting existing `query`, `queryindex`, config, and tests.

## Remaining

Implementation, docs, validation.

## Decisions / Blockers

- Use root `plans/`, not top-level `tasks/`, per current repo guidance.

## Verification

Pending.
