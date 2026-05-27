# Scan Command

- [x] Pull upstream agent skill locations from `vercel-labs/skills`.
- [x] Add read-only scan inventory model.
- [x] Wire `liveskills scan` with JSON and text output.
- [x] Add CLI tests for project/global/all-agent scanning.
- [x] Run `gofmt -w *.go` and `go test ./...`.

## Review

Implemented read-only `liveskills scan` inventory over upstream agent paths plus standard project skill folders. Verified with unit tests and a temp project/global manual smoke.

# Scan Output Polish

- [x] Add top summary counts for scan text output.
- [x] Group summary by scope and skill parent path.
- [x] Add blank lines between listed skills.
- [x] Cover readable text output in tests.
- [x] Run `gofmt -w *.go` and `go test ./...`.

## Review

Added readable scan text output with scope totals, per-location summary lines, and blank lines between skill blocks. Verified with unit tests and a manual temp-project smoke.

# Published Skills Visibility

- [x] Keep `liveskills list --all` as the published registry view.
- [x] Document `list --all` in help/README.
- [x] Trace why `liveskills add` does not create a real AFS volume mount.
- [x] Fix install to use direct AFS volume mounts where available.
- [x] Cover the AFS mount boundary in tests.
- [x] Run `gofmt -w *.go` and `go test ./...`.

## Review

Documented `liveskills list --all` as the published registry view. Added CLI-backed AFS behavior so publishing imports the staged skill into a direct AFS volume, creates a checkpoint, and install restores/mounts that volume at the exact skill folder. Existing local-only registry entries now try to import their staged checkpoint into AFS on first real mount. Kept tests in `LIVESKILLS_AFS_MODE=local` and added command-runner tests for the real AFS command boundary and migration path. Verified `gofmt -w *.go`, `go test ./...`, a local-adapter smoke, and rebuilt `~/.local/bin/liveskills`. Real AFS smoke reaches `afs vol import`, but the current AFS token/config returns `Forbidden` for raw volume create/import, so real mounting is blocked until AFS auth has volume creation/import permission.

# Command Vocabulary Cleanup

- [x] Mirror Vercel `skills` shape: discovery separate from installed list.
- [x] Add `liveskills find [query]` for published inventory.
- [x] Keep `liveskills list [-g]` for installed skills only.
- [x] Keep `liveskills publish <source>` for adding to the registry.
- [x] Make `liveskills add <owner>/<skill>` install locally only.
- [x] Update help, README, tests, and lessons.
- [x] Run `gofmt -w *.go`, `go test ./...`, and smoke commands.

## Review

Matched the Vercel `skills` mental model from `skills find`, `skills list`, and `skills add`: LiveSkills now uses `help` for available CLI commands, `find` for available published skills, `list` for installed skills, `publish` for adding a source to the registry, and `add` for installing a published skill locally. `list --all` now points users to `find` instead of overloading installed listing, and `add <local-path>` points users to `publish` first. Verified with `gofmt -w *.go`, `go test ./...`, a temp-project local-mode smoke, and rebuilt `~/.local/bin/liveskills`.

# Command Reference Help Text

- [x] Remove the Advanced split from CLI help and docs.
- [x] Put descriptions next to each CLI command.
- [x] Include all commands in the README command reference.
- [x] Update help tests for the new format.
- [x] Run `gofmt -w *.go`, `go test ./...`, and rebuild.

## Review

CLI help now uses one command reference with descriptions beside every command. Removed the Advanced split from help and README, added every routed command to the README command table, including `auth login`, and locked the format with a help-output test. Verified with `gofmt -w *.go`, `go test ./...`, rebuilt `~/.local/bin/liveskills`, and checked `liveskills help`.

# Find Output Polish

- [x] Replace wide available-skill table rows with readable blocks.
- [x] Wrap long descriptions intentionally.
- [x] Add regression coverage for find text output.
- [x] Update lessons for CLI text wrapping polish.
- [x] Run `gofmt -w *.go`, `go test ./...`, and rebuild.

## Review

Changed `liveskills find` text output from a wide table row to a block per skill with version, wrapped description, and install command. Added a regression test that checks long find output stays within the CLI text width. Verified with `gofmt -w *.go`, `go test ./...`, rebuilt `~/.local/bin/liveskills`, and checked `liveskills find` against the current registry.

# Minimal Help Text

- [x] Replace verbose CLI help with Vercel-style minimal help.
- [x] Keep full command reference in README.
- [x] Update help-output test.
- [x] Update lessons for concise default help.
- [x] Run `gofmt -w *.go`, `go test ./...`, and rebuild.

## Review

