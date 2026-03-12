# PRFlow Workspace — Local Repo Dashboard

## The Problem

You have 8 repos cloned locally. You context-switch between them constantly. When you come back to one:
- What branch am I on?
- How far behind is main?
- Do I have uncommitted changes?
- Did I forget to push something?
- Which PR does this branch belong to?

You end up running `git status`, `git log --oneline`, `git fetch`, `git diff` in every repo. Tedious.

## Solution: Workspace Section in PRFlow TUI

New section in the left sidebar: **📂 Workspace**

### Workspace TUI View

```
╔══════════════════════════════════════════════════════════════╗
║  PRFlow — Workspace                          8 repos        ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  📂 WORKSPACE                                                ║
║                                                              ║
║  ┌────────────────────────────────────────────────────────┐  ║
║  │ juniper/mist-api         ~/repos/mist-api              │  ║
║  │ ├─ branch: feature/auth-refactor                       │  ║
║  │ ├─ ↓ 12 behind main  ↑ 3 ahead                        │  ║
║  │ ├─ 2 modified · 1 untracked                            │  ║
║  │ ├─ linked: PR #412 (changes requested)                 │  ║
║  │ └─ last commit: 2h ago "fix token validation"          │  ║
║  ├────────────────────────────────────────────────────────┤  ║
║  │ hpe/wifi-engine          ~/repos/wifi-engine           │  ║
║  │ ├─ branch: main ✓ (clean, up to date)                  │  ║
║  │ ├─ ↓ 0 behind  ↑ 0 ahead                              │  ║
║  │ ├─ clean working tree                                   │  ║
║  │ └─ last commit: 1d ago "v2.3.1 release"                │  ║
║  ├────────────────────────────────────────────────────────┤  ║
║  │ hpe/wan-core             ~/repos/wan-core              │  ║
║  │ ├─ branch: fix/memory-leak                             │  ║
║  │ ├─ ↓ 47 behind main ⚠️  ↑ 5 ahead                     │  ║
║  │ ├─ 1 staged · 3 modified                               │  ║
║  │ ├─ unpushed commits: 2                                  │  ║
║  │ ├─ linked: PR #91 (waiting on review)                  │  ║
║  │ └─ last commit: 3d ago "fix goroutine leak in handler" │  ║
║  ├────────────────────────────────────────────────────────┤  ║
║  │ hpe/iot-dash             ~/repos/iot-dash              │  ║
║  │ ├─ branch: feature/dashboard-v2                        │  ║
║  │ ├─ ↓ 3 behind main  ↑ 8 ahead                         │  ║
║  │ ├─ clean working tree                                   │  ║
║  │ ├─ linked: PR #203 (merge conflict ⚠️)                 │  ║
║  │ └─ last commit: 5h ago "add chart component"           │  ║
║  └────────────────────────────────────────────────────────┘  ║
║                                                              ║
║  [enter] expand  [p] pull  [P] push  [f] fetch all          ║
║  [s] stash  [c] checkout branch  [o] open in GitHub          ║
╚══════════════════════════════════════════════════════════════╝
```

### Color Coding

- **Green**: Clean, up to date, on main
- **Yellow**: Uncommitted changes, or slightly behind main  
- **Red**: 20+ commits behind main, merge conflicts, unpushed commits > 1 day old

### Expanded Repo View (press Enter)

```
╔══════════════════════════════════════════════════════════════╗
║  juniper/mist-api                     ~/repos/mist-api      ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║  Current: feature/auth-refactor → main                      ║
║  Status:  ↓ 12 behind main · ↑ 3 ahead · 2 modified        ║
║  PR:      #412 — Refactor auth token handling               ║
║           ↗ github.com/juniper/mist-api/pull/412            ║
║                                                              ║
║  📊 Branch vs Main                                           ║
║  main ━━━━━━━━━━━━●━━━━━━━━━━━━● (12 new commits)          ║
║                    ╲                                         ║
║  feature/auth...    ●━━●━━● (3 commits ahead)               ║
║                                                              ║
║  📝 Modified Files                                           ║
║    M src/auth.rs          (+45 -12)                          ║
║    M src/middleware.rs     (+8 -3)                            ║
║    ? tests/auth_test.rs   (untracked)                        ║
║                                                              ║
║  📋 Recent Commits (on this branch)                          ║
║    abc1234 fix token validation            (2h ago)          ║
║    def5678 add expiry check                (1d ago)          ║
║    ghi9012 refactor auth module            (3d ago)          ║
║                                                              ║
║  🌿 Local Branches                                           ║
║    * feature/auth-refactor                                   ║
║      main                                                    ║
║      fix/ci-pipeline                                         ║
║      feature/metrics (stale — 14d ago)                       ║
║                                                              ║
║  Quick Actions:                                              ║
║  [p] git pull  [P] git push  [r] rebase on main             ║
║  [d] diff      [l] log       [b] switch branch              ║
║  [S] stash     [o] open GitHub  [esc] back                   ║
╚══════════════════════════════════════════════════════════════╝
```

