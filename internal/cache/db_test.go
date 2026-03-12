package cache

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/cheenu1092-oss/prflow/internal/gh"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &DB{db: db}
}

func TestOpenCreatesDB(t *testing.T) {
	// Override home for test
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	db, err := Open()
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Check file exists
	dbPath := filepath.Join(tmpDir, ".config", "prflow", "prflow.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestUpsertAndGetPR(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pr := &gh.PR{
		Number:         42,
		Title:          "Fix auth bug",
		State:          "OPEN",
		URL:            "https://github.com/org/repo/pull/42",
		HeadRefName:    "fix/auth",
		BaseRefName:    "main",
		Author:         gh.Author{Login: "nagaconda"},
		CreatedAt:      "2026-03-01T10:00:00Z",
		UpdatedAt:      "2026-03-10T15:30:00Z",
		ReviewDecision: "CHANGES_REQUESTED",
		IsDraft:        false,
	}

	err := db.UpsertPR(pr, "org/repo", "do_now")
	if err != nil {
		t.Fatalf("UpsertPR failed: %v", err)
	}

	// Get by section
	prs, err := db.GetPRsBySection("do_now")
	if err != nil {
		t.Fatalf("GetPRsBySection failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 42 {
		t.Errorf("expected PR #42, got #%d", prs[0].Number)
	}
	if prs[0].Repo != "org/repo" {
		t.Errorf("expected repo 'org/repo', got '%s'", prs[0].Repo)
	}
	if prs[0].Title != "Fix auth bug" {
		t.Errorf("expected title 'Fix auth bug', got '%s'", prs[0].Title)
	}
}

func TestUpsertUpdatesExisting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pr := &gh.PR{
		Number: 42,
		Title:  "Original title",
		State:  "OPEN",
		Author: gh.Author{Login: "user"},
	}

	db.UpsertPR(pr, "org/repo", "waiting")

	// Update same PR
	pr.Title = "Updated title"
	pr.ReviewDecision = "APPROVED"
	db.UpsertPR(pr, "org/repo", "do_now")

	// Should have only 1 PR
	all, _ := db.GetAllPRs()
	if len(all) != 1 {
		t.Fatalf("expected 1 PR after upsert, got %d", len(all))
	}
	if all[0].Title != "Updated title" {
		t.Errorf("expected 'Updated title', got '%s'", all[0].Title)
	}
	if all[0].Section != "do_now" {
		t.Errorf("expected section 'do_now', got '%s'", all[0].Section)
	}
}

func TestGetPRsBySectionEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	prs, err := db.GetPRsBySection("do_now")
	if err != nil {
		t.Fatalf("GetPRsBySection failed: %v", err)
	}
	if len(prs) != 0 {
		t.Errorf("expected 0 PRs, got %d", len(prs))
	}
}

func TestGetAllPRs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert multiple PRs
	for i := 1; i <= 5; i++ {
		pr := &gh.PR{
			Number: i,
			Title:  "PR",
			State:  "OPEN",
			Author: gh.Author{Login: "user"},
		}
		db.UpsertPR(pr, "org/repo", "waiting")
	}

	all, err := db.GetAllPRs()
	if err != nil {
		t.Fatalf("GetAllPRs failed: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 PRs, got %d", len(all))
	}
}

func TestGetAllPRsExcludesClosed(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	open := &gh.PR{Number: 1, Title: "Open", State: "OPEN", Author: gh.Author{Login: "u"}}
	closed := &gh.PR{Number: 2, Title: "Closed", State: "CLOSED", Author: gh.Author{Login: "u"}}

	db.UpsertPR(open, "org/repo", "waiting")
	db.UpsertPR(closed, "org/repo", "done")

	all, _ := db.GetAllPRs()
	if len(all) != 1 {
		t.Errorf("expected 1 open PR, got %d", len(all))
	}
}

func TestFavorites(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add favorites
	db.AddFavorite("org/repo1")
	db.AddFavorite("org/repo2")
	db.AddFavorite("org/repo3")

	favs, err := db.GetFavorites()
	if err != nil {
		t.Fatalf("GetFavorites failed: %v", err)
	}
	if len(favs) != 3 {
		t.Fatalf("expected 3 favorites, got %d", len(favs))
	}

	// Remove one
	db.RemoveFavorite("org/repo2")
	favs, _ = db.GetFavorites()
	if len(favs) != 2 {
		t.Errorf("expected 2 favorites after remove, got %d", len(favs))
	}

	// Add duplicate (should not error)
	err = db.AddFavorite("org/repo1")
	if err != nil {
		t.Errorf("duplicate add should not error: %v", err)
	}
	favs, _ = db.GetFavorites()
	if len(favs) != 2 {
		t.Errorf("expected 2 favorites after duplicate, got %d", len(favs))
	}
}

func TestFavoritesEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	favs, err := db.GetFavorites()
	if err != nil {
		t.Fatalf("GetFavorites failed: %v", err)
	}
	if len(favs) != 0 {
		t.Errorf("expected 0 favorites, got %d", len(favs))
	}
}

func TestDraftPR(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pr := &gh.PR{
		Number:  1,
		Title:   "Draft PR",
		State:   "OPEN",
		Author:  gh.Author{Login: "user"},
		IsDraft: true,
	}
	db.UpsertPR(pr, "org/repo", "waiting")

	prs, _ := db.GetPRsBySection("waiting")
	if len(prs) != 1 {
		t.Fatalf("expected 1, got %d", len(prs))
	}
	if !prs[0].IsDraft {
		t.Error("expected IsDraft true")
	}
}

func TestMultipleRepos(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	pr1 := &gh.PR{Number: 1, Title: "PR1", State: "OPEN", Author: gh.Author{Login: "u"}}
	pr2 := &gh.PR{Number: 1, Title: "PR2", State: "OPEN", Author: gh.Author{Login: "u"}}

	db.UpsertPR(pr1, "org/repo1", "waiting")
	db.UpsertPR(pr2, "org/repo2", "do_now")

	// Same PR number but different repos = 2 entries
	all, _ := db.GetAllPRs()
	if len(all) != 2 {
		t.Errorf("expected 2 PRs from different repos, got %d", len(all))
	}
}
