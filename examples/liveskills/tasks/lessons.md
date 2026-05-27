# Lessons

- When a user asks to see published skills and an existing command already does it, document and fix that command instead of adding a new top-level command.
- When command vocabulary is confusing, separate verbs by user intent: discovery, installed list, registry publish, and local install should not share one overloaded command.
- For human CLI lists, avoid long table rows with freeform descriptions; use block output with intentional wrapping so terminal wrapping does not make the UI look broken.
- Default CLI help should be tiny and action-oriented; keep full references in docs, not in the first help screen.
- Match the visible `npx skills` CLI shape when requested: compact search rows with a selected marker for `find`, and display-name-first installed rows with a softened path and agent label for `list`.
- Do not auto-enter raw terminal mode from a command that should be useful in agent shells; make interactive UI explicit and handle EOF as cancel.
- When writing raw terminal UIs after `stty raw`, print CRLF instead of LF and read from `/dev/tty` for interactive input.
- When moving a standalone CLI into `examples/`, keep a repo-local Makefile with `make` and `make install` so users can build and put the binary on PATH without remembering raw `go build` commands.
- When the user asks LiveSkills help to follow AFS formatting, preserve command semantics and copy the AFS section shape: title, `Usage:`, `Options:`, then `Commands:`. Do not fall back to `npx skills` prompt-style rows.
- `liveskills list` and `liveskills list -g` must report installed skill folders found on disk, not only LiveSkills registry-managed installs. Use scan-style discovery as a fallback so regular downloaded skills remain visible.
- Installed list output should make LiveSkills-managed skills visibly distinct from local/unmanaged skill folders. Registry-backed installs are live; disk-discovered fallback rows are local unless the registry owns the same install path.
- Non-interactive `liveskills find` must not look like an interactive selector. Show copyable install refs such as `liveskills add owner/skill`, and make add/update output say which `list` command shows the installed skill.
- `liveskills list -g` is also the user's global inventory view, so include current-project LiveSkills-managed mounts there with `Scope: project`; otherwise live mounts look missing beside local global skills.
- Re-adding an already mounted LiveSkills skill must be idempotent. If AFS reports the same volume already mounted at the same path, treat it as already mounted rather than failing.
- When installed-list output has live/local subsections, render empty subsections explicitly with `None installed.` instead of leaving a header with no body.
- When `list -g` includes current-project live mounts, label that section as current-project live instead of a generic live/global section, and keep `.agents/skills` agent attribution aligned with detected universal agents from `npx skills`.
- When mirroring upstream `npx skills add`, include the security assessment, proceed confirmation, installed-path summary, and full-permissions warning, but do not add the upstream one-time `find-skills` upsell prompt unless the user explicitly asks for it.