Replaced the verbose CLI help with a Vercel-style five-command screen, kept the full command reference in README, and locked the exact output with a regression test. Verified with `gofmt -w *.go`, `go test ./...`, rebuilt `~/.local/bin/liveskills`, and checked `liveskills help`.

# Find List Style

- [x] Mirror Vercel `skills find` compact result list shape.
- [x] Replace block find rows with selected-list rows.
- [x] Keep output honest about LiveSkills install command.
- [x] Update `list` to use display-name-first rows like the screenshot.
- [x] Add interactive find prompt/rendering support.
- [x] Update regression tests and lessons.
- [x] Run `gofmt -w *.go`, `go test ./...`, and rebuild.

## Review

Checked Vercel `skills` `src/find.ts`: interactive search renders `Search skills:`, a selected `>` row, up/down/enter/esc hint, and installs on selection. Updated LiveSkills `find` to use compact selected-list rows, added an interactive raw-terminal prompt for `liveskills find`, and kept queried/non-TTY output in the same visual shape. Updated `list` to match the screenshot style: display name first, home-relative full path, and cleaner agent line. Verified with `gofmt -w *.go`, `go test ./...`, rebuilt `~/.local/bin/liveskills`, and smoke-checked `liveskills find` and `liveskills list`.

# Find EOF Fix

- [x] Stop auto-entering raw interactive mode for plain `liveskills find`.
- [x] Keep interactive find available explicitly with `--interactive`.
- [x] Handle stdin EOF as a clean cancel inside interactive find.
- [x] Document explicit interactive find in README.
- [x] Run `gofmt -w *.go`, `go test ./...`, rebuild, and smoke test TTY/non-TTY.

## Review

Fixed the `EOF` failure by making plain `liveskills find` print results instead of auto-entering raw interactive mode. Interactive search remains available with `liveskills find --interactive`; EOF in that path now exits as a clean cancellation rather than an error. Documented the explicit interactive form, added a non-terminal guard test, verified `gofmt -w *.go`, `go test ./...`, rebuilt `~/.local/bin/liveskills`, and smoke-tested plain `find` in both pipe and PTY plus `find --interactive` cancellation in a PTY.

# Interactive Find Terminal Fix

- [x] Remove navigation hint from non-interactive `find`.
- [x] Open `/dev/tty` for interactive input instead of trusting stdin.
- [x] Use CRLF while raw mode is active so prompt lines do not stair-step.
- [x] Probe interactive input before painting the raw prompt and fall back cleanly.
- [x] Run `gofmt -w *.go`, `go test ./...`, rebuild, and smoke test.

## Review

Fixed the broken `find --interactive` terminal behavior by reading interactive input from `/dev/tty`, using CRLF while raw mode is active, and probing input before painting the raw prompt. If the host terminal cannot deliver interactive input, `find --interactive` now falls back to the normal compact result list instead of stair-stepping and printing `Search cancelled`. Plain `find` no longer shows navigation hints. Verified with `gofmt -w *.go`, `go test ./...`, rebuilt `~/.local/bin/liveskills`, and smoke-tested plain and PTY `find --interactive` fallback.

# Move Into Agent Filesystem Examples

- [x] Move the LiveSkills project under `~/git/agent-filesystem/examples/liveskills`.
- [x] Keep generated local runtime folders out of the example package.
- [x] Add local ignore rules for future LiveSkills runtime artifacts.
- [x] Run `go test ./...` from the new path.
- [x] Rebuild `~/.local/bin/liveskills` from the new path.
- [x] Confirm the original project path is gone.

## Review

Moved LiveSkills into `~/git/agent-filesystem/examples/liveskills`, removed local generated install/mount folders from the example package, added a local `.gitignore` so future `.agents`, `.liveskills`, and `demo-workspace` artifacts stay out of git, and updated `AGENTS.md` smoke paths to the new location. Verified the old path is gone, `go test ./...` passes from the new directory, `gofmt -w *.go` is clean, rebuilt `~/.local/bin/liveskills`, smoke-checked `liveskills help`, and confirmed no references to the old `~/git/liveskills` path remain.

# Make Install Surface

- [x] Add repo-local Makefile.
- [x] Make default `make` build a `liveskills` binary.
- [x] Add `make install` for command-line use.
- [x] Document the install path in README.
- [x] Run `make`, `make install`, and `go test ./...`.

## Review

Added a repo-local Makefile with `make`, `make install`, `make test`, `make fmt`, `make clean`, `make uninstall`, and `make help`. The default build writes `bin/liveskills`, and `make install` installs to `~/.local/bin/liveskills` unless `BINDIR` or `PREFIX` is overridden. Documented the flow in README, ignored `bin/`, verified `make`, `make install`, `make test`, `make help`, `command -v liveskills`, `liveskills help`, and `go test ./...`.

