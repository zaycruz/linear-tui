# linear-tui Roadmap

Fork of [roeyazroel/linear-tui](https://github.com/roeyazroel/linear-tui) — extended for daily operator use.

---

## Phase 1 — Core Workflow (current)

> Goal: turn the TUI from a viewer into a worker. Every action that currently requires leaving the terminal gets a keybinding.

| # | Feature | Key | Status |
|---|---------|-----|--------|
| 1.1 | **Quick status change** | `s` | In progress |
| 1.2 | **Git branch checkout** | `g` | In progress |
| 1.3 | **Cycle burndown chart** | auto in cycle view | In progress |
| 1.4 | Linear Cycles support | sidebar | Done ✓ |

**Keybinding additions:**
- `s` — change issue status inline (picker from workflow states)
- `g` — `git checkout -b <branchName>` in configured workspace

**Burndown chart format (cycle detail view):**
```
Cycle #1  May 11→25  ░░░▂▃▄▅▆  12/28 done  42%
```

---

## Phase 2 — Sprint Planning

> Goal: plan and manage sprints entirely from the TUI. Zero web app required for day-to-day sprint ops.

| # | Feature | Key/Trigger | Notes |
|---|---------|-------------|-------|
| 2.1 | **Priority change** | `p` | Picker: Urgent/High/Medium/Low/None |
| 2.2 | **Assignee change** | `a` | Picker from team members |
| 2.3 | **Bulk issue select** | `Space` to toggle | Multi-select in issues list |
| 2.4 | **Bulk add to cycle** | `Space` select → `: add_to_cycle` | Batch assign selected issues |
| 2.5 | **Bulk status change** | `Space` select → `s` | Batch status update |
| 2.6 | **Project health panel** | auto in project view | Status breakdown (N backlog/in-progress/done) in right pane when project selected, before drilling into issues |
| 2.7 | **Due date display** | column in issues table | Overdue issues highlighted red |
| 2.8 | **Estimate display** | column in issues table | Story points visible inline |

**Multi-select UX:**
- `Space` toggles selection on focused issue (checkbox indicator in leftmost column)
- Selected count shown in status bar: `3 selected`
- Bulk commands appear in palette when selection is active
- `Escape` clears selection

---

## Phase 3 — Navigation & Discovery

> Goal: find anything instantly. The TUI becomes the fastest way to get to any issue, not just issues you already know about.

| # | Feature | Key/Trigger | Notes |
|---|---------|-------------|-------|
| 3.1 | **Fuzzy issue finder** | `Ctrl+P` | Global search across all issues, instant filter, jump to result |
| 3.2 | **Saved filter views** | named, accessible from sidebar | Save current filter/sort combo with a name; persisted in config.json |
| 3.3 | **Assignee filter toggle** | `Shift+A` | Toggle between all issues and issues assigned to me |
| 3.4 | **Label filter** | `Shift+L` | Filter current view by label |
| 3.5 | **Open in browser** | `o` | Open focused issue in Linear web (`open <url>`) |
| 3.6 | **Copy branch name** | `Shift+C` | Copy issue git branch name to clipboard |
| 3.7 | **Issue linking** | `: link` | Mark focused issue as blocked-by or blocks another (type issue ID) |
| 3.8 | **Mention/notification view** | sidebar section | Issues where you're @mentioned, fetched from Linear notifications API |

**Fuzzy finder UX:**
- `Ctrl+P` opens full-screen overlay
- Type to filter across title + identifier across all cached issues
- Arrow keys navigate results, Enter jumps to issue in context
- Dismiss with Escape

---

## Phase 4 — Analytics & Reporting

> Goal: sprint reviews and team health visible without leaving the terminal.

| # | Feature | Key/Trigger | Notes |
|---|---------|-------------|-------|
| 4.1 | **Velocity chart** | `: velocity` or sidebar | Issues completed per cycle, last 6 cycles, ASCII bar chart |
| 4.2 | **Per-assignee breakdown** | in cycle detail | Below burndown: N done / N in-progress per person |
| 4.3 | **Team throughput summary** | `: stats` | Issues opened vs closed this week/month |
| 4.4 | **Cycle comparison** | `: compare` | Side-by-side stats for last 2 cycles |
| 4.5 | **Export cycle report** | `: export` | Markdown summary of cycle to clipboard or file |

**Velocity chart format:**
```
Velocity (last 6 cycles)
Cycle 1  ████████░░░░  8
Cycle 2  ██████████░░  10
Cycle 3  ████████████  12
Cycle 4  ██████░░░░░░  6
Cycle 5  █████████░░░  9
Cycle 6  ████████████  12 ← current
```

---

## Deferred / Won't Do

| Feature | Reason |
|---------|--------|
| Full PTY terminal emulator | tview wasn't designed for it; tmux splits are better |
| Time tracking | Out of scope for Linear TUI; use dedicated tool |
| Multi-workspace switching | Low frequency; restart with different API key |
| Slack/Discord integration | Different surface |

---

## Branch Strategy

| Branch | Purpose |
|--------|---------|
| `main` | Stable, shippable |
| `feature/linear-cycles` | Phase 1 work (current) |
| `feature/sprint-planning` | Phase 2 (next) |
| `feature/discovery` | Phase 3 |
| `feature/analytics` | Phase 4 |

Each phase merges to `main` before the next begins.

---

## Build

```bash
cd ~/projects/linear-tui-go
LINEAR_API_KEY=$LINEAR_API_KEY ./linear-tui-go
```

Config: `~/.linear-tui/config.json` — set `agent_provider: "claude"` and `agent_workspace` to your repo path.
