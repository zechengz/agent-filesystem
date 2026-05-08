# 0001 QMD-Inspired Workspace Query

Status: accepted
Date: 2026-05-07

## Context

AFS already has deterministic content search through `grep` and MCP
`file_grep`. That is the right interface when an agent knows the exact text,
glob, or regex to find. It is not the right interface for conceptual questions
such as "how do checkpoints work?" or QMD-style typed retrieval using lexical
and semantic clauses.

The candidate command surfaces were separate `search`, `vsearch`, and `query`
verbs, or one broader `query` command with narrower flags. Four public search
verbs made the CLI harder to explain.

## Decision

AFS exposes two public search verbs:

- `grep` for exact deterministic evidence.
- `query` for ranked retrieval.

`query` is the recommended hybrid surface. `query --keyword` selects
keyword-ranked retrieval and `query --semantic` selects vector-only semantic
retrieval. QMD-style typed documents use `lex:`, `vec:`, `hyde:`, and
`intent:` on the default `query` mode.

Embedding provider/model settings are global runtime settings, not workspace
settings. Operational vector-index commands live under `afs query index`.

MCP mirrors this split with `file_grep` and `file_query`.

## Consequences

Agents have one exact-search tool and one ranked-search tool, which keeps the
choice teachable. Plain `query` can fall back to keyword-ranked results while
hybrid vector/rerank is incomplete. Semantic embeddings are on by default; if
provider credentials are absent, semantic-only retrieval returns unavailable
status without a hard failure.

The semantic path uses the same Redis Search chunk documents with path filters.
It writes real provider embedding vectors onto chunk HASHes, uses Redis vector
KNN when available, and falls back to direct vector ranking when the Redis
vector backend is unavailable. The first real provider is OpenAI embeddings via
`OPENAI_API_KEY` in the control-plane environment and model names like
`openai:text-embedding-3-small`. QMD's local GGUF model path remains the target
local-first provider, but fake hash vectors must not be exposed as product
behavior. VectorSets are not the primary backend because AFS needs rich
metadata filters, lexical search, hybrid ranking, and result explanation in the
same retrieval model.

Keyword query uses the same projection pattern first: one derived HASH per text
chunk, indexed by RediSearch BM25 when available. Canonical file bytes remain
in the AFS content backend, including Redis Array storage when supported; the
projection is rebuildable and maintained asynchronously so file writes stay
fast.
