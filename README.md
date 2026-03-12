# PRFlow ⚡

**Terminal-first GitHub PR dashboard.** See all your PRs, review comments, and workspace status in one TUI. No more context-switching to GitHub.com.

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

## Why?

GitHub's PR management sucks when you're juggling multiple repos. Notifications are a firehose. The PR tab is per-repo. There's no unified **"what do I need to do right now?"** view.

PRFlow fixes this. One command, one TUI, everything you need.

## Features

- **⚡ Do Now** — PRs needing YOUR action (unresolved comments, ready to merge, conflicts)
- **⏳ Waiting** — PRs blocked on reviewers (with who + how long)
- **👀 Review** — PRs where you're a requested reviewer
- **📂 Workspace** — Local git status for all your repos (branch, behind/ahead main, dirty files)
- **✅ Done** — Recently merged PRs
- **★ Favorites** — Star repos for detailed sidebar tracking
- **🔗 Links** — Every item opens directly on GitHub.com with `[o]`
- **📝 Review Threads** — Expand any PR to see unresolved comment threads

## Install

### From Source

```bash
# Prerequisites
# - Go 1.25+
# - gh CLI (https://cli.github.com) authenticated

git clone https://github.com/cheenu1092-oss/prflow.git
cd prflow
go build -o prflow .

# Move to PATH
mv prflow /usr/local/bin/
# or
mv prflow ~/.local/bin/
```

### Prerequisites

1. **Go 1.25+** — [golang.org/dl](https://golang.org/dl/)
2. **GitHub CLI (`gh`)** — [cli.github.com](https://cli.github.com)
3. **Authenticate gh:**
   ```bash
   gh auth login
   gh auth status  # verify it works
   ```

## Quick Start

```bash
# First run — launches onboarding wizard
prflow

# Or run setup explicitly
prflow setup
```

The onboarding wizard will:
1. ✅ Check your `gh` auth
2. 📡 Scan repos from your recent PR activity
3. ☑️ Let you select which repos to track
4. ⭐ Pick favorites for sidebar tracking
5. 💾 Save config to `~/.config/prflow/config.yaml`

## Usage

```bash
prflow              # Launch TUI dashboard
prflow setup        # Re-run onboarding wizard
prflow sync         # Force refresh PR cache
prflow ls           # Quick list (no TUI)
prflow config       # Show config file path
prflow version      # Print version
```

## TUI Key Bindings

### Navigation
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate items |
| `Tab` | Next section |
| `Shift+Tab` | Previous section |
| `Enter` | Expand PR detail |
| `Esc` | Back to list |
| `q` | Quit |

### Actions
| Key | Action |
|-----|--------|
| `o` | Open in GitHub.com (browser) |
| `c` | Checkout PR branch locally (finds repo or clones) |
| `C` | Clone PR's repo to repos dir |
| `/` | Search org repos to clone |
| `a` | Approve PR (detail view) |
| `m` | Merge PR with squash (detail view) |
| `r` | Resolve selected review thread (detail view) |
| `R` | Force refresh all data |

### Workspace (📂 section)
| Key | Action |
|-----|--------|
| `p` | `git pull` current branch |
| `P` | `git push` current branch |
| `f` | Fetch all repos in parallel |

## Architecture

PRFlow is a thin wrapper around the `gh` CLI. It doesn't talk to the GitHub API directly — if `gh` works, PRFlow works.

```
┌─────────────┐
│   Human     │ ← TUI (Bubbletea)
├─────────────┤
│   PRFlow    │ ← smart wrapper, caching, favorites
├─────────────┤
│   gh CLI    │ ← does ALL the GitHub API work
├─────────────┤
│  GitHub.com │ ← every item links back here
└─────────────┘
```

**No tokens. No OAuth. No API keys.** Just `gh auth login` and go.

### Tech Stack
- **Go** — single binary, no runtime deps
- **[Bubbletea](https://github.com/charmbracelet/bubbletea)** — terminal UI framework
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** — TUI styling
- **SQLite** — local cache for instant TUI (no API wait on navigate)
- **YAML** — config at `~/.config/prflow/config.yaml`

## Configuration

Config lives at `~/.config/prflow/config.yaml`:

```yaml
repos:
  - org/repo-one
  - org/repo-two
  - org/repo-three

favorites:
  - org/repo-one
  - org/repo-two

workspace:
  scan_dirs:
    - ~/repos
    - ~/Projects
    - ~/work
  repos:
    org/repo-one: ~/repos/repo-one
    org/repo-two: ~/repos/repo-two

settings:
  refresh_interval: 2m
  stale_threshold: 3d
  editor: vim
  repos_dir: ~/repos
  merge_method: squash
  page_size: 50
```

## How Sections Work

### ⚡ Do Now
PRs where only **you** can unblock progress:
- Unresolved review comments on your PRs
- PRs approved + CI green → ready to merge
- Merge conflicts on your PRs
- CI failures you need to fix

### ⏳ Waiting
PRs blocked on **other people**:
- Your PRs waiting for reviewer response
- Shows who + how long (color-coded: 🟢 < 1d, 🟡 1-3d, 🔴 3d+)

### 👀 Review
PRs where **you're blocking someone**:
- Review requested from you
- Sorted by how long they've been waiting

### 📂 Workspace
Local git state for all your repos:
- Current branch + behind/ahead of main
- Modified/staged/untracked files
- Unpushed commits
- Last commit info
- Quick git shortcuts (pull, push, fetch)

## Roadmap

- [x] **v0.2** — Checkout PR branch locally (auto-find or clone)
- [x] **v0.2** — Search org repos + clone from TUI
- [x] **v0.2** — Done section (recently merged PRs)
- [x] **v0.2** — 2-level deep workspace scanning
- [ ] **v0.2** — Reply to review comments from TUI
- [ ] **v0.2** — Merge from TUI
- [ ] **v0.2** — Nudge stale reviewers
- [ ] **v0.3** — Background auto-refresh
- [ ] **v0.3** — Branch cleanup suggestions (merged PR branches)
- [ ] **v0.3** — `prflow ls --json` for scripting
- [ ] **v1.0** — Installable via `go install` / Homebrew

## Contributing

PRs welcome! This project is in early development.

```bash
# Dev setup
git clone https://github.com/cheenu1092-oss/prflow.git
cd prflow
go mod download
go build -o prflow .
./prflow
```

## License

MIT — see [LICENSE](LICENSE)
