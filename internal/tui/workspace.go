package tui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// RepoStatus holds local git state for a workspace repo
type RepoStatus struct {
	Name       string // org/repo
	Path       string // local path
	Branch     string
	Behind     int
	Ahead      int
	Modified   int
	Staged     int
	Untracked  int
	Unpushed   int
	LastCommit string // short hash + message + relative time
	Clean      bool
	HasRemote  bool
	LinkedPR   string // e.g., "#412 (changes requested)"
}

// ScanWorkspaceRepo gets git status for a local repo
func ScanWorkspaceRepo(path string) (*RepoStatus, error) {
	rs := &RepoStatus{Path: path}

	// Get repo name from remote
	remote, err := gitCmd(path, "remote", "get-url", "origin")
	if err != nil {
		rs.Name = filepath.Base(path)
	} else {
		rs.Name = parseRepoName(remote)
		rs.HasRemote = true
	}

	// Current branch
	branch, err := gitCmd(path, "branch", "--show-current")
	if err != nil {
		rs.Branch = "detached"
	} else {
		rs.Branch = branch
	}

	// Fetch silently (don't block on this)
	// We just use cached data

	// Behind/ahead of main
	if rs.HasRemote {
		counts, err := gitCmd(path, "rev-list", "--left-right", "--count", "origin/main...HEAD")
		if err == nil {
			parts := strings.Fields(counts)
			if len(parts) == 2 {
				fmt.Sscanf(parts[0], "%d", &rs.Behind)
				fmt.Sscanf(parts[1], "%d", &rs.Ahead)
			}
		}
	}

	// Working tree status
	status, err := gitCmd(path, "status", "--porcelain")
	if err == nil && status != "" {
		for _, line := range strings.Split(status, "\n") {
			if len(line) < 2 {
				continue
			}
			x, y := line[0], line[1]
			if x == '?' {
				rs.Untracked++
			} else if x != ' ' {
				rs.Staged++
			}
			if y != ' ' && y != '?' {
				rs.Modified++
			}
		}
	}

	// Unpushed commits
	if rs.HasRemote && rs.Branch != "" && rs.Branch != "detached" {
		unpushed, err := gitCmd(path, "log", fmt.Sprintf("origin/%s..HEAD", rs.Branch), "--oneline")
		if err == nil && unpushed != "" {
			rs.Unpushed = len(strings.Split(unpushed, "\n"))
		}
	}

	// Last commit
	lastCommit, err := gitCmd(path, "log", "-1", "--format=%h %s (%cr)")
	if err == nil {
		rs.LastCommit = lastCommit
	}

	rs.Clean = rs.Modified == 0 && rs.Staged == 0 && rs.Untracked == 0

	return rs, nil
}

// RenderRepoStatus renders a workspace repo for the TUI
func RenderRepoStatus(rs *RepoStatus, selected bool) string {
	var s strings.Builder

	nameStr := rs.Name
	if selected {
		nameStr = prItemSelectedStyle.Render(rs.Name)
	} else {
		nameStr = prNumberStyle.Render(rs.Name)
	}

	s.WriteString(fmt.Sprintf("  %s\n", nameStr))
	s.WriteString(fmt.Sprintf("  ├─ branch: %s\n", rs.Branch))

	// Behind/ahead
	behindAhead := ""
	if rs.Behind > 0 {
		ba := fmt.Sprintf("↓ %d behind main", rs.Behind)
		if rs.Behind > 20 {
			behindAhead = statusBehind.Render(ba+" ⚠️")
		} else {
			behindAhead = statusDirty.Render(ba)
		}
	} else {
		behindAhead = statusClean.Render("↓ 0 behind")
	}
	if rs.Ahead > 0 {
		behindAhead += fmt.Sprintf("  ↑ %d ahead", rs.Ahead)
	}
	s.WriteString(fmt.Sprintf("  ├─ %s\n", behindAhead))

	// Working tree
	if rs.Clean {
		s.WriteString(fmt.Sprintf("  ├─ %s\n", statusClean.Render("clean working tree ✓")))
	} else {
		var parts []string
		if rs.Staged > 0 {
			parts = append(parts, fmt.Sprintf("%d staged", rs.Staged))
		}
		if rs.Modified > 0 {
			parts = append(parts, fmt.Sprintf("%d modified", rs.Modified))
		}
		if rs.Untracked > 0 {
			parts = append(parts, fmt.Sprintf("%d untracked", rs.Untracked))
		}
		s.WriteString(fmt.Sprintf("  ├─ %s\n", statusDirty.Render(strings.Join(parts, " · "))))
	}

	// Unpushed
	if rs.Unpushed > 0 {
		s.WriteString(fmt.Sprintf("  ├─ %s\n", statusDirty.Render(fmt.Sprintf("unpushed commits: %d", rs.Unpushed))))
	}

	// Linked PR
	if rs.LinkedPR != "" {
		s.WriteString(fmt.Sprintf("  ├─ linked: %s\n", rs.LinkedPR))
	}

	// Last commit
	if rs.LastCommit != "" {
		s.WriteString(fmt.Sprintf("  └─ %s\n", repoStyle.Render(rs.LastCommit)))
	}

	return s.String()
}

func parseRepoName(remote string) string {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	// Handle SSH: git@github.com:org/repo
	if strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(remote, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	// Handle HTTPS: https://github.com/org/repo
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return remote
}

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
