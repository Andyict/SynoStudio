package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) handleStaffLogin(w http.ResponseWriter, r *http.Request) error {
	var in struct{ Username, Password string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	u := strings.TrimSpace(in.Username)
	if err := a.checkLoginAllowed(r, "s:"+strings.ToLower(u)); err != nil {
		return &apiError{429, "too_many", "Sai quá nhiều lần, thử lại sau 15 phút"}
	}
	var id int64
	var hash string
	var active int
	err := a.db.QueryRow(`SELECT id, password_hash, active FROM staff WHERE username = ?`, u).Scan(&id, &hash, &active)
	if err != nil || active == 0 || !checkPassword(hash, in.Password) {
		a.loginFailed(r, "s:"+strings.ToLower(u))
		return &apiError{401, "invalid_login", "Tên đăng nhập hoặc mật khẩu không đúng"}
	}
	a.db.Exec(`UPDATE staff SET last_login = ? WHERE id = ?`, now(), id)
	if err := a.createSession(w, r, "staff", id); err != nil {
		return err
	}
	return ok(w)
}

func (a *App) handleStaffLogout(w http.ResponseWriter, r *http.Request) error {
	a.destroySession(w, r, cookieStaff)
	return ok(w)
}

func (a *App) handleStaffMe(w http.ResponseWriter, r *http.Request, p *Principal) error {
	perms := map[string]bool{}
	for _, k := range AllPerms {
		perms[k] = p.Can(k)
	}
	var username string
	a.db.QueryRow(`SELECT username FROM staff WHERE id = ?`, p.ID).Scan(&username)
	writeJSON(w, 200, map[string]any{"id": p.ID, "name": p.Name, "username": username, "owner": p.IsOwner, "perms": perms})
	return nil
}

func (a *App) handleStaffPassword(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in struct{ Old, New string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var hash string
	a.db.QueryRow(`SELECT password_hash FROM staff WHERE id = ?`, p.ID).Scan(&hash)
	if !checkPassword(hash, in.Old) {
		return errBad("wrong_password", "Mật khẩu hiện tại không đúng")
	}
	if len(in.New) < 6 {
		return errBad("weak_password", "Mật khẩu mới phải có ít nhất 6 ký tự")
	}
	h, _ := hashPassword(in.New)
	a.db.Exec(`UPDATE staff SET password_hash = ? WHERE id = ?`, h, p.ID)
	return ok(w)
}

// visibleAlbumFilter returns a SQL condition limiting albums to those p may see.
func visibleAlbumFilter(p *Principal) (string, []any) {
	if p.Can(PermAlbumsAll) {
		return "1=1", nil
	}
	return "a.id IN (SELECT album_id FROM album_editors WHERE staff_id = ?)", []any{p.ID}
}

type albumSummary struct {
	*Album
	FavCount  int      `json:"fav_count"`
	EditCount int      `json:"edit_count"`
	DoneCount int      `json:"done_count"`
	Unread    int      `json:"unread"`
	Customers []string `json:"customers"`
	Editors   []string `json:"editors"`
}

func (a *App) summarize(p *Principal, al *Album) albumSummary {
	s := albumSummary{Album: al, Customers: []string{}, Editors: []string{}}
	a.db.QueryRow(`SELECT COALESCE(SUM(fav),0), COALESCE(SUM(edit_pick),0), COALESCE(SUM(edit_pick*edit_done),0) FROM photos WHERE album_id = ?`, al.ID).
		Scan(&s.FavCount, &s.EditCount, &s.DoneCount)
	s.Unread = a.unreadFor(p, al.ID)
	rows, _ := a.db.Query(`SELECT CASE WHEN c.name <> '' THEN c.name || ' (' || c.phone || ')' ELSE c.phone END
		FROM album_customers ac JOIN customers c ON c.id = ac.customer_id WHERE ac.album_id = ?`, al.ID)
	for rows != nil && rows.Next() {
		var n string
		rows.Scan(&n)
		s.Customers = append(s.Customers, n)
	}
	if rows != nil {
		rows.Close()
	}
	rows, _ = a.db.Query(`SELECT CASE WHEN s.display_name <> '' THEN s.display_name ELSE s.username END
		FROM album_editors ae JOIN staff s ON s.id = ae.staff_id WHERE ae.album_id = ?`, al.ID)
	for rows != nil && rows.Next() {
		var n string
		rows.Scan(&n)
		s.Editors = append(s.Editors, n)
	}
	if rows != nil {
		rows.Close()
	}
	return s
}

func (a *App) queryAlbums(p *Principal, extra string, args ...any) ([]*Album, error) {
	cond, cargs := visibleAlbumFilter(p)
	q := `SELECT ` + prefixCols("a.", albumCols) + ` FROM albums a WHERE ` + cond
	if extra != "" {
		q += " AND " + extra
	}
	rows, err := a.db.Query(q+` ORDER BY a.updated_at DESC LIMIT 500`, append(cargs, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Album
	for rows.Next() {
		if al, err := scanAlbum(rows); err == nil {
			out = append(out, al)
		}
	}
	return out, nil
}

func (a *App) handleStaffAlbums(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var conds []string
	var args []any
	if st := r.URL.Query().Get("status"); validStatus[st] {
		conds = append(conds, "a.status = ?")
		args = append(args, st)
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		like := "%" + q + "%"
		conds = append(conds, `(a.title LIKE ? OR a.rel_path LIKE ? OR a.id IN (SELECT ac.album_id FROM album_customers ac
			JOIN customers c ON c.id = ac.customer_id WHERE c.name LIKE ? OR c.phone LIKE ?))`)
		args = append(args, like, like, like, "%"+normalizePhone(q)+"%")
	}
	list, err := a.queryAlbums(p, strings.Join(conds, " AND "), args...)
	if err != nil {
		return err
	}
	out := []albumSummary{}
	for _, al := range list {
		out = append(out, a.summarize(p, al))
	}
	writeJSON(w, 200, out)
	return nil
}

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request, p *Principal) error {
	counts := map[string]int{}
	cond, cargs := visibleAlbumFilter(p)
	rows, err := a.db.Query(`SELECT a.status, COUNT(*) FROM albums a WHERE `+cond+` GROUP BY a.status`, cargs...)
	if err == nil {
		for rows.Next() {
			var s string
			var n int
			rows.Scan(&s, &n)
			counts[s] = n
		}
		rows.Close()
	}
	submitted, _ := a.queryAlbums(p, "a.status = ?", StatusSubmitted)
	due, _ := a.queryAlbums(p, "a.due_at IS NOT NULL AND a.due_at < ? AND a.status <> ?", now()+3*86400, StatusDelivered)
	all, _ := a.queryAlbums(p, "")
	var unread []albumSummary
	for _, al := range all {
		if a.unreadFor(p, al.ID) > 0 {
			unread = append(unread, a.summarize(p, al))
		}
	}
	sum := func(list []*Album) []albumSummary {
		out := []albumSummary{}
		for _, al := range list {
			out = append(out, a.summarize(p, al))
		}
		return out
	}
	if unread == nil {
		unread = []albumSummary{}
	}
	writeJSON(w, 200, map[string]any{"counts": counts, "submitted": sum(submitted), "due": sum(due), "unread": unread})
	return nil
}

func (a *App) handleNotifications(w http.ResponseWriter, r *http.Request, p *Principal) error {
	cond, cargs := visibleAlbumFilter(p)
	rows, err := a.db.Query(`SELECT n.id, n.kind, COALESCE(n.album_id,0), n.text, n.created_at FROM notifications n
		JOIN albums a ON a.id = n.album_id WHERE `+cond+` ORDER BY n.id DESC LIMIT 50`, cargs...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var seen int64
	a.db.QueryRow(`SELECT notif_seen_id FROM staff WHERE id = ?`, p.ID).Scan(&seen)
	list := []map[string]any{}
	unseen := 0
	for rows.Next() {
		var id, album, at int64
		var kind, text string
		rows.Scan(&id, &kind, &album, &text, &at)
		if id > seen {
			unseen++
		}
		list = append(list, map[string]any{"id": id, "kind": kind, "album_id": album, "text": text, "created_at": at, "new": id > seen})
	}
	writeJSON(w, 200, map[string]any{"items": list, "unseen": unseen})
	return nil
}

func (a *App) handleNotificationsSeen(w http.ResponseWriter, r *http.Request, p *Principal) error {
	a.db.Exec(`UPDATE staff SET notif_seen_id = (SELECT COALESCE(MAX(id),0) FROM notifications) WHERE id = ?`, p.ID)
	return ok(w)
}

// ---------- album CRUD ----------

type albumInput struct {
	Title         string  `json:"title"`
	Share         string  `json:"share"`
	RelPath       string  `json:"rel_path"`
	MaxEdit       int     `json:"max_edit"`
	AllowDownload bool    `json:"allow_download"`
	ExpiresAt     int64   `json:"expires_at"`
	DueAt         int64   `json:"due_at"`
	NoteInternal  string  `json:"note_internal"`
	NoteCustomer  string  `json:"note_customer"`
	CustomerIDs   []int64 `json:"customer_ids"`
	EditorIDs     []int64 `json:"editor_ids"`
}

func (in *albumInput) validate() error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return errBad("title", "Vui lòng nhập tên album")
	}
	if in.MaxEdit < 0 || in.MaxEdit > 100000 {
		return errBad("max_edit", "Số ảnh edit không hợp lệ")
	}
	return nil
}

func (a *App) setAlbumPeople(tx *sql.Tx, albumID int64, in *albumInput) {
	tx.Exec(`DELETE FROM album_customers WHERE album_id = ?`, albumID)
	for _, c := range in.CustomerIDs {
		tx.Exec(`INSERT OR IGNORE INTO album_customers(album_id, customer_id) VALUES(?,?)`, albumID, c)
	}
	tx.Exec(`DELETE FROM album_editors WHERE album_id = ?`, albumID)
	for _, s := range in.EditorIDs {
		tx.Exec(`INSERT OR IGNORE INTO album_editors(album_id, staff_id) VALUES(?,?)`, albumID, s)
	}
}

func (a *App) handleCreateAlbum(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in albumInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	rel, okp := cleanRel(in.RelPath)
	if !okp {
		return errBad("bad_path", "Đường dẫn không hợp lệ")
	}
	if _, err := a.resolve(in.Share, rel); err != nil {
		return err
	}
	var dup int64
	if a.db.QueryRow(`SELECT id FROM albums WHERE share = ? AND rel_path = ?`, in.Share, rel).Scan(&dup) == nil {
		return errConflict("duplicate", fmt.Sprintf("Thư mục này đã được dùng cho album #%d", dup))
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO albums(title, share, rel_path, max_edit, allow_download, expires_at, due_at, note_internal,
		note_customer, created_by, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.Title, in.Share, rel, in.MaxEdit, b2i(in.AllowDownload), toNull(in.ExpiresAt), toNull(in.DueAt),
		in.NoteInternal, in.NoteCustomer, p.ID, now(), now())
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	a.setAlbumPeople(tx, id, &in)
	if err := tx.Commit(); err != nil {
		return err
	}
	al, _ := a.loadAlbum(id)
	if err := a.scanAlbum(al, true); err != nil {
		return err
	}
	writeJSON(w, 200, map[string]any{"id": id})
	return nil
}

func (a *App) handleStaffAlbum(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	if err := a.scanAlbum(al, false); err != nil {
		writeJSON(w, 200, map[string]any{"album": a.summarize(p, al), "photos": []PhotoJSON{}, "scan_error": err.Error()})
		return nil
	}
	al, _ = a.loadAlbum(al.ID)
	photos, err := a.listPhotos(al.ID, true)
	if err != nil {
		return err
	}
	var customerIDs, editorIDs []int64
	rows, _ := a.db.Query(`SELECT customer_id FROM album_customers WHERE album_id = ?`, al.ID)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		customerIDs = append(customerIDs, id)
	}
	rows.Close()
	rows, _ = a.db.Query(`SELECT staff_id FROM album_editors WHERE album_id = ?`, al.ID)
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		editorIDs = append(editorIDs, id)
	}
	rows.Close()
	writeJSON(w, 200, map[string]any{
		"album": a.summarize(p, al), "photos": photos, "customer_ids": nonNil(customerIDs), "editor_ids": nonNil(editorIDs),
		"public_url": getSetting(a.db, "public_url", ""),
	})
	return nil
}

func nonNil(s []int64) []int64 {
	if s == nil {
		return []int64{}
	}
	return s
}

func (a *App) handleUpdateAlbum(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in albumInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if err := in.validate(); err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE albums SET title=?, max_edit=?, allow_download=?, expires_at=?, due_at=?, note_internal=?,
		note_customer=?, updated_at=? WHERE id=?`, in.Title, in.MaxEdit, b2i(in.AllowDownload), toNull(in.ExpiresAt),
		toNull(in.DueAt), in.NoteInternal, in.NoteCustomer, now(), al.ID)
	if err != nil {
		return err
	}
	a.setAlbumPeople(tx, al.ID, &in)
	if err := tx.Commit(); err != nil {
		return err
	}
	a.hub.Album(al.ID, map[string]any{"type": "album_changed"})
	return ok(w)
}

func (a *App) handleDeleteAlbum(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	// Only database records and cached thumbnails are removed; the photo files stay on the NAS.
	if _, err := a.db.Exec(`DELETE FROM albums WHERE id = ?`, al.ID); err != nil {
		return err
	}
	a.thumbs.RemoveAlbum(al.ID)
	return ok(w)
}

func (a *App) handleAlbumStatus(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in struct{ Status string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if !validStatus[in.Status] {
		return errBad("bad_status", "Trạng thái không hợp lệ")
	}
	if !p.Can(PermAlbumsUnlock) {
		if !p.Can(PermEditWork) || (in.Status != StatusEditing && in.Status != StatusDelivered) {
			return errForbidden("Bạn không có quyền đổi trạng thái này")
		}
	}
	a.db.Exec(`UPDATE albums SET status = ?, updated_at = ? WHERE id = ?`, in.Status, now(), al.ID)
	a.hub.Album(al.ID, map[string]any{"type": "status", "status": in.Status})
	return ok(w)
}

func (a *App) handleRescan(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	if err := a.scanAlbum(al, true); err != nil {
		return err
	}
	return ok(w)
}

func (a *App) handleShareLink(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in struct {
		Enabled    bool  `json:"enabled"`
		Regenerate bool  `json:"regenerate"`
		ExpiresAt  int64 `json:"expires_at"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	tok := al.ShareToken
	if tok == "" || in.Regenerate {
		tok = randomToken(16)
	}
	a.db.Exec(`UPDATE albums SET share_token = ?, share_enabled = ?, share_expires_at = ? WHERE id = ?`,
		tok, b2i(in.Enabled), toNull(in.ExpiresAt), al.ID)
	writeJSON(w, 200, map[string]any{"share_token": tok, "share_enabled": in.Enabled, "share_expires_at": in.ExpiresAt})
	return nil
}

func (a *App) handleEditDone(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	var in struct{ On bool }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	pid := pathID(r, "pid")
	res, _ := a.db.Exec(`UPDATE photos SET edit_done = ? WHERE id = ? AND album_id = ?`, b2i(in.On), pid, al.ID)
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound()
	}
	a.hub.Album(al.ID, map[string]any{"type": "done", "id": pid, "done": in.On})
	return ok(w)
}

