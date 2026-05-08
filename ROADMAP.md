# linear-tui Roadmap

Fork of [roeyazroel/linear-tui](https://github.com/roeyazroel/linear-tui) — extended for daily operator use.

---

## Phase 1 — Core Workflow Done ✓

> Goal: turn the TUI from a viewer into a worker. Every action that currently requires leaving the terminal gets a keybinding.

| # | Feature | Key | Status |
|---|---------|-----|--------|
| 1.1 | **Quick status change** | `s` | Done ✓ |
| 1.2 | **Git branch checkout** | `g` | Done ✓ |
| 1.3 | **Cycle burndown chart** | auto in cycle view | Done ✓ |
| 1.4 | Linear Cycles support | sidebar | Done ✓ |

**Keybinding additions:**
- `s` — change issue status inline (picker from workflow states)
- `g` — `git checkout -b <branchName>` in configured workspace

**Burndown chart format (cycle detail view):**
```
Cycle #1  May 11→25  ░░░▂▃▄▅▆  12/28 done  42%
Alex: 3 done  2 in-progress  |  Jordan: 1 done  4 in-progress
```

---

## Phase 2 — Sprint Planning Done ✓

> Goal: plan and manage sprints entirely from the TUI. Zero web app required for day-to-day sprint ops.

| # | Feature | Key/Trigger | Status |
|---|---------|-------------|--------|
| 2.1 | **Priority change** | `p` | Done ✓ |
| 2.2 | **Assignee change** | `a` | Done ✓ |
| 2.3 | **Bulk issue select** | `Space` to toggle | Done ✓ |
| 2.4 | **Bulk add to cycle** | `Space` select → `: add_to_cycle` | Done ✓ |
| 2.5 | **Bulk status change** | `Space` select → `s` | Done ✓ |
| 2.6 | **Project health panel** | auto in project view | Done ✓ |
| 2.7 | **Due date display** | column in issues table | Done ✓ |
| 2.8 | **Estimate display** | column in issues table | Done ✓ |

---

## Phase 3 — Navigation & Discovery Done ✓

> Goal: find anything instantly. The TUI becomes the fastest way to get to any issue, not just issues you already know about.

| # | Feature | Key/Trigger | Status |
|---|---------|-------------|--------|
| 3.1 | **Fuzzy issue finder** | `Ctrl+P` | Done ✓ |
| 3.2 | **Saved filter views** | named, accessible from sidebar | Done ✓ |
| 3.3 | **Assignee filter toggle** | `Shift+A` | Done ✓ |
| 3.4 | **Label filter** | `Shift+L` | Done ✓ |
| 3.5 | **Open in browser** | `o` | Done ✓ |
| 3.6 | **Copy URL** | `y` | Done ✓ |
| 3.7 | **Copy branch name** | `Shift+Y` | Done ✓ |
| 3.8 | **Issue linking** | `: create_relation` | Done ✓ |
| 3.9 | **Mention/notification view** | sidebar section | Done ✓ |
| 3.10 | **Triage mode** | `: triage` | Done ✓ |

**Fuzzy finder UX:**
- `Ctrl+P` opens full-screen overlay
- Type to filter across title + identifier across all cached issues
- `↑↓/j/k` navigate results, `Enter` jumps to issue in context
- Dismiss with `Escape`

**Saved views:**
- `: save_view` — prompts for name, saves current filters
- `: delete_view` — picker to remove a view
- Views appear in sidebar under "Saved Views" group

**Triage mode:**
- `: triage` opens rapid-fire backlog grooming overlay
- `s`/`p`/`a`/`e` — change status/priority/assignee/estimate, then auto-advance
- `→` or `n` — skip to next issue
- `q` or `Esc` — exit triage

---

## Phase 4 — Analytics & Reporting Done ✓

> Goal: sprint reviews and team health visible without leaving the terminal.

| # | Feature | Key/Trigger | Status |
|---|---------|-------------|--------|
| 4.1 | **Velocity chart** | `: velocity` | Done ✓ |
| 4.2 | **Per-assignee breakdown** | in cycle detail (below burndown) | Done ✓ |
| 4.3 | **Team throughput summary** | `: stats` | Done ✓ |
| 4.4 | **Export cycle report** | `: export_cycle` | Done ✓ |

**Velocity chart format:**
```
─── Velocity — Raava Solutions ────────────────
Cycle #1  May 11–25  ████████░░░░░░░░  8 done
Cycle #2  May 25–Jun 8  ██████░░░░░░░░░░  6 done  ← current
──────────────────────────────────────────────
Avg: 7 / cycle    Best: 8    Trend: ↓
```

---

## Keybinding Reference

| Key | Action |
|-----|--------|
| `o` | Open focused issue in browser |
| `y` | Copy issue URL to clipboard |
| `Shift+Y` | Copy git branch name to clipboard |
| `Ctrl+P` | Open fuzzy issue finder |
| `Shift+A` | Toggle "my issues" filter |
| `Shift+L` | Label filter picker |
| `s` | Change issue status |
| `p` | Change issue priority |
| `a` | Assign issue to user |
| `e` | Set story point estimate |
| `d` | Set due date |
| `m` | Assign to me |
| `u` | Unassign |
| `n` | Create new issue |
| `g` | Git checkout branch |
| `r` | Refresh |
| `c` | Copy issue identifier |
| `t` | Add comment |
| `b` | Create sub-issue |
| `x` | Archive issue |
| `T` | Edit title |
| `f` | Edit labels |
| `Space` | Toggle bulk select |
| `[` / `]` | Collapse / Expand all sub-issues |
| `:` | Open command palette |
| `/` | Search |
| `q` | Quit |
| `: velocity` | Velocity chart overlay |
| `: stats` | Team throughput stats |
| `: export_cycle` | Export cycle report to clipboard |
| `: triage` | Triage mode (rapid backlog grooming) |
| `: save_view` | Save current filters as named view |
| `: delete_view` | Delete a saved view |

---

## Deferred / Won't Do

| Feature | Reason |
|---------|--------|
| Full PTY terminal emulator | tview wasn't designed for it; tmux splits are better |
| Time tracking | Out of scope for Linear TUI; use dedicated tool |
| Multi-workspace switching | Low frequency; restart with different API key |
| Slack/Discord integration | Different surface |

---

## Build

```bash
cd ~/projects/linear-tui-go
LINEAR_API_KEY=$LINEAR_API_KEY ./linear-tui-go
```

Config: `~/.linear-tui/config.json` — set `agent_provider: "claude"` and `agent_workspace` to your repo path.
