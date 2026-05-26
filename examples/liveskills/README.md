# LiveSkills

LiveSkills is an AFS-backed CLI for installing agent skills into the folders
used by Codex, Claude Code, and other local agents.

The product direction is intentionally simple:

```bash
liveskills add <source-or-ref>
liveskills list
liveskills add -g <source-or-ref>
liveskills list -g
```

`add` is the happy path. It should register or version a skill in AFS when
needed, then install it. Users should not need a separate `publish` step for
normal installs.

LiveSkills installs through the workspace-plus-symlink architecture: one
mounted LiveSkills workspace per scope, with agent folders linked to the
canonical skill copy by default.

## Quick Start

```bash
make
make install
liveskills help
go test ./...
liveskills add ./examples/react-best-practices
liveskills list
liveskills add -g ./examples/react-best-practices
liveskills list -g
make surface-test
```

LiveSkills uses the `afs` CLI when it is available. Set
`LIVESKILLS_AFS_MODE=local` to use the local development stand-in instead.

Registry data and the local AFS staging area live under `~/.liveskills` by default. Set `LIVESKILLS_HOME` to isolate a run:

```bash
LIVESKILLS_HOME=/tmp/liveskills-demo go run . find
```

## Install

Build the local binary:

```bash
make
```

Install it onto your command line:

```bash
make install
```

By default this installs `liveskills` to `~/.local/bin/liveskills`. Override the target with `BINDIR=/path/to/bin` or `PREFIX=/usr/local`:

```bash
make install PREFIX=/usr/local
```

## Full Surface Test

Run the isolated CLI harness when changing command behavior:

```bash
make surface-test
```

The harness builds a temporary `liveskills` binary, runs `go test ./...`, then
uses isolated `HOME`, `LIVESKILLS_HOME`, and `LIVESKILLS_AFS_MODE=local` values
to exercise the current Go command surface: auth, publish, find, show,
download, add, list, update, remove, scan, project/global installs,
agent-specific paths, validation errors, and deterministic stress coverage.

Use `--keep-temp` to inspect the generated project, home, store, and source
fixtures after a run:

```bash
python3 scripts/liveskills_surface_test.py --keep-temp --verbose
```

## Install Model

Project installs are the default. Global installs use `-g` / `--global`,
matching the `skills` CLI scope model.

LiveSkills should keep one hidden AFS-backed skills workspace per scope and put
registered skill content under `skills/<skill-slug>` inside that workspace.
Agent-facing skill folders then point at that canonical copy:

```text
.liveskills/mount/skills/<skill>      # project canonical source
.agents/skills/<skill>                # relative symlink by default
.claude/skills/<skill>                # relative symlink by default

~/.liveskills/mount/skills/<skill>    # global canonical source
~/.codex/skills/<skill>               # relative symlink by default
~/.claude/skills/<skill>              # relative symlink by default
```

LiveSkills owns one skill folder at a time, not the whole skills parent
directory. Neighboring manually downloaded skills under `.agents/skills`,
`.claude/skills`, or global agent folders must remain untouched.

Use `--copy` when symlinks are not wanted or not supported. Copy installs are
not live-linked to the AFS workspace and should be reported that way by `list`.

## Add Behavior

Target command shape:

```bash
liveskills add <source-or-ref>
liveskills add <source-or-ref> --agent codex --agent claude
liveskills add <source-or-ref> --skill react-best-practices --skill docs-review
liveskills add <source-or-ref> --copy
liveskills add -g <source-or-ref>
```

`<source-or-ref>` may be a local skill folder, a source containing multiple
skills, or a registry reference once registry lookup is implemented for that
shape.

Repeated `--agent` selects multiple target agents. Repeated `--skill` selects
multiple skills from a multi-skill source. Without explicit targets, `add`
should prompt for any ambiguous agent, skill, scope, or install-method choice.

The install flow should show:

- selected skills, agents, scope, and install method
- security risk assessment before making filesystem changes
- confirmation prompt unless a future noninteractive yes flag is used
- installed paths grouped by universal, symlinked, and copied targets
- final full-permissions warning for installed agent skills

Do not add the upstream `find-skills` upsell prompt.

## Commands

| Command | Description |
| --- | --- |
| `liveskills help` | Show available commands. |
| `liveskills add <source-or-ref>` | Register/version if needed, then install skills into the project. |
| `liveskills add -g <source-or-ref>` | Install skills into global agent folders. |
| `liveskills add <source-or-ref> --agent <agent>` | Install for one or more selected agents. |
| `liveskills add <source-or-ref> --skill <skill>` | Install one or more selected skills from a source. |
| `liveskills add <source-or-ref> --copy` | Copy instead of symlinking from the LiveSkills workspace. |
| `liveskills list [-g]` | Show skills installed on this computer. |
| `liveskills remove <skill>` | Remove a skill from this computer. |
| `liveskills update <skill>` | Update an installed skill. |
| `liveskills scan [-g|-p] [--agent codex]` | Scan local skill folders on this computer. |
| `liveskills find [query]` | Show published registry skills when registry discovery is needed. |
| `liveskills find --interactive [query]` | Search published registry skills interactively. |
| `liveskills publish <source> [--skill <name>]` | Advanced: publish/register without installing. |
| `liveskills show <owner>/<skill>` | Show registry details for a skill. |
| `liveskills download <owner>/<skill> --output <dir>` | Download a registry skill snapshot. |
| `liveskills add <source> --list` | Show skills available in a source. |
| `liveskills auth login` | Configure registry authentication. |

## AFS Boundary

`afs.go` contains the AFS adapter. In normal use it calls the `afs` CLI; in
`LIVESKILLS_AFS_MODE=local` it uses the local development stand-in.

Production behavior should preserve the same CLI contract while moving installs
to the workspace-plus-symlink model:

- one hidden skills workspace per scope
- skill content under `skills/<skill-slug>` in that workspace
- symlinked agent folders by default
- copy fallback with `--copy`
- no ownership of the whole agent skills parent directory