// ---------- customers ----------

func (a *App) handleListCustomers(w http.ResponseWriter, r *http.Request, p *Principal) error {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	sqlq := `SELECT c.id, c.phone, c.name, c.note, c.active, c.created_at, COALESCE(c.last_login,0),
		(SELECT COUNT(*) FROM album_customers ac WHERE ac.customer_id = c.id),
		COALESCE((SELECT GROUP_CONCAT(album_id) FROM album_customers ac WHERE ac.customer_id = c.id), '') FROM customers c`
	var args []any
	if q != "" {
		sqlq += ` WHERE c.name LIKE ? OR c.phone LIKE ?`
		args = append(args, "%"+q+"%", "%"+normalizePhone(q)+"%")
	}
	rows, err := a.db.Query(sqlq+` ORDER BY c.created_at DESC LIMIT 1000`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, created, last int64
		var phone, name, note, albumList string
		var active, albums int
		rows.Scan(&id, &phone, &name, &note, &active, &created, &last, &albums, &albumList)
		ids := []int64{}
		for _, s := range strings.Split(albumList, ",") {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				ids = append(ids, n)
			}
		}
		out = append(out, map[string]any{"id": id, "phone": phone, "name": name, "note": note, "active": active == 1,
			"created_at": created, "last_login": last, "albums": albums, "album_ids": ids})
	}
	writeJSON(w, 200, out)
	return nil
}