# Help Formatting Cleanup

- [x] Compare `liveskills help` against `~/git/agent-filesystem/afs help`.
- [x] Keep the same LiveSkills command rows.
- [x] Reformat help with AFS-style `Usage`, `Options`, and `Commands` sections.
- [x] Run `gofmt -w *.go`, `go test ./...`, rebuild, and smoke-check `liveskills help`.

## Review

Replaced the `npx skills`-style prompt rows with AFS-style help sections while preserving the same LiveSkills command rows. Verified with `gofmt -w *.go`, `go test ./...`, focused `TestHelpUsesAFSStyleFormatting`, `make install`, `go run . help`, and installed `liveskills help`.

# Global List Discovery Fix

- [x] Reproduce `liveskills list -g` returning empty while global skill folders exist.
- [x] Trace `list -g` to registry-only installs.
- [x] Reuse local skill discovery for `list -g` output.
- [x] Preserve LiveSkills-managed rows and avoid duplicate linked skills.
- [x] Run `gofmt -w *.go`, `go test ./...`, rebuild, and smoke-check `liveskills list -g`.

## Review

Fixed `liveskills list -g` to include installed skill folders discovered on disk, while preserving registry-managed LiveSkills rows and collapsing duplicate linked skills by skill name. The installed binary now matches `npx skills list -g` for the current global skills: `afs-shared-memory`, `afs-team-board`, `shared-memory`, `imagegen`, `screenshot`, and `vercel-deploy`. Verified with `gofmt -w *.go`, focused global-list tests, `go test ./...`, `make install`, `go run . list -g`, installed `liveskills list -g`, and `npx skills list -g`.

# Live vs Local List Distinction

- [x] Tag registry-backed installs as live and LiveSkills-managed.
- [x] Keep disk-discovered skills as local and unmanaged.
- [x] Split installed list output into live and local sections.
- [x] Include live metadata in JSON output.
- [x] Run `gofmt -w *.go`, focused list tests, `go test ./...`, rebuild, and smoke-check `liveskills list -g`.

## Review

Installed list rows now distinguish LiveSkills-managed installs from unmanaged folders. Registry-backed installs carry `live: true`, `managed: true`, `managedBy: "LiveSkills"`, version, volume, and checkpoint metadata in JSON; text output groups rows under `Live Skills (managed by LiveSkills)` and `Local Skills (not managed by LiveSkills)`. Verified with `gofmt -w *.go`, focused list tests, `go test ./...`, `make install`, current-home `go run . list -g`, current-home JSON output, and an isolated temp-home smoke with one live global skill plus one local-only global skill.

# Find And Scope Clarity

- [x] Reproduce non-interactive `find` rendering as a selection list.
- [x] Confirm `liveskills add local/screenshot` installed a project live skill.
- [x] Change non-interactive `find` to show copyable install refs.
- [x] Add install result scope and matching list command.
- [x] Run `gofmt -w *.go`, focused tests, `go test ./...`, rebuild, and smoke-check `find`, `add`, `list`, and `list -g`.

## Review

Changed non-interactive `find` from selector-style rows to plain result blocks with `Ref`, `Version`, and a copyable `Install` command. `add` and `update` results now include the install `Scope` plus the exact `List with` command, so project installs point to `liveskills list` and global installs point to `liveskills list -g`. Verified with `gofmt -w *.go`, focused find/list/add tests, `go test ./...`, `make install`, current-home `liveskills find`, `liveskills list`, `liveskills list -g`, and isolated temp-home project/global install smokes.

# List Global Live Mount Visibility

- [x] Reproduce `list -g` hiding current-project LiveSkills mounts.
- [x] Include current-project live mounts in `list -g`.
- [x] Show `Scope` on live rows so project/global is explicit.
- [x] Treat same-volume same-path AFS mount overlap as already mounted.
- [x] Run `gofmt -w *.go`, focused tests, `go test ./...`, rebuild, and smoke-check repeat add plus `list -g`.

## Review

`liveskills list -g` now includes current-project LiveSkills-managed mounts above local global skills, with `Scope: project` so the scope remains honest. Re-adding an already mounted skill now returns `Skill already added`; if AFS reports the same volume already mounted at the same path, LiveSkills treats that as success instead of surfacing the overlap error. Verified with `gofmt -w *.go`, focused global-list/idempotency tests, `go test ./...`, `go test ./... -count=1`, `make install`, installed `liveskills list -g`, and installed `liveskills add local/screenshot`.

