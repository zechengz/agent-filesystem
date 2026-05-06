# QMD-Inspired `file_query`

Status: active
Owner: rowan / Codex
Created: 2026-05-06
Updated: 2026-05-06

## Goal

Add a QMD-inspired workspace query surface that lets humans and agents search
for ideas, concepts, and implementation intent across AFS workspaces.

`file_query` is intentionally separate from `file_grep`. Grep remains the
deterministic text-search tool for exact strings, regex, glob matching, counts,
and line-oriented matches. `file_query` is the ranked retrieval tool for
lexical search, local vector similarity, typed query documents, and hybrid
search.

The design is inspired by Tobi Lutke's QMD command split:

- `qmd search` -> lexical ranked search.
- `qmd vsearch` -> vector-only semantic search.
- `qmd query` -> recommended hybrid search.
- `qmd embed` -> generate or refresh local embeddings.
- QMD typed query documents: `lex:`, `vec:`, `hyde:`, and `intent:`.

## Scope

In scope:

- Add a new MCP `file_query` tool for local and hosted MCP.
- Add CLI commands:
  - `afs fs <workspace> search`
  - `afs fs <workspace> vsearch`
  - `afs fs <workspace> query`
  - `afs fs <workspace> embed`
- Use Redis Search / `FT.SEARCH` as the canonical search backend, with
  `FT.HYBRID` as an optimization where available.
- Store embeddings as chunk-level Redis HASH documents indexed by RediSearch
  vector fields.
- Use a local GGUF embedding model by default, matching QMD's local-first
  posture.
- Enqueue embedding work from control-plane materialization and sync/mount write
  paths without blocking file writes.
- Provide status, rebuild, stale-index, and explain/profiling surfaces.

Out of scope:

- Replacing or weakening `afs fs grep` / MCP `file_grep`.
- Redis VectorSets as the primary backend.
- Hosted cloud embeddings as the default.
- Full LLM reranking in the first production slice. Keep the interface ready
  for it, but ship hybrid retrieval first.
- UI search surfaces. This plan is CLI/MCP/backend-first.

## User Interface

### CLI

Keep existing deterministic grep unchanged:

```bash
afs fs repo grep "DirtyHint"
afs fs repo grep -E "error|warning" --path /internal
afs fs repo grep -l -i "disk full"
```

Add QMD-style ranked search commands:

```bash
afs fs repo search "workspace dirty marker"
afs fs repo vsearch "how does the UI know a workspace has unsaved changes?"
afs fs repo query "how does auth attach tenant scope to a workspace?"
```

Support typed query documents for the recommended hybrid command:

```bash
afs fs repo query $'lex: "dirty marker" workspace\nvec: how does the UI detect unsaved workspace changes?'
afs fs repo query $'intent: AFS live mount setup\nlex: "mount backend"\nvec: where does setup choose between NFS and FUSE?'
afs fs repo query $'hyde: The setup command stores a selected live mount backend in local configuration.'
```

Shared search options:

```bash
--path <workspace-path>
-n, --limit <num>
--all
--min-score <num>
--json
--files
--md
--full
--line-numbers
--explain
--candidate-limit <n>
--no-rerank
--intent <text>
--chunk-strategy <regex|auto>
```

Embedding commands:

```bash
afs fs repo embed
afs fs repo embed --wait
afs fs repo embed --force
afs fs repo embed --path /cmd/afs
afs fs repo embed --model embeddinggemma
afs fs repo embed --chunk-strategy auto
afs fs repo embed status
afs fs repo embed clean
```

### MCP

Keep `file_grep` exact and deterministic.

Add `file_query`:

```json
{
  "workspace": "repo",
  "path": "/",
  "query": "how does auth attach tenant scope?",
  "searches": [
    { "type": "lex", "query": "\"auth subject\" workspace" },
    { "type": "vec", "query": "how bearer tokens map to tenant scoped workspaces" }
  ],
  "intent": "AFS control-plane auth",
  "limit": 10,
  "min_score": 0.2,
  "rerank": "auto",
  "explain": false
}
```

Result shape:

```json
{
  "matches": [
    {
      "path": "/internal/controlplane/http.go",
      "start_line": 120,
      "end_line": 168,
      "score": 0.82,
      "source": "hybrid",
      "snippet": "...",
      "explain": {
        "lex_rank": 2,
        "vector_rank": 5,
        "rrf_score": 0.031
      }
    }
  ],
  "index": {
    "model": "embeddinggemma",
    "embedding_state": "ready",
    "pending_files": 0,
    "stale_files": 0
  }
}
```

Add a lightweight status tool only if it proves necessary after the first MCP
implementation. Prefer including enough index status in `file_query` errors and
responses first.

## Architecture

### Packages

Add focused shared packages so CLI, local MCP, and hosted MCP use the same
logic:

- `internal/querysearch`: public query orchestration, typed query parsing,
  result shaping, RRF fusion, explain traces.
- `internal/vectorindex`: Redis key builders, RediSearch vector index creation,
  chunk upserts/deletes, vector KNN search.
- `internal/embedding`: local embedding engine interface and model config.
- `internal/chunking`: text/code chunking with line spans and chunk previews.

Keep existing `internal/searchindex` for deterministic grep candidate indexing.
Add lexical ranked search support there only if it can be done without coupling
grep behavior to semantic query behavior.

### Redis Data Model

Do not store vectors on inode HASHes. Store one HASH per embedded chunk:

```text
afs:{fsKey}:vchunk:<modelDigest>:<inodeID>:<contentHash>:<seq>
  type=chunk
  path=/cmd/afs/afs_grep.go
  path_ancestors=/cmd,/cmd/afs
  inode_id=<inode>
  content_hash=<sha256>
  model=<model id>
  seq=<chunk seq>
  pos=<byte offset>
  start_line=<line>
  end_line=<line>
  preview=<short text>
  embedding=<float32 bytes>
```

Create one RediSearch vector index per workspace/model digest:

```text
FT.CREATE afs:vidx:{fsKey}:<modelDigest>:v1
ON HASH PREFIX 1 afs:{fsKey}:vchunk:<modelDigest>:
SCHEMA
  type TAG
  path TAG
  path_ancestors TAG SEPARATOR ","
  inode_id TAG
  content_hash TAG
  model TAG
  seq NUMERIC
  pos NUMERIC
  start_line NUMERIC
  end_line NUMERIC
  preview TEXT NOSTEM
  embedding VECTOR HNSW 6 TYPE FLOAT32 DIM <dim> DISTANCE_METRIC COSINE
```

Use the `{fsKey}` hash tag on all vector keys so Redis Cluster colocates a
workspace's vector data.

### Embedding Engine

Use a local model by default, matching QMD's approach:

- Default model: `embeddinggemma-300M-Q8_0.gguf`.
- Default cache directory: `~/.cache/afs/models`.
- Environment override:

```bash
AFS_EMBED_MODEL="hf:ggml-org/embeddinggemma-300M-GGUF/embeddinggemma-300M-Q8_0.gguf"
AFS_EMBED_PROVIDER=local
```

Initial interface:

```go
type Engine interface {
    Name() string
    ModelID() ModelID
    EmbedDocuments(ctx context.Context, chunks []ChunkForEmbedding) ([][]float32, error)
    EmbedQuery(ctx context.Context, query string) ([]float32, error)
}
```

Testing uses a deterministic fake embedder. If the local model is unavailable,
`search` still works, `query` falls back to lexical-only results with a clear
warning, and `vsearch` returns a structured "embeddings unavailable" error.

### Chunking

Start with a deterministic regex/line-aware chunker:

- Target around 900 tokens or a conservative byte approximation.
- 15% overlap.
- Preserve `start_line`, `end_line`, and byte offsets.
- Avoid splitting inside fenced code blocks where practical.
- Include title/path context in the embedding input, e.g. `path | heading |
  chunk`.

Add `--chunk-strategy auto` after the first slice to use AST-aware boundaries
for Go, TypeScript, JavaScript, Python, and Rust, inspired by QMD's tree-sitter
mode.

### Write Path