type customerInput struct {
	Phone    string  `json:"phone"`
	Name     string  `json:"name"`
	Password string  `json:"password"`
	Note     string  `json:"note"`
	Active   *bool   `json:"active"`
	AlbumIDs []int64 `json:"album_ids"`
}

func (a *App) handleCreateCustomer(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in customerInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	phone := normalizePhone(in.Phone)
	if len(phone) < 8 {
		return errBad("phone", "Số điện thoại không hợp lệ")
	}
	pw := in.Password
	if pw == "" {
		pw = randomPassword()
	} else if len(pw) < 6 {
		return errBad("weak_password", "Mật khẩu phải có ít nhất 6 ký tự")
	}
	h, _ := hashPassword(pw)
	res, err := a.db.Exec(`INSERT INTO customers(phone, name, password_hash, note, created_at) VALUES(?,?,?,?,?)`,
		phone, strings.TrimSpace(in.Name), h, in.Note, now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errConflict("duplicate", "Số điện thoại đã tồn tại")
		}
		return err
	}
	id, _ := res.LastInsertId()
	cond, cargs := visibleAlbumFilter(p)
	for _, al := range in.AlbumIDs {
		a.db.Exec(`INSERT OR IGNORE INTO album_customers(album_id, customer_id)
			SELECT a.id, ? FROM albums a WHERE a.id = ? AND `+cond, append([]any{id, al}, cargs...)...)
	}
	writeJSON(w, 200, map[string]any{"id": id, "phone": phone, "password": pw})
	return nil
}