# Empty Installed Sections

- [x] Reproduce installed-list headers feeling empty after live/local split.
- [x] Show `None installed.` for empty live/local subsections.
- [x] Run `gofmt -w *.go`, focused list tests, `go test ./...`, rebuild, and smoke-check `list` plus `list -g`.

## Review

Installed list subsections now always render a body. If either `Live Skills (managed by LiveSkills)` or `Local Skills (not managed by LiveSkills)` is empty, the section prints `None installed.` instead of leaving a bare header. Verified with `gofmt -w *.go`, focused list tests, `go test ./...`, `go test ./... -count=1`, `make install`, installed `liveskills list`, and installed `liveskills list -g`.

# List Consistency Check

- [x] Compare `liveskills list`/`list -g` against `npx skills list`/`list -g`.
- [x] Make `list -g` labels explicit when it includes current-project live skills.
- [x] Align project `.agents/skills` live rows with detected universal agents.
- [x] Run `gofmt -w *.go`, focused tests, `go test ./...`, rebuild, and smoke-check the four list commands.

## Review

`list -g` now labels project-scoped live mounts as `Current Project Live Skills (managed by LiveSkills)` before the global-live and local sections, so the project screenshot mount is visible without pretending it is global. Live project rows under `.agents/skills` now use detected universal agents, matching `npx skills list` for this machine: `Antigravity, Codex, Cursor, Gemini CLI`. Verified with `gofmt -w *.go`, focused list tests, `go test ./... -count=1`, `make install`, installed `liveskills list`, installed `liveskills list -g`, JSON list outputs, repeat `liveskills add local/screenshot`, and `npx skills list` / `npx skills list -g`.

# Workspace Symlink Architecture Design

- [x] Review upstream `vercel-labs/skills` source for agents, scopes, symlink/copy install, find/add prompts, list/update/remove, and locks.
- [x] Review `vercel-labs/agent-skills` as the skill collection shape exposed through `skills.sh`.
- [x] Review local LiveSkills installer, registry, scan, and AFS adapter seams.
- [x] Spawn a subagent for an independent read-only review.
- [x] Capture the proposed workspace-plus-symlink design and implementation plan.

## Review

Upstream `npx skills` uses broad agent metadata, repeated `--agent` and `--skill`, project/global scopes, a canonical skill location, and symlinks from agent-specific directories. Its interactive `find` calls `add`, then shows installation summary, security risk assessment, confirmation, installed-path grouping, and the full-permissions warning. LiveSkills should mirror that flow but skip the one-time `find-skills` upsell prompt.

Current LiveSkills has the scanner data for many agent directories, but install still supports one scalar `--agent`, defaults to Codex, refuses `add <source>` until `publish` is run, and mounts each skill as a direct AFS volume at the final agent skill folder.

Recommended architecture: keep one hidden LiveSkills AFS workspace mount per scope, place each skill at `skills/<slug>` inside that workspace, and link agent directories to that mounted source. Project example: `.liveskills/mount/skills/<slug>` as the mounted source, `.agents/skills/<slug>` and `.claude/skills/<slug>` as relative symlinks. Global example: `~/.liveskills/mount/skills/<slug>` as the mounted source, with `~/.codex/skills/<slug>` and `~/.claude/skills/<slug>` linking to it.

Implementation plan:

- Replace the install-time agent switch with a shared agent registry based on `scanAgents`, including universal `.agents/skills` agents and per-agent global paths.
- Extend argument parsing to preserve repeated `-a/--agent` and `-s/--skill`, plus wildcard support and `--copy`, `--all`, and `-y`.
- Make `add <source>` register/version the skill in AFS when needed, then install in one command; keep `publish`, `show`, and `download` as advanced/debug/export workflows.
- Add AFS adapter methods for `EnsureSkillsWorkspace`, `AttachSkillVolume`, `MountSkillsWorkspace`, and `DetachSkillVolumeIfUnused`, while keeping local-mode metadata equivalent.
- Change persisted installs from scalar `Agent` to `Targets[]` with `agent`, `linkPath`, `mode`, `status`, and ownership metadata. Keep migration from the current schema.
- Add safe symlink helpers: relative links, parent creation, existing-link validation, broken-link repair when LiveSkills owns it, and refusal to overwrite unmanaged directories.
- Keep `--copy` as an explicit fallback path that copies from the mounted workspace into each selected agent directory and marks the target as non-live.
- Update `list`, `update`, and `remove` around targets rather than scalar installs. Removing one target should keep the workspace skill mounted for other agents; removing the final target should detach the skill volume.
- Add prompt/output flow: agent multiselect, scope prompt, install method prompt, installation summary, security risk assessment, proceed confirmation, completion summary grouped as `universal`, `symlinked`, and `copied`, final full-permissions warning, and no `find-skills` prompt.

