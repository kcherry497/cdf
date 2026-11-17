---
name: CDF
description: >
  ClaudeDocFormat (.cdf) — a portable document format for operational notes,
  change logs, runbooks, status reports, and tabular content the user and Claude
  exchange. A .cdf file is pretty-printed JSON with a defined schema; the
  reader/editor is a single Bun script (`cdf`) that renders it in the terminal
  with bordered tables, dark theme, and keyboard navigation — the same one
  program on every platform. Cell edits open in $EDITOR (vim/nano/whatever).
  No browser anywhere. Optional HTML export exists for one-off sharing.
  USE WHEN: the user asks to "make a cdf", "save this as a doc", "put this in
  the format we use", or a response is table-shaped and worth preserving as
  a shareable file. NOT FOR: single answers, or artifacts the user wants hosted
  at a claude.ai URL (use the Artifact tool instead).
---

# CDF — ClaudeDocFormat

## What it is

A `.cdf` file is **pretty-printed JSON** matching a defined schema (see
`spec.md`). The **reader/editor** is a single Bun script at
`~/.claude/skills/CDF/Tools/cdf.ts`, meant to be run in a terminal or
compiled to a standalone binary via `bun build --compile`. There is no
browser dependency; rendering is ANSI + box-drawing characters — the same
primitives every terminal has spoken for decades.

## When Claude should reach for it

- The user asks to save a response as a portable, editable doc: notes,
  logs, runbooks, checklists, change registers, decisions.
- The user says "make a cdf", "same format as [prior CDF]", "keep this".
- Table-shaped content that shouldn't die in terminal scrollback.

Skip CDF for:
- One-line answers.
- Web pages hosted at a claude.ai URL (Artifact tool).
- Prose without tables (plain .md is fine).

## How Claude authors a CDF

The file is JSON — you can just Write it directly.

Minimum shape (see `spec.md` for full schema):

```json
{
  "meta": {
    "cdf_version": 1,
    "title": "My Document",
    "subtitle": "One-line description.",
    "updated": "2026-09-09"
  },
  "sections": [
    {
      "id": "unique-id",
      "name": "Section Name",
      "note": "Optional paragraph under the heading.",
      "rows": [
        {"date": "2026-09-09", "action": "Did a thing", "detail": "Long description..."}
      ]
    }
  ]
}
```

**Row visual state is auto-classified by tag** (never manually colored):

| Rule | Marker | Meaning |
|---|---|---|
| `action` starts with `MILESTONE:` | teal | key event |
| `action` starts with `EXPOSURE:` | red | security incident |
| `date` is `—` or `-` | amber | deferred |
| default | dim | ordinary |

## How Claude reads / edits an existing CDF

It's JSON. Read with the Read tool, mutate in-code, Write back. Or use
the `cdf` tool for scripted work:

```bash
bun ~/.claude/skills/CDF/Tools/cdf.ts new "Title"      # print skeleton to stdout
bun ~/.claude/skills/CDF/Tools/cdf.ts export file.cdf  # print HTML to stdout (for sharing)
bun ~/.claude/skills/CDF/Tools/cdf.ts view file.cdf    # open TUI viewer/editor
bun ~/.claude/skills/CDF/Tools/cdf.ts validate file.cdf # check schema, report errors
```

## How the human uses it

- **View/edit**: `cdf file.cdf` (once the binary is installed on PATH).
  - Arrow keys navigate. Enter opens the cell in `$EDITOR`. `a` add row,
    `d` delete row, `A` add section, `D` delete section. `s` save. `q` quit.
- **Share with someone who doesn't have `cdf`**: `cdf export file.cdf > file.html`
  and send the HTML. Read-only, dark, tables, no dependencies.
- **Diff a CDF in version control**: `.cdf` is JSON — diffs cleanly.
- **Pipe through jq**: `.cdf` is JSON — `jq '.sections[].name' file.cdf` works.

## Install (one-time)

```bash
# Run directly with Bun (no install)
bun ~/.claude/skills/CDF/Tools/cdf.ts view file.cdf

# Or compile a standalone binary and drop on PATH
bun build --compile ~/.claude/skills/CDF/Tools/cdf.ts --outfile ~/bin/cdf
cdf file.cdf   # now globally available
```

Bun cross-compiles to mac/linux/windows via `--target=bun-<platform>-x64`.

## Design invariants

- **Single implementation renders it.** One Bun script = one renderer.
  No browser variance, no policy churn.
- **Always dark, always bordered tables, system-terminal colors.**
- **Editing delegated to `$EDITOR`.** Don't reimplement text editors.
- **JSON on disk, never anything else.** Grep-able, diff-able, pipeable.
- **No runtime dependencies beyond Bun** (compile to a single binary and
  even Bun is bundled).
