package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Schema migrations; append only. Index = schema version.
var migrations = []string{
	`
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);

CREATE TABLE shares (
	name TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE roles (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	perms TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE staff (
	id INTEGER PRIMARY KEY,
	username TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	role_id INTEGER REFERENCES roles(id) ON DELETE SET NULL,
	is_owner INTEGER NOT NULL DEFAULT 0,
	active INTEGER NOT NULL DEFAULT 1,
	notif_seen_id INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL,
	last_login INTEGER
);

CREATE TABLE customers (
	id INTEGER PRIMARY KEY,
	phone TEXT NOT NULL UNIQUE,
	name TEXT NOT NULL DEFAULT '',
	password_hash TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	lang TEXT NOT NULL DEFAULT '',
	active INTEGER NOT NULL DEFAULT 1,
	created_at INTEGER NOT NULL,
	last_login INTEGER
);

CREATE TABLE albums (
	id INTEGER PRIMARY KEY,
	title TEXT NOT NULL,
	share TEXT NOT NULL,
	rel_path TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'selecting',
	max_edit INTEGER NOT NULL DEFAULT 0,
	allow_download INTEGER NOT NULL DEFAULT 1,
	expires_at INTEGER,
	due_at INTEGER,
	note_internal TEXT NOT NULL DEFAULT '',
	note_customer TEXT NOT NULL DEFAULT '',
	customer_note TEXT NOT NULL DEFAULT '',
	share_token TEXT,
	share_enabled INTEGER NOT NULL DEFAULT 0,
	share_expires_at INTEGER,
	cover_photo_id INTEGER,
	photo_count INTEGER NOT NULL DEFAULT 0,
	final_count INTEGER NOT NULL DEFAULT 0,
	last_scan INTEGER NOT NULL DEFAULT 0,
	submitted_at INTEGER,
	created_by INTEGER,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX albums_share_token ON albums(share_token) WHERE share_token IS NOT NULL;

CREATE TABLE album_customers (
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	customer_id INTEGER NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
	PRIMARY KEY (album_id, customer_id)
);
CREATE INDEX album_customers_c ON album_customers(customer_id);

CREATE TABLE album_editors (
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	staff_id INTEGER NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
	PRIMARY KEY (album_id, staff_id)
);
CREATE INDEX album_editors_s ON album_editors(staff_id);

CREATE TABLE photos (
	id INTEGER PRIMARY KEY,
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	rel TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'orig',
	name TEXT NOT NULL,
	size INTEGER NOT NULL DEFAULT 0,
	mtime INTEGER NOT NULL DEFAULT 0,
	width INTEGER NOT NULL DEFAULT 0,
	height INTEGER NOT NULL DEFAULT 0,
	taken_at INTEGER NOT NULL DEFAULT 0,
	thumb_ok INTEGER NOT NULL DEFAULT 0,
	fav INTEGER NOT NULL DEFAULT 0,
	edit_pick INTEGER NOT NULL DEFAULT 0,
	edit_done INTEGER NOT NULL DEFAULT 0,
	viewed INTEGER NOT NULL DEFAULT 0,
	pick_by TEXT NOT NULL DEFAULT '',
	UNIQUE (album_id, rel)
);

CREATE TABLE comments (
	id INTEGER PRIMARY KEY,
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	photo_id INTEGER REFERENCES photos(id) ON DELETE CASCADE,
	author_type TEXT NOT NULL,
	author_id INTEGER NOT NULL,
	author_name TEXT NOT NULL,
	body TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX comments_album ON comments(album_id, id);
CREATE INDEX comments_photo ON comments(photo_id);

CREATE TABLE reads (
	album_id INTEGER NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
	reader TEXT NOT NULL,
	last_comment_id INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (album_id, reader)
);

CREATE TABLE notifications (
	id INTEGER PRIMARY KEY,
	kind TEXT NOT NULL,
	album_id INTEGER REFERENCES albums(id) ON DELETE CASCADE,
	text TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE sessions (
	token_hash TEXT PRIMARY KEY,
	user_type TEXT NOT NULL,
	user_id INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
`,
}

func openDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(8000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	for i := v; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return seed(db)
}

// seed inserts the default roles once.
func seed(db *sql.DB) error {
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM roles`).Scan(&n)
	if n > 0 {
		return nil
	}
	defaults := []struct {
		name  string
		perms []string
	}{
		{"Quản trị", AllPerms},
		{"Sale / CSKH", []string{PermCustomers, PermAlbumsAll, PermAlbumsManage, PermAlbumsUnlock, PermChat}},
		{"Thợ edit", []string{PermEditWork, PermChat}},
	}
	for _, r := range defaults {
		b, _ := json.Marshal(r.perms)
		if _, err := db.Exec(`INSERT INTO roles(name, perms) VALUES(?, ?)`, r.name, string(b)); err != nil {
			return err
		}
	}
	return nil
}

func now() int64 { return time.Now().Unix() }

func getSetting(db *sql.DB, key, def string) string {
	var v string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

func setSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// nullInt converts a nullable unix timestamp for JSON (0 = unset).
func nullInt(n sql.NullInt64) int64 {
	if n.Valid {
		return n.Int64
	}
	return 0
}

func toNull(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