Test plan:

- Multi-agent project install creates one workspace mount and multiple symlinks.
- Adding a second skill reuses the same workspace mount.
- Global Codex plus Claude install links into both global dirs.
- `--copy` creates independent copies and marks targets non-live.
- Existing unmanaged skill directories are refused, not overwritten.
- Removing one agent link keeps the skill available to remaining agents.
- Removing the final target detaches the skill volume.
- `list` reports linked agents, broken links, copied targets, and missing workspace mounts.
- `add <local-source>` registers and installs in one command.
- Neighboring manual `.agents/skills/*` folders remain untouched.

# Workspace Symlink Architecture Implementation

- [x] Add shared agent metadata and multi-value CLI parsing.
- [x] Add safe symlink/copy target helpers.
- [x] Add AFS workspace adapter methods for one mounted skills workspace per scope.
- [x] Migrate install registry/manifest model to target-based installs.
- [x] Make `add <source-or-ref>` publish/register and install in one step.
- [x] Implement multi-agent project/global install with symlink default and `--copy`.
- [x] Update list/update/remove for target-based installs and broken-link state.
- [x] Add upstream-style install summary/security/confirmation output without the `find-skills` upsell.
- [x] Update README and regression tests.
- [x] Run `gofmt -w *.go`, `go test ./...`, `make`, manual project/global smoke, and `make surface-test`.

## Review

Implementation landed across the Go CLI. LiveSkills now resolves repeated
`--agent`/`--skill`, centralizes upstream-style agent metadata, installs source
folders directly by publishing/registering when needed, mounts one hidden skills
workspace per scope, attaches each skill under `skills/<slug>`, and symlinks or
copies selected agent folders from that canonical source. Registry and
workspace manifests now track targets instead of a single scalar agent, with
migration compatibility for older install records.

`list`, `update`, and `remove` now operate on target records. Removing one agent
target leaves other targets and the canonical workspace skill intact; removing
the final target detaches the skill volume. The add output includes a local
security assessment and the full-permissions warning, without the upstream
`find-skills` upsell prompt.

Verified with `gofmt -w *.go`, `go test ./...`, `make`, `make surface-test`,
and an isolated manual smoke using a temporary binary, `HOME`, and
`LIVESKILLS_HOME`. The smoke covered empty project list, project source add with
Codex plus Claude Code symlinks, project list, converting a target to `--copy`,
global add/list, removing one agent target, and confirming the remaining target
still works. The surface harness completed 39 checks across 9 sections and 113
subprocess commands against the workspace-plus-symlink contract.

# Workspace Symlink Documentation

- [x] Update README around target `add <source-or-ref>` workflow.
- [x] Document repeated `--agent`, repeated `--skill`, and `--copy`.
- [x] Document project/global scope behavior.
- [x] Document workspace-plus-symlink model.
- [x] Document security prompt flow and no `find-skills` upsell.
- [x] Reconcile README command details with Go help once implementation lands.
- [x] Add/update regression tests after code shape is final.

## Review

Updated README as a current-state-honest target contract for the
workspace-plus-symlink implementation. The docs now match the implemented
`add <source-or-ref>` flow, repeated agent/skill selectors, project/global
scopes, `--copy`, security confirmation, and the deliberate omission of the
upstream `find-skills` upsell. Verified with the implementation test sweep
above.

# Full Surface Test Script

- [x] Build an isolated CLI test harness with clear UX and failure logs.
- [x] Cover help/auth/publish/find/show/download/add/list/update/remove/scan.
- [x] Cover project, global, agent-specific, JSON, source-list, and error paths.
- [x] Add deterministic stress coverage for many published/installed skills.
- [x] Wire the harness into Makefile/README.
- [x] Run `gofmt -w *.go`, `go test ./...`, and the new harness.

## Open Questions

- None.

## Review

Added `scripts/liveskills_surface_test.py`, a Python stdlib CLI harness that
builds an isolated binary, runs `go test ./...`, and exercises the current
LiveSkills command surface with isolated `HOME`, `LIVESKILLS_HOME`, and local
AFS mode. It covers auth token persistence, publish/find/show/download,
project/global add/list/update/remove, `ls`/`rm` aliases, agent-specific paths,
source discovery, validation failures, scan filters, and a deterministic stress
run. Wired it into `make surface-test` and documented the harness in README.
Verified with `make surface-test`, which completed 39 checks across 9 sections
and 113 subprocess commands.