func (a *App) handleUpdateCustomer(w http.ResponseWriter, r *http.Request, p *Principal) error {
	id := pathID(r, "id")
	var in customerInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	phone := normalizePhone(in.Phone)
	if len(phone) < 8 {
		return errBad("phone", "Số điện thoại không hợp lệ")
	}
	active := 1
	if in.Active != nil && !*in.Active {
		active = 0
	}
	res, err := a.db.Exec(`UPDATE customers SET phone=?, name=?, note=?, active=? WHERE id=?`, phone, strings.TrimSpace(in.Name), in.Note, active, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errConflict("duplicate", "Số điện thoại đã tồn tại")
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound()
	}
	if in.Password != "" {
		if len(in.Password) < 6 {
			return errBad("weak_password", "Mật khẩu phải có ít nhất 6 ký tự")
		}
		h, _ := hashPassword(in.Password)
		a.db.Exec(`UPDATE customers SET password_hash = ? WHERE id = ?`, h, id)
		a.db.Exec(`DELETE FROM sessions WHERE user_type = 'customer' AND user_id = ?`, id)
	}
	if in.AlbumIDs != nil {
		// Only touch albums this staff member can see; assignments to other albums are kept.
		cond, cargs := visibleAlbumFilter(p)
		a.db.Exec(`DELETE FROM album_customers WHERE customer_id = ? AND album_id IN (SELECT a.id FROM albums a WHERE `+cond+`)`,
			append([]any{id}, cargs...)...)
		for _, al := range in.AlbumIDs {
			a.db.Exec(`INSERT OR IGNORE INTO album_customers(album_id, customer_id)
				SELECT a.id, ? FROM albums a WHERE a.id = ? AND `+cond, append([]any{id, al}, cargs...)...)
		}
	}
	if active == 0 {
		a.db.Exec(`DELETE FROM sessions WHERE user_type = 'customer' AND user_id = ?`, id)
	}
	return ok(w)
}

