package main

import (
	"database/sql"
	"os"
	"path"
	"strings"
	"sync"
)

const (
	StatusSelecting = "selecting" // customer is choosing
	StatusSubmitted = "submitted" // customer sent the selection; picks locked
	StatusEditing   = "editing"   // studio is retouching
	StatusDelivered = "delivered" // final photos delivered
)

var validStatus = map[string]bool{StatusSelecting: true, StatusSubmitted: true, StatusEditing: true, StatusDelivered: true}

type Album struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Share          string `json:"share,omitempty"`
	RelPath        string `json:"rel_path,omitempty"`
	Status         string `json:"status"`
	MaxEdit        int    `json:"max_edit"`
	AllowDownload  bool   `json:"allow_download"`
	ExpiresAt      int64  `json:"expires_at"`
	DueAt          int64  `json:"due_at,omitempty"`
	NoteInternal   string `json:"note_internal,omitempty"`
	NoteCustomer   string `json:"note_customer"`
	CustomerNote   string `json:"customer_note"`
	ShareToken     string `json:"share_token,omitempty"`
	ShareEnabled   bool   `json:"share_enabled,omitempty"`
	ShareExpiresAt int64  `json:"share_expires_at,omitempty"`
	CoverPhotoID   int64  `json:"cover"`
	PhotoCount     int    `json:"photo_count"`
	FinalCount     int    `json:"final_count"`
	LastScan       int64  `json:"-"`
	SubmittedAt    int64  `json:"submitted_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

const albumCols = `id, title, share, rel_path, status, max_edit, allow_download, expires_at, due_at, note_internal,
	note_customer, customer_note, share_token, share_enabled, share_expires_at, cover_photo_id, photo_count,
	final_count, last_scan, submitted_at, created_at, updated_at`

type scanner interface{ Scan(...any) error }

func scanAlbum(s scanner) (*Album, error) {
	var a Album
	var exp, due, sexp, cover, sub sql.NullInt64
	var tok sql.NullString
	var dl, sen int
	err := s.Scan(&a.ID, &a.Title, &a.Share, &a.RelPath, &a.Status, &a.MaxEdit, &dl, &exp, &due, &a.NoteInternal,
		&a.NoteCustomer, &a.CustomerNote, &tok, &sen, &sexp, &cover, &a.PhotoCount, &a.FinalCount, &a.LastScan,
		&sub, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	a.AllowDownload, a.ShareEnabled = dl == 1, sen == 1
	a.ExpiresAt, a.DueAt, a.ShareExpiresAt = nullInt(exp), nullInt(due), nullInt(sexp)
	a.CoverPhotoID, a.SubmittedAt, a.ShareToken = nullInt(cover), nullInt(sub), tok.String
	return &a, nil
}

func (a *App) loadAlbum(id int64) (*Album, error) {
	al, err := scanAlbum(a.db.QueryRow(`SELECT `+albumCols+` FROM albums WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, errNotFound()
	}
	return al, err
}

func (al *Album) expired() bool { return al.ExpiresAt > 0 && al.ExpiresAt < now() }

// customerView strips staff-only fields.
func (al *Album) customerView() *Album {
	c := *al
	c.Share, c.RelPath, c.NoteInternal, c.ShareToken, c.DueAt = "", "", "", "", 0
	c.ShareEnabled, c.ShareExpiresAt = false, 0
	return &c
}

// albumFor loads an album and checks that principal p may see it.
func (a *App) albumFor(p *Principal, id int64) (*Album, error) {
	al, err := a.loadAlbum(id)
	if err != nil {
		return nil, err
	}
	switch p.Type {
	case "staff":
		if p.Can(PermAlbumsAll) {
			return al, nil
		}
		var n int
		a.db.QueryRow(`SELECT COUNT(*) FROM album_editors WHERE album_id = ? AND staff_id = ?`, id, p.ID).Scan(&n)
		if n == 0 {
			return nil, errForbidden("Bạn không được gán vào album này")
		}
	case "customer":
		var n int
		a.db.QueryRow(`SELECT COUNT(*) FROM album_customers WHERE album_id = ? AND customer_id = ?`, id, p.ID).Scan(&n)
		if n == 0 {
			return nil, errNotFound()
		}
		if al.expired() {
			return nil, &apiError{403, "expired", "Album đã hết hạn truy cập"}
		}
	case "guest":
		if p.Album != id {
			return nil, errNotFound()
		}
	default:
		return nil, errUnauthorized()
	}
	return al, nil
}

// ---------- folder scanning ----------

var scanLocks sync.Map // album id -> *sync.Mutex

func (a *App) finalFolderName() string {
	return getSetting(a.db, "final_folder", "Final")
}

type diskFile struct {
	rel, name, kind string
	size, mtime     int64
}

