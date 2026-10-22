package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

func (a *App) handleCustomerLogin(w http.ResponseWriter, r *http.Request) error {
	var in struct{ Phone, Password string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	phone := normalizePhone(in.Phone)
	if err := a.checkLoginAllowed(r, "c:"+phone); err != nil {
		return &apiError{429, "too_many", "Bạn nhập sai quá nhiều lần, vui lòng thử lại sau 15 phút"}
	}
	var id int64
	var hash string
	var active int
	err := a.db.QueryRow(`SELECT id, password_hash, active FROM customers WHERE phone = ?`, phone).Scan(&id, &hash, &active)
	if err != nil || active == 0 || !checkPassword(hash, in.Password) {
		a.loginFailed(r, "c:"+phone)
		return &apiError{401, "invalid_login", "Số điện thoại hoặc mật khẩu không đúng"}
	}
	a.limit.reset("acct:c:" + phone)
	a.db.Exec(`UPDATE customers SET last_login = ? WHERE id = ?`, now(), id)
	if err := a.createSession(w, r, "customer", id); err != nil {
		return err
	}
	return ok(w)
}

func (a *App) handleCustomerLogout(w http.ResponseWriter, r *http.Request) error {
	a.destroySession(w, r, cookieCustomer)
	return ok(w)
}

func (a *App) handleCustomerMe(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var phone, lang string
	a.db.QueryRow(`SELECT phone, lang FROM customers WHERE id = ?`, p.ID).Scan(&phone, &lang)
	writeJSON(w, 200, map[string]any{"id": p.ID, "name": p.Name, "phone": phone, "lang": lang})
	return nil
}

func (a *App) handleCustomerLang(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in struct{ Lang string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Lang != "vi" && in.Lang != "en" {
		return errBad("bad_lang", "lang")
	}
	a.db.Exec(`UPDATE customers SET lang = ? WHERE id = ?`, in.Lang, p.ID)
	return ok(w)
}

func (a *App) handleCustomerPassword(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in struct{ Old, New string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var hash string
	a.db.QueryRow(`SELECT password_hash FROM customers WHERE id = ?`, p.ID).Scan(&hash)
	if !checkPassword(hash, in.Old) {
		return errBad("wrong_password", "Mật khẩu hiện tại không đúng")
	}
	if len(in.New) < 6 {
		return errBad("weak_password", "Mật khẩu mới phải có ít nhất 6 ký tự")
	}
	h, _ := hashPassword(in.New)
	a.db.Exec(`UPDATE customers SET password_hash = ? WHERE id = ?`, h, p.ID)
	return ok(w)
}

func (a *App) handleCustomerAlbums(w http.ResponseWriter, r *http.Request, p *Principal) error {
	rows, err := a.db.Query(`SELECT `+prefixCols("a.", albumCols)+` FROM albums a
		JOIN album_customers ac ON ac.album_id = a.id WHERE ac.customer_id = ? ORDER BY a.created_at DESC`, p.ID)
	if err != nil {
		return err
	}
	var list []*Album
	for rows.Next() {
		al, err := scanAlbum(rows)
		if err == nil {
			list = append(list, al)
		}
	}
	rows.Close()
	out := []map[string]any{}
	for _, al := range list {
		var fav, edit int
		a.db.QueryRow(`SELECT COALESCE(SUM(fav),0), COALESCE(SUM(edit_pick),0) FROM photos WHERE album_id = ?`, al.ID).Scan(&fav, &edit)
		out = append(out, map[string]any{
			"album": al.customerView(), "expired": al.expired(), "fav_count": fav, "edit_count": edit,
			"unread": a.unreadFor(p, al.ID),
		})
	}
	writeJSON(w, 200, out)
	return nil
}

func prefixCols(prefix, cols string) string {
	parts := strings.Split(cols, ",")
	for i, c := range parts {
		parts[i] = prefix + strings.TrimSpace(c)
	}
	return strings.Join(parts, ", ")
}

func (a *App) handleCustomerAlbum(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	if err := a.scanAlbum(al, false); err != nil {
		return err
	}
	al, _ = a.loadAlbum(al.ID)
	photos, err := a.listPhotos(al.ID, true)
	if err != nil {
		return err
	}
	writeJSON(w, 200, map[string]any{
		"album": al.customerView(), "photos": photos, "unread": a.unreadFor(p, al.ID),
		"locked": al.Status != StatusSelecting,
	})
	return nil
}

// handleCustomerPick toggles the heart (fav) and/or the edit selection of a photo.
func (a *App) handleCustomerPick(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	if al.Status != StatusSelecting {
		return errConflict("locked", "Album đã gửi cho studio, không thể thay đổi lựa chọn")
	}
	var in struct {
		Fav  *bool `json:"fav"`
		Edit *bool `json:"edit"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	pid := pathID(r, "pid")
	var fav, edit int
	var kind string
	if err := a.db.QueryRow(`SELECT fav, edit_pick, kind FROM photos WHERE id = ? AND album_id = ?`, pid, al.ID).Scan(&fav, &edit, &kind); err != nil {
		return errNotFound()
	}
	if kind != "orig" {
		return errBad("not_selectable", "Ảnh này không thể chọn")
	}
	if in.Fav != nil {
		fav = b2i(*in.Fav)
		if fav == 0 {
			edit = 0
		}
	}
	if in.Edit != nil {
		edit = b2i(*in.Edit)
		if edit == 1 {
			fav = 1
			if al.MaxEdit > 0 {
				var n int
				a.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE album_id = ? AND edit_pick = 1 AND id <> ?`, al.ID, pid).Scan(&n)
				if n >= al.MaxEdit {
					return errConflict("limit", fmt.Sprintf("Bạn chỉ được chọn tối đa %d ảnh để chỉnh sửa", al.MaxEdit))
				}
			}
		}
	}
	a.db.Exec(`UPDATE photos SET fav = ?, edit_pick = ?, pick_by = ?, viewed = 1 WHERE id = ?`, fav, edit, p.Name, pid)
	a.db.Exec(`UPDATE albums SET updated_at = ? WHERE id = ?`, now(), al.ID)
	ev := map[string]any{"type": "photo", "id": pid, "fav": fav == 1, "edit": edit == 1}
	a.hub.Album(al.ID, ev)
	writeJSON(w, 200, ev)
	return nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (a *App) handleCustomerViewed(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in struct{ IDs []int64 }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	for _, id := range in.IDs {
		a.db.Exec(`UPDATE photos SET viewed = 1 WHERE id = ? AND album_id = ?`, id, al.ID)
	}
	return ok(w)
}

func (a *App) handleCustomerSubmit(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	if al.Status != StatusSelecting {
		return errConflict("locked", "Album đã được gửi trước đó")
	}
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE album_id = ? AND edit_pick = 1`, al.ID).Scan(&n)
	if n == 0 {
		return errBad("empty", "Bạn chưa chọn ảnh nào để chỉnh sửa")
	}
	a.db.Exec(`UPDATE albums SET status = ?, submitted_at = ?, updated_at = ? WHERE id = ?`, StatusSubmitted, now(), now(), al.ID)
	a.hub.Album(al.ID, map[string]any{"type": "status", "status": StatusSubmitted})
	a.notify("submitted", al.ID, fmt.Sprintf("%s đã gửi %d ảnh chọn edit — %s", p.Name, n, al.Title))
	return ok(w)
}

func (a *App) handleCustomerNote(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in struct{ Text string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if len(in.Text) > 5000 {
		return errBad("too_long", "Ghi chú quá dài")
	}
	a.db.Exec(`UPDATE albums SET customer_note = ?, updated_at = ? WHERE id = ?`, in.Text, now(), al.ID)
	a.hub.Album(al.ID, map[string]any{"type": "customer_note", "text": in.Text})
	if strings.TrimSpace(in.Text) != "" {
		a.notify("note", al.ID, fmt.Sprintf("%s cập nhật ghi chú chung — %s", p.Name, al.Title))
	}
	return ok(w)
}

// ---------- guest (public link) ----------

func (a *App) handleGuestAlbum(w http.ResponseWriter, r *http.Request) error {
	g := a.guestFromToken(r.PathValue("token"))
	if g == nil {
		return errNotFound()
	}
	al, err := a.loadAlbum(g.Album)
	if err != nil {
		return err
	}
	if err := a.scanAlbum(al, false); err != nil {
		return err
	}
	photos, err := a.listPhotos(al.ID, false)
	if err != nil {
		return err
	}
	v := al.customerView()
	v.CustomerNote = ""
	writeJSON(w, 200, map[string]any{"album": v, "photos": photos})
	return nil
}

// ---------- shared comment handlers (customer & staff) ----------

type Comment struct {
	ID         int64  `json:"id"`
	AlbumID    int64  `json:"album_id"`
	PhotoID    int64  `json:"photo_id"`
	PhotoName  string `json:"photo_name,omitempty"`
	AuthorType string `json:"author_type"`
	AuthorName string `json:"author_name"`
	Body       string `json:"body"`
	CreatedAt  int64  `json:"created_at"`
	Mine       bool   `json:"mine,omitempty"`
}

// handleListComments: ?photo=ID for one photo, ?scope=chat for the album chat, ?scope=all for everything.
func (a *App) handleListComments(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	q := `SELECT c.id, c.album_id, COALESCE(c.photo_id, 0), COALESCE(ph.name, ''), c.author_type, c.author_id, c.author_name, c.body, c.created_at
		FROM comments c LEFT JOIN photos ph ON ph.id = c.photo_id WHERE c.album_id = ?`
	args := []any{al.ID}
	switch {
	case r.URL.Query().Get("photo") != "":
		q += ` AND c.photo_id = ?`
		args = append(args, r.URL.Query().Get("photo"))
	case r.URL.Query().Get("scope") == "chat":
		q += ` AND c.photo_id IS NULL`
	}
	q += ` ORDER BY c.id`
	rows, err := a.db.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		var authorID int64
		rows.Scan(&c.ID, &c.AlbumID, &c.PhotoID, &c.PhotoName, &c.AuthorType, &authorID, &c.AuthorName, &c.Body, &c.CreatedAt)
		c.Mine = c.AuthorType == p.Type && authorID == p.ID
		out = append(out, c)
	}
	writeJSON(w, 200, out)
	return nil
}

func (a *App) handlePostComment(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in struct {
		PhotoID int64  `json:"photo_id"`
		Body    string `json:"body"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || len(in.Body) > 4000 {
		return errBad("bad_body", "Nội dung không hợp lệ")
	}
	var photo any
	photoName := ""
	if in.PhotoID > 0 {
		if err := a.db.QueryRow(`SELECT name FROM photos WHERE id = ? AND album_id = ?`, in.PhotoID, al.ID).Scan(&photoName); err != nil {
			return errNotFound()
		}
		photo = in.PhotoID
	}
	name := p.Name
	if p.Type == "staff" {
		name = getSetting(a.db, "studio_name", "Studio") + " · " + p.Name
	}
	res, err := a.db.Exec(`INSERT INTO comments(album_id, photo_id, author_type, author_id, author_name, body, created_at) VALUES(?,?,?,?,?,?,?)`,
		al.ID, photo, p.Type, p.ID, name, in.Body, now())
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	c := Comment{ID: id, AlbumID: al.ID, PhotoID: in.PhotoID, PhotoName: photoName, AuthorType: p.Type, AuthorName: name, Body: in.Body, CreatedAt: now()}
	// The sender has obviously read everything up to their own message.
	a.db.Exec(`INSERT INTO reads(album_id, reader, last_comment_id) VALUES(?,?,?)
		ON CONFLICT(album_id, reader) DO UPDATE SET last_comment_id = excluded.last_comment_id`, al.ID, p.readerKey(), id)
	a.hub.Album(al.ID, map[string]any{"type": "comment", "comment": c})
	if p.Type == "customer" {
		where := "chat"
		if photoName != "" {
			where = "ảnh " + photoName
		}
		a.notify("message", al.ID, fmt.Sprintf("%s (%s): %s", p.Name, where, truncate(in.Body, 80)))
	}
	c.Mine = true
	writeJSON(w, 200, c)
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (a *App) handleMarkRead(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var last int64
	a.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM comments WHERE album_id = ?`, al.ID).Scan(&last)
	_, err = a.db.Exec(`INSERT INTO reads(album_id, reader, last_comment_id) VALUES(?,?,?)
		ON CONFLICT(album_id, reader) DO UPDATE SET last_comment_id = excluded.last_comment_id`, al.ID, p.readerKey(), last)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if p.Type == "staff" {
		a.hub.Staff(map[string]any{"type": "read", "album_id": al.ID})
	}
	return ok(w)
}