func (a *App) handleDeleteCustomer(w http.ResponseWriter, r *http.Request, p *Principal) error {
	id := pathID(r, "id")
	a.db.Exec(`DELETE FROM sessions WHERE user_type = 'customer' AND user_id = ?`, id)
	if _, err := a.db.Exec(`DELETE FROM customers WHERE id = ?`, id); err != nil {
		return err
	}
	return ok(w)
}

func (a *App) handleResetCustomerPassword(w http.ResponseWriter, r *http.Request, p *Principal) error {
	id := pathID(r, "id")
	pw := randomPassword()
	h, _ := hashPassword(pw)
	res, _ := a.db.Exec(`UPDATE customers SET password_hash = ? WHERE id = ?`, h, id)
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound()
	}
	a.db.Exec(`DELETE FROM sessions WHERE user_type = 'customer' AND user_id = ?`, id)
	writeJSON(w, 200, map[string]any{"password": pw})
	return nil
}

// ---------- staff & roles ----------

func (a *App) handleListStaff(w http.ResponseWriter, r *http.Request, p *Principal) error {
	rows, err := a.db.Query(`SELECT s.id, s.username, s.display_name, COALESCE(s.role_id,0), COALESCE(r.name,''), s.is_owner, s.active,
		COALESCE(s.last_login,0) FROM staff s LEFT JOIN roles r ON r.id = s.role_id ORDER BY s.is_owner DESC, s.username`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, roleID, last int64
		var username, display, role string
		var owner, active int
		rows.Scan(&id, &username, &display, &roleID, &role, &owner, &active, &last)
		perms := []string{}
		if owner == 1 {
			perms = AllPerms
		} else {
			var pj string
			a.db.QueryRow(`SELECT perms FROM roles WHERE id = ?`, roleID).Scan(&pj)
			json.Unmarshal([]byte(pj), &perms)
		}
		out = append(out, map[string]any{"id": id, "username": username, "display_name": display, "role_id": roleID,
			"role": role, "owner": owner == 1, "active": active == 1, "last_login": last, "perms": perms})
	}
	writeJSON(w, 200, out)
	return nil
}

type staffInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	RoleID      int64  `json:"role_id"`
	Active      *bool  `json:"active"`
}

func (a *App) handleCreateStaff(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in staffInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) < 3 {
		return errBad("username", "Tên đăng nhập phải có ít nhất 3 ký tự")
	}
	if len(in.Password) < 6 {
		return errBad("weak_password", "Mật khẩu phải có ít nhất 6 ký tự")
	}
	h, _ := hashPassword(in.Password)
	res, err := a.db.Exec(`INSERT INTO staff(username, password_hash, display_name, role_id, created_at) VALUES(?,?,?,?,?)`,
		in.Username, h, strings.TrimSpace(in.DisplayName), toNull(in.RoleID), now())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errConflict("duplicate", "Tên đăng nhập đã tồn tại")
		}
		return err
	}
	id, _ := res.LastInsertId()
	writeJSON(w, 200, map[string]any{"id": id})
	return nil
}

func (a *App) handleUpdateStaff(w http.ResponseWriter, r *http.Request, p *Principal) error {
	id := pathID(r, "id")
	var in staffInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var owner int
	if err := a.db.QueryRow(`SELECT is_owner FROM staff WHERE id = ?`, id).Scan(&owner); err != nil {
		return errNotFound()
	}
	if owner == 1 && !p.IsOwner {
		return errForbidden("Chỉ chủ tài khoản mới sửa được tài khoản chủ")
	}
	active := 1
	if in.Active != nil && !*in.Active && owner == 0 {
		active = 0
	}
	if _, err := a.db.Exec(`UPDATE staff SET display_name = ?, role_id = ?, active = ? WHERE id = ?`,
		strings.TrimSpace(in.DisplayName), toNull(in.RoleID), active, id); err != nil {
		return err
	}
	if in.Password != "" {
		if len(in.Password) < 6 {
			return errBad("weak_password", "Mật khẩu phải có ít nhất 6 ký tự")
		}
		h, _ := hashPassword(in.Password)
		a.db.Exec(`UPDATE staff SET password_hash = ? WHERE id = ?`, h, id)
		if id != p.ID {
			a.db.Exec(`DELETE FROM sessions WHERE user_type = 'staff' AND user_id = ?`, id)
		}
	}
	if active == 0 {
		a.db.Exec(`DELETE FROM sessions WHERE user_type = 'staff' AND user_id = ?`, id)
	}
	return ok(w)
}

