package main

import (
	"database/sql"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ShareInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Readable bool   `json:"readable"`
	Enabled  bool   `json:"enabled"`
	Known    bool   `json:"known"` // still present on disk
}

func skipName(n string) bool {
	return n == "" || n[0] == '@' || n[0] == '#' || n[0] == '.' || n == "lost+found"
}

// discoverShares lists shared folders on the NAS (top-level folders of each volume).
func discoverShares(cfg *Config) []ShareInfo {
	var roots []string
	if cfg.SharesRoot != "" {
		roots = []string{cfg.SharesRoot}
	} else {
		v, _ := filepath.Glob("/volume[0-9]*")
		u, _ := filepath.Glob("/volumeUSB[0-9]*")
		roots = append(v, u...)
	}
	var out []ShareInfo
	seen := map[string]bool{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || skipName(e.Name()) || seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			p := filepath.Join(root, e.Name())
			_, err := os.ReadDir(p)
			out = append(out, ShareInfo{Name: e.Name(), Path: p, Readable: err == nil, Known: true})
		}
	}
	return out
}

// listSharesWithState merges discovered shares with the enabled flags stored in the DB.
func listSharesWithState(cfg *Config, db *sql.DB) []ShareInfo {
	found := discoverShares(cfg)
	byName := map[string]*ShareInfo{}
	for i := range found {
		byName[found[i].Name] = &found[i]
	}
	rows, err := db.Query(`SELECT name, path, enabled FROM shares`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s ShareInfo
			var en int
			rows.Scan(&s.Name, &s.Path, &en)
			if f, ok := byName[s.Name]; ok {
				f.Enabled = en == 1
			} else if en == 1 {
				s.Enabled = true
				found = append(found, s) // enabled but missing on disk
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return strings.ToLower(found[i].Name) < strings.ToLower(found[j].Name) })
	return found
}

// ShareCache keeps the enabled share map; the console may change it from another process.
type ShareCache struct {
	app    *App
	mu     sync.Mutex
	at     time.Time
	shares map[string]string
}

func (c *ShareCache) Enabled() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.at) < 5*time.Second && c.shares != nil {
		return c.shares
	}
	m := map[string]string{}
	rows, err := c.app.db.Query(`SELECT name, path FROM shares WHERE enabled = 1`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var n, p string
			rows.Scan(&n, &p)
			m[n] = p
		}
	}
	c.shares, c.at = m, time.Now()
	return m
}

// cleanRel normalises a slash-separated relative path and rejects traversal.
func cleanRel(rel string) (string, bool) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	c := path.Clean("/" + rel)
	c = strings.TrimPrefix(c, "/")
	if c == "." {
		c = ""
	}
	for _, part := range strings.Split(c, "/") {
		if part == ".." {
			return "", false
		}
	}
	return c, true
}

// resolve maps (share, rel) to an absolute path inside an enabled share.
func (a *App) resolve(share, rel string) (string, error) {
	base, ok := a.shares.Enabled()[share]
	if !ok {
		return "", errForbidden("Share folder chưa được bật trong Console")
	}
	c, ok := cleanRel(rel)
	if !ok {
		return "", errBad("bad_path", "Đường dẫn không hợp lệ")
	}
	abs := filepath.Join(base, filepath.FromSlash(c))
	// Guard against symlinks pointing outside the share.
	realBase, err1 := filepath.EvalSymlinks(base)
	realAbs, err2 := filepath.EvalSymlinks(abs)
	if err1 == nil && err2 == nil {
		if realAbs != realBase && !strings.HasPrefix(realAbs, realBase+string(filepath.Separator)) {
			return "", errForbidden("Đường dẫn nằm ngoài share folder")
		}
	}
	return abs, nil
}

func (a *App) handleStaffShares(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var out []map[string]any
	en := a.shares.Enabled()
	names := make([]string, 0, len(en))
	for n := range en {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		_, err := os.ReadDir(en[n])
		out = append(out, map[string]any{"name": n, "readable": err == nil})
	}
	writeJSON(w, 200, out)
	return nil
}

// handleBrowse lists sub-folders (and the image count) of share/path for the album folder picker.
func (a *App) handleBrowse(w http.ResponseWriter, r *http.Request, p *Principal) error {
	share := r.URL.Query().Get("share")
	rel, _ := cleanRel(r.URL.Query().Get("path"))
	abs, err := a.resolve(share, rel)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return errForbidden("Ứng dụng không đọc được thư mục này. Kiểm tra quyền share folder trong Console.")
	}
	type dir struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	dirs := []dir{}
	images := 0
	for _, e := range entries {
		if e.IsDir() {
			if !skipName(e.Name()) {
				dirs = append(dirs, dir{e.Name(), path.Join(rel, e.Name())})
			}
		} else if isImageName(e.Name()) {
			images++
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	// Is this folder already used by an album?
	var used int64
	a.db.QueryRow(`SELECT id FROM albums WHERE share = ? AND rel_path = ?`, share, rel).Scan(&used)
	writeJSON(w, 200, map[string]any{"share": share, "path": rel, "dirs": dirs, "images": images, "album_id": used})
	return nil
}

func isImageName(n string) bool {
	switch strings.ToLower(filepath.Ext(n)) {
	case ".jpg", ".jpeg", ".png":
		return !strings.HasPrefix(n, "._") && !strings.HasPrefix(n, ".")
	}
	return false
}