### Quick Actions (Wrappers)

These are one-keypress git operations on the selected repo:

| Key | Action | Git Command |
|-----|--------|-------------|
| `f` | Fetch all repos | `git fetch --all` (runs on ALL workspace repos in parallel) |
| `p` | Pull current branch | `git pull origin <branch>` |
| `P` | Push current branch | `git push origin <branch>` |
| `r` | Rebase on main | `git fetch origin main && git rebase origin/main` |
| `m` | Merge main into branch | `git fetch origin main && git merge origin/main` |
| `b` | Switch branch | Shows branch picker (local branches) |
| `d` | Show diff | `git diff` (in pager or inline) |
| `S` | Stash changes | `git stash push -m "prflow stash"` |
| `U` | Pop stash | `git stash pop` |
| `l` | Show log | Last 10 commits inline |
| `n` | New branch | Prompt for name, `git checkout -b <name>` |

**Important:** All actions show confirmation before executing. Show the exact git command that will run.

### Workspace Auto-Discovery

On first run (during onboarding), scan common directories for git repos:
- `~/repos/`
- `~/Projects/`
- `~/src/`
- `~/code/`
- `~/work/`
- Or user-configured `repos_dir` from config

Match local repos to GitHub remotes:
```bash
# For each found git repo:
git -C <path> remote get-url origin
# Parse org/repo from the URL
# Match against user's GitHub repos
```

### Config Addition

```yaml
# ~/.config/prflow/config.yaml (add to existing)
workspace:
  scan_dirs:
    - ~/repos
    - ~/Projects
    - ~/work
  repos:
    # Auto-discovered or manually added:
    juniper/mist-api: ~/repos/mist-api
    hpe/wifi-engine: ~/repos/wifi-engine
    hpe/wan-core: ~/repos/wan-core
```

### How Workspace Links to PRs

The magic: workspace knows which PR each branch belongs to.

```bash
# For each local branch, find its PR:
gh pr list -R <repo> --head <branch-name> --json number,title,state
```

This lets the workspace view show:
- `linked: PR #412 (changes requested)` — your branch has an open PR
- `no PR` — branch exists locally but no PR created yet
- `PR #87 merged` — you can clean up this branch

### Git Operations Under the Hood

All workspace data comes from local git commands (fast, no API needed):

```bash
# Current branch
git -C <path> branch --show-current

# Behind/ahead of main
git -C <path> rev-list --left-right --count origin/main...HEAD

# Working tree status
git -C <path> status --porcelain

# Unpushed commits  
git -C <path> log origin/<branch>..HEAD --oneline

# Last commit
git -C <path> log -1 --format="%h %s (%cr)"

# Local branches with last activity
git -C <path> for-each-ref --sort=-committerdate refs/heads/ --format="%(refname:short) %(committerdate:relative)"
```

### Fetch All (bulk operation)

Press `f` in workspace view → fetches ALL repos in parallel:

```
Fetching 8 repos...
  ✓ mist-api        (2 new commits on main)
  ✓ wifi-engine      (up to date)
  ✓ wan-core         (15 new commits on main ⚠️)
  ✓ iot-dash         (1 new commit on main)
  ✓ ...
Done in 3.2s
```

## CLI Commands

```bash
prflow workspace              # show workspace summary (no TUI)
prflow workspace sync         # re-scan directories for repos
prflow workspace add <path>   # manually add a repo
prflow workspace rm <repo>    # remove from workspace
prflow workspace fetch        # fetch all repos
prflow workspace status       # quick status of all repos
```

## Implementation Notes

- Workspace data is LOCAL git commands — instant, no API rate limits
- PR linking uses `gh pr list` — cached in SQLite, refreshed with PRs
- Parallel fetch uses goroutines (one per repo, bounded concurrency)
- Stale branch detection: branches with no commits in 14+ days
- Branch cleanup suggestion: merged PR branches that still exist locally
