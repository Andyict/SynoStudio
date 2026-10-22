package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// principalForMedia accepts staff, customer, or guest (?g=token) credentials.
func (a *App) principalForMedia(r *http.Request) *Principal {
	if p := a.staffFromRequest(r); p != nil {
		return p
	}
	if p := a.customerFromRequest(r); p != nil {
		return p
	}
	return a.guestFromToken(r.URL.Query().Get("g"))
}

func (a *App) handleImage(w http.ResponseWriter, r *http.Request) {
	p := a.principalForMedia(r)
	if p == nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	pid := pathID(r, "pid")
	var albumID, mtime int64
	var rel, name string
	if err := a.db.QueryRow(`SELECT album_id, rel, name, mtime FROM photos WHERE id = ?`, pid).Scan(&albumID, &rel, &name, &mtime); err != nil {
		http.NotFound(w, r)
		return
	}
	al, err := a.albumFor(p, albumID)
	if err != nil {
		http.Error(w, "forbidden", 403)
		return
	}
	size := r.PathValue("size")
	switch size {
	case "s", "m":
		file, err := a.thumbs.Ensure(pid, size, true)
		if err != nil {
			log.Printf("image %d/%s: %v", pid, size, err)
			http.Error(w, "cannot render image", 500)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=604800")
		serveFileETag(w, r, file, fmt.Sprintf(`"%d-%s-%d"`, pid, size, mtime), "image/jpeg")
	case "o":
		if p.Type != "staff" && !al.AllowDownload {
			http.Error(w, "download disabled", 403)
			return
		}
		abs, err := a.resolve(al.Share, path.Join(al.RelPath, rel))
		if err != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.URL.Query().Get("dl") == "1" {
			w.Header().Set("Content-Disposition", contentDisposition(name))
		}
		w.Header().Set("Cache-Control", "private, max-age=86400")
		serveFileETag(w, r, abs, fmt.Sprintf(`"%d-o-%d"`, pid, mtime), mime.TypeByExtension(strings.ToLower(filepath.Ext(name))))
	default:
		http.NotFound(w, r)
	}
}

func serveFileETag(w http.ResponseWriter, r *http.Request, file, etag, ctype string) {
	f, err := os.Open(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("ETag", etag)
	if ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	http.ServeContent(w, r, "", st.ModTime(), f)
}

func contentDisposition(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, url.PathEscape(name))
}

type zipItem struct {
	rel, name, kind string
}

// selectPhotos returns photos of a set: all | fav | edit | final | ids (comma list in ?ids=).
func (a *App) selectPhotos(albumID int64, set, ids string) ([]zipItem, error) {
	q := `SELECT rel, name, kind FROM photos WHERE album_id = ?`
	args := []any{albumID}
	switch set {
	case "all":
		q += ` AND kind = 'orig'`
	case "fav":
		q += ` AND kind = 'orig' AND fav = 1`
	case "edit":
		q += ` AND kind = 'orig' AND edit_pick = 1`
	case "final":
		q += ` AND kind = 'final'`
	case "ids":
		var list []string
		for _, s := range strings.Split(ids, ",") {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				list = append(list, strconv.FormatInt(n, 10))
			}
		}
		if len(list) == 0 {
			return nil, errBad("empty", "Chưa chọn ảnh")
		}
		q += ` AND id IN (` + strings.Join(list, ",") + `)`
	default:
		return nil, errBad("bad_set", "Tập ảnh không hợp lệ")
	}
	rows, err := a.db.Query(q+` ORDER BY kind DESC, name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []zipItem
	for rows.Next() {
		var it zipItem
		rows.Scan(&it.rel, &it.name, &it.kind)
		out = append(out, it)
	}
	return out, nil
}

var zipSetLabel = map[string]string{"all": "TatCa", "fav": "YeuThich", "edit": "ChonEdit", "final": "Final", "ids": "DaChon"}

// streamZip writes a store-only ZIP (JPEGs do not compress) straight to the response.
func (a *App) streamZip(w http.ResponseWriter, r *http.Request, al *Album, items []zipItem, set string) error {
	if len(items) == 0 {
		return errBad("empty", "Không có ảnh nào để tải")
	}
	base := safeFileName(al.Title)
	if base == "" {
		base = "album-" + itoa(al.ID)
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition(base+"_"+zipSetLabel[set]+".zip"))
	w.Header().Set("Cache-Control", "no-store")
	zw := zip.NewWriter(w)
	for _, it := range items {
		abs, err := a.resolve(al.Share, path.Join(al.RelPath, it.rel))
		if err != nil {
			continue
		}
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		st, _ := f.Stat()
		name := it.name
		if it.kind == "final" && set != "final" {
			name = "Final/" + it.name
		}
		hdr := &zip.FileHeader{Name: name, Method: zip.Store, Modified: st.ModTime()}
		hdr.SetMode(0o644)
		fw, err := zw.CreateHeader(hdr)
		if err != nil {
			f.Close()
			return nil // client went away
		}
		_, err = io.Copy(fw, f)
		f.Close()
		if err != nil {
			return nil
		}
	}
	zw.Close()
	return nil
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		if r < 32 {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	if len([]rune(s)) > 80 {
		s = string([]rune(s)[:80])
	}
	return s
}

func (a *App) handleCustomerZip(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	if !al.AllowDownload {
		return errForbidden("Studio chưa bật quyền tải ảnh cho album này")
	}
	set := r.URL.Query().Get("set")
	items, err := a.selectPhotos(al.ID, set, r.URL.Query().Get("ids"))
	if err != nil {
		return err
	}
	return a.streamZip(w, r, al, items, set)
}

func (a *App) handleGuestZip(w http.ResponseWriter, r *http.Request) error {
	g := a.guestFromToken(r.PathValue("token"))
	if g == nil {
		return errNotFound()
	}
	al, err := a.loadAlbum(g.Album)
	if err != nil {
		return err
	}
	if !al.AllowDownload {
		return errForbidden("Album không cho phép tải")
	}
	set := r.URL.Query().Get("set")
	if set != "all" && set != "final" && set != "ids" {
		return errBad("bad_set", "Tập ảnh không hợp lệ")
	}
	items, err := a.selectPhotos(al.ID, set, r.URL.Query().Get("ids"))
	if err != nil {
		return err
	}
	return a.streamZip(w, r, al, items, set)
}

func (a *App) handleStaffZip(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	set := r.URL.Query().Get("set")
	items, err := a.selectPhotos(al.ID, set, r.URL.Query().Get("ids"))
	if err != nil {
		return err
	}
	return a.streamZip(w, r, al, items, set)
}

// handleExportList exports the customer's edit picks: ?format=txt (Lightroom filter list) or csv (with comments).
func (a *App) handleExportList(w http.ResponseWriter, r *http.Request, p *Principal) error {
	al, err := a.albumFor(p, pathID(r, "id"))
	if err != nil {
		return err
	}
	set := r.URL.Query().Get("set")
	if set == "" {
		set = "edit"
	}
	cond := "edit_pick = 1"
	if set == "fav" {
		cond = "fav = 1"
	}
	rows, err := a.db.Query(`SELECT id, name, edit_done FROM photos WHERE album_id = ? AND kind = 'orig' AND `+cond+` ORDER BY name COLLATE NOCASE`, al.ID)
	if err != nil {
		return err
	}
	type item struct {
		id   int64
		name string
		done int
	}
	var list []item
	for rows.Next() {
		var it item
		rows.Scan(&it.id, &it.name, &it.done)
		list = append(list, it)
	}
	rows.Close()
	base := safeFileName(al.Title) + "_" + set
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", contentDisposition(base+".csv"))
		w.Write([]byte("\xEF\xBB\xBF")) // BOM so Excel reads UTF-8
		cw := csv.NewWriter(w)
		cw.Write([]string{"STT", "Tên file", "Đã edit", "Ghi chú của khách"})
		for i, it := range list {
			var notes []string
			crows, _ := a.db.Query(`SELECT author_name, body FROM comments WHERE photo_id = ? ORDER BY id`, it.id)
			for crows.Next() {
				var n, b string
				crows.Scan(&n, &b)
				notes = append(notes, n+": "+b)
			}
			crows.Close()
			done := ""
			if it.done == 1 {
				done = "x"
			}
			cw.Write([]string{strconv.Itoa(i + 1), it.name, done, strings.Join(notes, " | ")})
		}
		cw.Flush()
		return nil
	}
	names := make([]string, len(list))
	for i, it := range list {
		names[i] = strings.TrimSuffix(it.name, filepath.Ext(it.name))
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if r.URL.Query().Get("inline") != "1" {
		w.Header().Set("Content-Disposition", contentDisposition(base+".txt"))
	}
	// Comma separated base names can be pasted straight into Lightroom's Library Filter (Text: "Contains").
	io.WriteString(w, strings.Join(names, ", "))
	return nil
}