File writes and imports must not synchronously embed.

On control-plane materialization/import and mount/sync writes:

1. Write file content and existing search metadata.
2. Compute content hash.
3. Mark inode vector state as `pending` for the active model.
4. Enqueue a deduplicated embedding job.
5. Return to the caller.

Suggested queue keys:

```text
afs:{fsKey}:vector_pending:<modelDigest>
afs:{fsKey}:vector_events
afs:{fsKey}:vector_meta:<modelDigest>
```

The worker:

1. Claims pending inode/path.
2. Re-reads current inode metadata and content.
3. Skips binary files and files over the configured vector indexing cap.
4. Chunks content.
5. Embeds chunks in batches.
6. Deletes stale chunks for the inode/model.
7. Upserts chunk HASHes with vector bytes.
8. Marks the inode `vector_state=ready`, `skipped`, or `error`.

Renames can update chunk path metadata without re-embedding. Deletes should
delete chunks by inode id. Full workspace restore/root replacement can clear the
vector namespace and enqueue a rebuild.

### Query Flow

`search`:

1. Parse lexical query.
2. Query RediSearch text fields.
3. Return ranked chunk/file results with snippets.

`vsearch`:

1. Embed query locally.
2. Run RediSearch vector KNN with path filters.
3. Return ranked chunk results.

`query`:

1. Parse query document or single natural-language query.
2. If single query, use it as the original search with optional local expansion
   in a later phase.
3. Run lexical and vector retrieval in parallel.
4. Fuse results using reciprocal rank fusion, with extra weight for the first
   query clause and exact lexical hits.
5. Return top chunks/files with snippets and explain traces.
6. Later: add local reranking behind `--no-rerank` / `rerank:auto`.

Prefer `FT.HYBRID` when the deployed Redis supports it. Keep a manual fallback
using separate `FT.SEARCH` lexical and vector queries plus Go-side RRF.

## Checklist

### Phase 1 - Contracts and shared query core

- [ ] Add this plan to `plans/`.
- [ ] Define `file_query` MCP request/response structs in a shared package.
- [ ] Define CLI option structs for `search`, `vsearch`, `query`, and `embed`.
- [ ] Implement typed query parsing for `lex:`, `vec:`, `hyde:`, and `intent:`.
- [ ] Add unit tests for query parsing, invalid mixed query documents, balanced
      quote checks, and line constraints.
- [ ] Add result structs with stable JSON field names.

### Phase 2 - Redis vector index

- [ ] Add `internal/vectorindex` key builders.
- [ ] Add RediSearch vector index creation and capability detection.
- [ ] Add chunk HASH upsert/delete operations.
- [ ] Add vector KNN query with path/path ancestor filtering.
- [ ] Add manual lexical/vector RRF fusion helpers.
- [ ] Add tests using a fake Redis or integration Redis where RediSearch vector
      support is available.

### Phase 3 - Local embedding engine

- [ ] Add `internal/embedding` engine interface.
- [ ] Add deterministic fake embedder for tests.
- [ ] Add local GGUF model config and model identity/dimension handling.
- [ ] Add model cache path and environment overrides.
- [ ] Add clear errors for missing model/runtime.
- [ ] Add model-change detection that requires `embed --force`.

### Phase 4 - Chunking

- [ ] Add line-aware chunker with overlap and fenced-code handling.
- [ ] Include path/title context in embedding input.
- [ ] Add chunk preview extraction.
- [ ] Add chunk tests for Markdown, Go, empty files, large files, and binary
      detection.
- [ ] Add `--chunk-strategy auto` as a no-op alias or feature flag until
      AST-aware chunking lands.

### Phase 5 - Embedding worker and write hooks

- [ ] Add embedding pending queue and metadata keys.
- [ ] Hook control-plane workspace materialization/import to enqueue vector work.
- [ ] Hook mount/sync write paths to enqueue vector work after content changes.
- [ ] Add delete and rename handling.
- [ ] Add `afs fs <workspace> embed` to process pending work.
- [ ] Add `afs fs <workspace> embed --wait`, `--force`, `status`, and `clean`.
- [ ] Ensure writes never block on model download or embedding inference.

