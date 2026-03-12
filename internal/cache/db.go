package cache

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/cheenu1092-oss/prflow/internal/gh"
)

type DB struct {
	db *sql.DB
}

type CachedPR struct {
	gh.PR
	Repo           string
	Section        string // do_now, waiting, review, done
	CIStatus       string
	ReviewDecision string
	FetchedAt      time.Time
}

func Open() (*DB, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "prflow")
	os.MkdirAll(dir, 0755)
	dbPath := filepath.Join(dir, "prflow.db")

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL")
	if err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &DB{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS prs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo TEXT NOT NULL,
			number INTEGER NOT NULL,
			title TEXT,
			state TEXT,
			author TEXT,
			branch TEXT,
			base_branch TEXT,
			url TEXT,
			created_at TEXT,
			updated_at TEXT,
			mergeable TEXT,
			review_decision TEXT,
			ci_status TEXT,
			is_draft INTEGER DEFAULT 0,
			section TEXT,
			raw_json TEXT,
			fetched_at TEXT,
			UNIQUE(repo, number)
		);

		CREATE TABLE IF NOT EXISTS review_threads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			thread_id TEXT,
			path TEXT,
			line INTEGER,
			is_resolved INTEGER DEFAULT 0,
			last_author TEXT,
			last_body TEXT,
			needs_my_reply INTEGER DEFAULT 0,
			url TEXT,
			raw_json TEXT,
			UNIQUE(repo, pr_number, thread_id)
		);

		CREATE TABLE IF NOT EXISTS favorites (
			repo TEXT PRIMARY KEY,
			added_at TEXT
		);
	`)
	return err
}

func (d *DB) UpsertPR(pr *gh.PR, repo, section string) error {
	raw, _ := json.Marshal(pr)
	ciStatus := "UNKNOWN"
	for _, check := range pr.StatusCheckRollup {
		if check.Conclusion == "FAILURE" {
			ciStatus = "FAILURE"
			break
		}
		if check.Status == "IN_PROGRESS" {
			ciStatus = "PENDING"
		}
		if check.Conclusion == "SUCCESS" && ciStatus != "PENDING" {
			ciStatus = "SUCCESS"
		}
	}

	_, err := d.db.Exec(`
		INSERT INTO prs (repo, number, title, state, author, branch, base_branch, url,
			created_at, updated_at, mergeable, review_decision, ci_status, is_draft, section, raw_json, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo, number) DO UPDATE SET
			title=excluded.title, state=excluded.state, author=excluded.author,
			branch=excluded.branch, base_branch=excluded.base_branch, url=excluded.url,
			updated_at=excluded.updated_at, mergeable=excluded.mergeable,
			review_decision=excluded.review_decision, ci_status=excluded.ci_status,
			is_draft=excluded.is_draft, section=excluded.section, raw_json=excluded.raw_json,
			fetched_at=excluded.fetched_at
	`,
		repo, pr.Number, pr.Title, pr.State, pr.Author.Login,
		pr.HeadRefName, pr.BaseRefName, pr.URL,
		pr.CreatedAt, pr.UpdatedAt, pr.Mergeable, pr.ReviewDecision,
		ciStatus, pr.IsDraft, section, string(raw), time.Now().Format(time.RFC3339),
	)
	return err
}

func (d *DB) GetPRsBySection(section string) ([]CachedPR, error) {
	rows, err := d.db.Query(`
		SELECT repo, number, title, state, author, branch, base_branch, url,
			created_at, updated_at, mergeable, review_decision, ci_status, is_draft, section, fetched_at
		FROM prs WHERE section = ? ORDER BY updated_at DESC
	`, section)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []CachedPR
	for rows.Next() {
		var p CachedPR
		var isDraft int
		var fetchedAt string
		err := rows.Scan(
			&p.Repo, &p.Number, &p.Title, &p.State, &p.Author.Login,
			&p.HeadRefName, &p.BaseRefName, &p.URL,
			&p.CreatedAt, &p.UpdatedAt, &p.Mergeable, &p.ReviewDecision,
			&p.CIStatus, &isDraft, &p.Section, &fetchedAt,
		)
		if err != nil {
			continue
		}
		p.IsDraft = isDraft == 1
		p.FetchedAt, _ = time.Parse(time.RFC3339, fetchedAt)
		prs = append(prs, p)
	}
	return prs, nil
}

func (d *DB) GetAllPRs() ([]CachedPR, error) {
	rows, err := d.db.Query(`
		SELECT repo, number, title, state, author, branch, base_branch, url,
			created_at, updated_at, mergeable, review_decision, ci_status, is_draft, section, fetched_at
		FROM prs WHERE state = 'OPEN' OR state = 'open' ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []CachedPR
	for rows.Next() {
		var p CachedPR
		var isDraft int
		var fetchedAt string
		err := rows.Scan(
			&p.Repo, &p.Number, &p.Title, &p.State, &p.Author.Login,
			&p.HeadRefName, &p.BaseRefName, &p.URL,
			&p.CreatedAt, &p.UpdatedAt, &p.Mergeable, &p.ReviewDecision,
			&p.CIStatus, &isDraft, &p.Section, &fetchedAt,
		)
		if err != nil {
			continue
		}
		p.IsDraft = isDraft == 1
		p.FetchedAt, _ = time.Parse(time.RFC3339, fetchedAt)
		prs = append(prs, p)
	}
	return prs, nil
}

func (d *DB) AddFavorite(repo string) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO favorites (repo, added_at) VALUES (?, ?)
	`, repo, time.Now().Format(time.RFC3339))
	return err
}

func (d *DB) RemoveFavorite(repo string) error {
	_, err := d.db.Exec(`DELETE FROM favorites WHERE repo = ?`, repo)
	return err
}

func (d *DB) GetFavorites() ([]string, error) {
	rows, err := d.db.Query(`SELECT repo FROM favorites ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var favs []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err == nil {
			favs = append(favs, repo)
		}
	}
	return favs, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}