func (a *App) handleDeleteStaff(w http.ResponseWriter, r *http.Request, p *Principal) error {
	id := pathID(r, "id")
	if id == p.ID {
		return errBad("self", "Không thể xóa chính mình")
	}
	var owner int
	if err := a.db.QueryRow(`SELECT is_owner FROM staff WHERE id = ?`, id).Scan(&owner); err != nil {
		return errNotFound()
	}
	if owner == 1 {
		return errForbidden("Không thể xóa tài khoản chủ (đổi trong Console)")
	}
	a.db.Exec(`DELETE FROM sessions WHERE user_type = 'staff' AND user_id = ?`, id)
	a.db.Exec(`DELETE FROM staff WHERE id = ?`, id)
	return ok(w)
}

func (a *App) handleListRoles(w http.ResponseWriter, r *http.Request, p *Principal) error {
	rows, err := a.db.Query(`SELECT r.id, r.name, r.perms, (SELECT COUNT(*) FROM staff s WHERE s.role_id = r.id) FROM roles r ORDER BY r.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, pj string
		var n int
		rows.Scan(&id, &name, &pj, &n)
		perms := []string{}
		json.Unmarshal([]byte(pj), &perms)
		out = append(out, map[string]any{"id": id, "name": name, "perms": perms, "staff_count": n})
	}
	writeJSON(w, 200, map[string]any{"roles": out, "all_perms": AllPerms})
	return nil
}

func (a *App) handleSaveRole(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in struct {
		Name  string   `json:"name"`
		Perms []string `json:"perms"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errBad("name", "Vui lòng nhập tên nhóm quyền")
	}
	valid := map[string]bool{}
	for _, k := range AllPerms {
		valid[k] = true
	}
	clean := []string{}
	for _, k := range in.Perms {
		if valid[k] {
			clean = append(clean, k)
		}
	}
	b, _ := json.Marshal(clean)
	var err error
	if id := pathID(r, "id"); id > 0 {
		_, err = a.db.Exec(`UPDATE roles SET name = ?, perms = ? WHERE id = ?`, in.Name, string(b), id)
	} else {
		_, err = a.db.Exec(`INSERT INTO roles(name, perms) VALUES(?, ?)`, in.Name, string(b))
	}
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return errConflict("duplicate", "Tên nhóm quyền đã tồn tại")
	}
	if err != nil {
		return err
	}
	return ok(w)
}

func (a *App) handleDeleteRole(w http.ResponseWriter, r *http.Request, p *Principal) error {
	id := pathID(r, "id")
	var n int
	a.db.QueryRow(`SELECT COUNT(*) FROM staff WHERE role_id = ?`, id).Scan(&n)
	if n > 0 {
		return errConflict("in_use", "Nhóm quyền đang được gán cho nhân viên")
	}
	a.db.Exec(`DELETE FROM roles WHERE id = ?`, id)
	return ok(w)
}