// scanAlbum syncs the photos table with the album folder (and its Final sub-folder).
// Unless force is set, it is skipped when the last scan is less than a minute old.
func (a *App) scanAlbum(al *Album, force bool) error {
	if !force && now()-al.LastScan < 60 {
		return nil
	}
	lk, _ := scanLocks.LoadOrStore(al.ID, &sync.Mutex{})
	mu := lk.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	abs, err := a.resolve(al.Share, al.RelPath)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return errForbidden("Không đọc được thư mục album. Kiểm tra quyền share folder.")
	}
	finalName := a.finalFolderName()
	var files []diskFile
	add := func(dirRel, kind string, list []os.DirEntry) {
		for _, e := range list {
			if e.IsDir() || !isImageName(e.Name()) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, diskFile{path.Join(dirRel, e.Name()), e.Name(), kind, info.Size(), info.ModTime().Unix()})
		}
	}
	add("", "orig", entries)
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), finalName) {
			if sub, err := os.ReadDir(abs + string(os.PathSeparator) + e.Name()); err == nil {
				add(e.Name(), "final", sub)
			}
		}
	}

	type existing struct {
		id          int64
		size, mtime int64
	}
	have := map[string]existing{}
	rows, err := a.db.Query(`SELECT id, rel, size, mtime FROM photos WHERE album_id = ?`, al.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var e existing
		var rel string
		rows.Scan(&e.id, &rel, &e.size, &e.mtime)
		have[rel] = e
	}
	rows.Close()

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var queue []int64
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.rel] = true
		if e, ok := have[f.rel]; ok {
			if e.size != f.size || e.mtime != f.mtime {
				tx.Exec(`UPDATE photos SET size = ?, mtime = ?, thumb_ok = 0 WHERE id = ?`, f.size, f.mtime, e.id)
				a.thumbs.RemovePhoto(al.ID, e.id)
				queue = append(queue, e.id)
			}
			continue
		}
		res, err := tx.Exec(`INSERT INTO photos(album_id, rel, kind, name, size, mtime) VALUES(?,?,?,?,?,?)`,
			al.ID, f.rel, f.kind, f.name, f.size, f.mtime)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		queue = append(queue, id)
	}
	for rel, e := range have {
		if !seen[rel] {
			tx.Exec(`DELETE FROM photos WHERE id = ?`, e.id)
			a.thumbs.RemovePhoto(al.ID, e.id)
		}
	}
	tx.Exec(`UPDATE albums SET
		photo_count = (SELECT COUNT(*) FROM photos WHERE album_id = ?1 AND kind = 'orig'),
		final_count = (SELECT COUNT(*) FROM photos WHERE album_id = ?1 AND kind = 'final'),
		cover_photo_id = COALESCE((SELECT id FROM photos WHERE id = cover_photo_id),
			(SELECT id FROM photos WHERE album_id = ?1 AND kind = 'orig' ORDER BY name LIMIT 1)),
		last_scan = ?2 WHERE id = ?1`, al.ID, now())
	if err := tx.Commit(); err != nil {
		return err
	}
	for _, id := range queue {
		a.thumbs.Enqueue(id)
	}
	if len(queue) > 0 || len(have) != len(files) {
		a.hub.Album(al.ID, map[string]any{"type": "album_changed"})
	}
	return nil
}

// ---------- photo listing ----------

type PhotoJSON struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	W        int    `json:"w"`
	H        int    `json:"h"`
	Fav      bool   `json:"fav"`
	Edit     bool   `json:"edit"`
	Done     bool   `json:"done,omitempty"`
	Viewed   bool   `json:"viewed,omitempty"`
	Comments int    `json:"cm,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

func (a *App) listPhotos(albumID int64, withPicks bool) ([]PhotoJSON, error) {
	rows, err := a.db.Query(`SELECT p.id, p.name, p.kind, p.width, p.height, p.fav, p.edit_pick, p.edit_done, p.viewed, p.size,
		(SELECT COUNT(*) FROM comments c WHERE c.photo_id = p.id)
		FROM photos p WHERE p.album_id = ? ORDER BY p.kind DESC, p.name COLLATE NOCASE`, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PhotoJSON{}
	for rows.Next() {
		var p PhotoJSON
		var fav, edit, done, viewed int
		rows.Scan(&p.ID, &p.Name, &p.Kind, &p.W, &p.H, &fav, &edit, &done, &viewed, &p.Size, &p.Comments)
		if withPicks {
			p.Fav, p.Edit, p.Done, p.Viewed = fav == 1, edit == 1, done == 1, viewed == 1
		} else {
			p.Comments = 0
		}
		out = append(out, p)
	}
	return out, nil
}

// unreadFor counts comments from the other side that p has not read yet.
func (a *App) unreadFor(p *Principal, albumID int64) int {
	var last int64
	a.db.QueryRow(`SELECT last_comment_id FROM reads WHERE album_id = ? AND reader = ?`, albumID, p.readerKey()).Scan(&last)
	cond := `author_type = 'staff'`
	if p.Type == "staff" {
		cond = `author_type <> 'staff'`
	}
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM comments WHERE album_id = ? AND id > ? AND `+cond, albumID, last).Scan(&n)
	return n
}

func (a *App) notify(kind string, albumID int64, text string) {
	res, err := a.db.Exec(`INSERT INTO notifications(kind, album_id, text, created_at) VALUES(?,?,?,?)`, kind, albumID, text, now())
	if err != nil {
		return
	}
	id, _ := res.LastInsertId()
	a.hub.Staff(map[string]any{"type": "notification", "id": id, "kind": kind, "album_id": albumID, "text": text, "created_at": now()})
}

func (a *App) albumTitle(id int64) string {
	var t string
	a.db.QueryRow(`SELECT title FROM albums WHERE id = ?`, id).Scan(&t)
	return t
}