### Phase 6 - CLI search commands

- [ ] Add `afs fs <workspace> search`.
- [ ] Add `afs fs <workspace> vsearch`.
- [ ] Add `afs fs <workspace> query`.
- [ ] Support shared output options: `--json`, `--files`, `--md`, `--full`,
      `--line-numbers`, `--explain`, `--limit`, `--all`, and `--min-score`.
- [ ] Add profile timings similar to `AFS_GREP_PROFILE`.
- [ ] Add user-friendly fallback messages for missing/stale embeddings.

### Phase 7 - MCP tools

- [ ] Add local MCP `file_query`.
- [ ] Add hosted MCP `file_query`.
- [ ] Keep `file_grep` documentation pointed at deterministic text search.
- [ ] Update MCP tool descriptions so agents choose `file_query` for concepts
      and `file_grep` for exact occurrences.
- [ ] Add MCP tests for lexical, vector-unavailable fallback, typed queries,
      path scoping, and JSON result shape.

### Phase 8 - Hybrid and explain quality

- [ ] Add RRF explain traces.
- [ ] Weight the first typed search clause higher, matching QMD's pattern.
- [ ] Boost exact lexical hits when they rank highly.
- [ ] Add `--candidate-limit`.
- [ ] Add `--no-rerank` as a forward-compatible option.
- [ ] Evaluate whether local reranking is worth enabling by default after the
      retrieval-only version is stable.

### Phase 9 - Documentation and cleanup

- [ ] Update `README.md` with the new query command family.
- [ ] Update `docs/reference/control-plane-api.md` if hosted MCP/API contracts
      change.
- [ ] Update MCP setup docs with `file_query` examples.
- [ ] Add a short ADR under `docs/internals/decisions/` for QMD-inspired
      hybrid search, local embeddings, and RediSearch over VectorSets.
- [ ] Remove any stale entries from `plans/future-work.md` if this work lands.

## In Flight

- Plan created. Implementation not started.

## Decisions / Blockers

- **Separate tool boundary.** `file_query` is a new ranked retrieval surface.
  `file_grep` remains deterministic and line-oriented.
- **Separate CLI verbs.** Follow QMD with `search`, `vsearch`, `query`, and
  `embed` rather than one overloaded `search --mode ...` command.
- **Redis Search first.** Use chunk HASHes plus RediSearch vector fields.
  VectorSets are not the primary backend because AFS needs rich path filters,
  lexical search, hybrid ranking, and explainability.
- **Local-first embeddings.** Default to a local GGUF embedding model,
  conceptually matching QMD's `embeddinggemma-300M-Q8_0` setup. Cloud/provider
  embeddings can be added later behind explicit configuration.
- **Async indexing.** File writes enqueue embedding work and return. Search
  status must make staleness visible.
- **Reranking later.** Preserve `--no-rerank` / `rerank:auto` API space, but
  avoid making local reranking a first-slice dependency.
- **Redis capability detection.** Need runtime detection for RediSearch vector
  support and `FT.HYBRID`. Manual `FT.SEARCH` + Go RRF is the compatibility
  path.

## Verification

- [ ] `make commands`
- [ ] Targeted CLI tests under `./cmd/afs/...`
- [ ] Targeted control-plane tests under `./internal/controlplane/...`
- [ ] Targeted vector/query package unit tests.
- [ ] `cd mount && go test ./...` after write-hook changes.
- [ ] MCP local and hosted tool tests pass.
- [ ] Manual smoke:

```bash
afs fs getting-started embed --force --wait
afs fs getting-started search "workspace"
afs fs getting-started vsearch "how do I save a snapshot?"
afs fs getting-started query "how do checkpoints work?"
afs fs getting-started query $'lex: checkpoint\nvec: how do I save a snapshot?'
```

- [ ] Confirm `afs fs grep` and MCP `file_grep` behavior is unchanged.
- [ ] Confirm vector search degrades cleanly when embeddings or RediSearch
      vector support are unavailable.

## Result

Fill this in before archiving.
