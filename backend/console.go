package main

// The admin Console (like Synology Drive Admin Console) has two front doors sharing one action set:
//   1. DSM menu app -> /webman/3rdparty/StudioAlbum/api.cgi -> `studioalbum cgi`, authenticated by the DSM session.
//   2. http://NAS:8686/console/ (LAN only), authenticated with a DSM administrator login.

import (
	"bufio"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cgi"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type consoleCtx struct {
	cfg  *Config
	db   *sql.DB
	user string
}

func (c *consoleCtx) dispatch(w http.ResponseWriter, r *http.Request) error {
	action := r.URL.Query().Get("action")
	switch action {
	case "status":
		return c.status(w)
	case "shares":
		writeJSON(w, 200, map[string]any{"shares": listSharesWithState(c.cfg, c.db), "run_as": runAsUser()})
		return nil
	case "share":
		return c.setShare(w, r)
	case "owners":
		return c.owners(w)
	case "owner":
		return c.setOwner(w, r)
	case "settings":
		return c.settings(w, r)
	case "clear-cache":
		return c.clearCache(w, r)
	case "logs":
		return c.logs(w)
	case "backup":
		return c.backup(w)
	case "update":
		writeJSON(w, 200, updateStatus(c.cfg, c.db, r.URL.Query().Get("force") == "1"))
		return nil
	}
	return errBad("bad_action", "Unknown action")
}

func runAsUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return PackageName
}

func dirSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}

func (c *consoleCtx) status(w http.ResponseWriter) error {
	thumbs, previews, files := cacheStats(c.cfg.cacheDir())
	counts := map[string]int{}
	for _, t := range []string{"albums", "customers", "staff", "photos"} {
		var n int
		c.db.QueryRow(`SELECT COUNT(*) FROM ` + t).Scan(&n)
		counts[t] = n
	}
	var pending int
	c.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE thumb_ok = 0`).Scan(&pending)
	running := false
	cl := http.Client{Timeout: 2 * time.Second}
	if resp, err := cl.Get(fmt.Sprintf("http://127.0.0.1:%d/api/public/branding", c.cfg.Port)); err == nil {
		running = resp.StatusCode == 200
		resp.Body.Close()
	}
	writeJSON(w, 200, map[string]any{
		"version": Version, "port": c.cfg.Port, "data_dir": c.cfg.DataDir, "running": running, "user": c.user,
		"db_size": dirSize(c.cfg.dbPath()) + dirSize(c.cfg.dbPath()+"-wal"),
		"cache":   map[string]any{"thumbs": thumbs, "previews": previews, "files": files},
		"counts":  counts, "thumbs_pending": pending, "os": runtime.GOOS + "/" + runtime.GOARCH,
		"public_url":       getSetting(c.db, "public_url", ""),
		"preview_cache_gb": getSetting(c.db, "preview_cache_gb", "20"),
		"use_syno_thumbs":  getSetting(c.db, "use_syno_thumbs", "1"),
		"update_repo":      updateRepo(c.db),
	})
	return nil
}

func (c *consoleCtx) setShare(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var path string
	for _, s := range discoverShares(c.cfg) {
		if s.Name == in.Name {
			path = s.Path
		}
	}
	if path == "" {
		if !in.Enabled {
			c.db.Exec(`UPDATE shares SET enabled = 0 WHERE name = ?`, in.Name)
			return ok(w)
		}
		return errNotFound()
	}
	_, err := c.db.Exec(`INSERT INTO shares(name, path, enabled) VALUES(?,?,?)
		ON CONFLICT(name) DO UPDATE SET path = excluded.path, enabled = excluded.enabled`, in.Name, path, b2i(in.Enabled))
	if err != nil {
		return err
	}
	return ok(w)
}

func (c *consoleCtx) owners(w http.ResponseWriter) error {
	rows, err := c.db.Query(`SELECT id, username, COALESCE(last_login,0) FROM staff WHERE is_owner = 1 ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, last int64
		var u string
		rows.Scan(&id, &u, &last)
		out = append(out, map[string]any{"id": id, "username": u, "last_login": last})
	}
	writeJSON(w, 200, out)
	return nil
}

func (c *consoleCtx) setOwner(w http.ResponseWriter, r *http.Request) error {
	var in struct{ Username, Password string }
	if err := readJSON(r, &in); err != nil {
		return err
	}
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) < 3 || len(in.Password) < 6 {
		return errBad("invalid", "Tên đăng nhập ≥ 3 ký tự, mật khẩu ≥ 6 ký tự")
	}
	if err := setOwner(c.db, in.Username, in.Password); err != nil {
		return err
	}
	return ok(w)
}

func (c *consoleCtx) settings(w http.ResponseWriter, r *http.Request) error {
	var in map[string]string
	if err := readJSON(r, &in); err != nil {
		return err
	}
	for _, k := range []string{"public_url", "preview_cache_gb", "use_syno_thumbs", "update_repo"} {
		if v, ok := in[k]; ok {
			if k == "public_url" {
				v = strings.TrimRight(strings.TrimSpace(v), "/")
			}
			setSetting(c.db, k, v)
		}
	}
	return ok(w)
}

func (c *consoleCtx) clearCache(w http.ResponseWriter, r *http.Request) error {
	var in struct{ What string }
	readJSON(r, &in)
	if in.What == "all" {
		os.RemoveAll(c.cfg.cacheDir())
		c.db.Exec(`UPDATE photos SET thumb_ok = 0`)
	} else {
		filepath.Walk(c.cfg.cacheDir(), func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, "_m.jpg") {
				os.Remove(p)
			}
			return nil
		})
	}
	return ok(w)
}

func (c *consoleCtx) logs(w http.ResponseWriter) error {
	f, err := os.Open(c.cfg.logPath())
	if err != nil {
		writeJSON(w, 200, map[string]any{"lines": []string{}})
		return nil
	}
	defer f.Close()
	if st, _ := f.Stat(); st != nil && st.Size() > 256*1024 {
		f.Seek(-256*1024, io.SeekEnd)
	}
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) > 400 {
		lines = lines[len(lines)-400:]
	}
	writeJSON(w, 200, map[string]any{"lines": lines})
	return nil
}

func (c *consoleCtx) backup(w http.ResponseWriter) error {
	tmp := filepath.Join(c.cfg.DataDir, fmt.Sprintf("backup-%d.db", time.Now().UnixNano()))
	if _, err := c.db.Exec(`VACUUM INTO ?`, tmp); err != nil {
		return err
	}
	defer os.Remove(tmp)
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition("studioalbum-"+time.Now().Format("20060102-1504")+".db"))
	io.Copy(w, f)
	return nil
}

// ---------- DSM authentication ----------

// dsmUserFromCGI asks DSM which user owns the current browser session (CGI env carries the cookies).
func dsmUserFromCGI(r *http.Request) (string, error) {
	tok := r.Header.Get("X-SYNO-TOKEN")
	if tok == "" {
		tok = r.URL.Query().Get("SynoToken")
	}
	cmd := exec.Command("/usr/syno/synoman/webman/modules/authenticate.cgi")
	env := os.Environ()
	if tok != "" {
		env = append(env, "HTTP_X_SYNO_TOKEN="+tok, "QUERY_STRING=SynoToken="+url.QueryEscape(tok))
	}
	cmd.Env = append(env, "REQUEST_METHOD=GET")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	u := strings.TrimSpace(string(out))
	if u == "" || strings.ContainsAny(u, " \n<{") {
		return "", fmt.Errorf("not logged in")
	}
	return u, nil
}

func isDSMAdmin(username string) bool {
	if username == "admin" {
		return true
	}
	out, err := exec.Command("id", "-Gn", username).Output()
	if err != nil {
		return false
	}
	for _, g := range strings.Fields(string(out)) {
		if g == "administrators" {
			return true
		}
	}
	return false
}

// runCGI serves console actions when invoked by DSM's web server.
func runCGI(cfg *Config) {
	db, err := openDB(cfg.dbPath())
	if err != nil {
		fmt.Printf("Status: 500\r\nContent-Type: text/plain\r\n\r\n%v", err)
		return
	}
	defer db.Close()
	cgi.Serve(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := dsmUserFromCGI(r)
		if err != nil {
			writeJSON(w, 401, &apiError{401, "dsm_auth", "Phiên DSM không hợp lệ, hãy đăng nhập lại DSM"})
			return
		}
		if !isDSMAdmin(u) {
			writeJSON(w, 403, &apiError{403, "not_admin", "Chỉ quản trị viên DSM mới dùng được Console"})
			return
		}
		if r.URL.Query().Get("action") == "whoami" {
			writeJSON(w, 200, map[string]any{"user": u, "mode": "dsm"})
			return
		}
		c := &consoleCtx{cfg: cfg, db: db, user: u}
		if err := c.dispatch(w, r); err != nil {
			writeErr(w, err)
		}
	}))
}

// ---------- console on the app port ----------

var consoleSessions = struct {
	sync.Mutex
	m map[string]consoleSession
}{m: map[string]consoleSession{}}

type consoleSession struct {
	user string
	exp  time.Time
}

func (a *App) handleConsoleServer(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.Dev && !isPrivateIP(clientIP(r)) {
		writeJSON(w, 403, &apiError{403, "lan_only", "Console chỉ truy cập được trong mạng nội bộ"})
		return
	}
	action := r.URL.Query().Get("action")
	if action == "login" {
		a.consoleLogin(w, r)
		return
	}
	user := ""
	if a.cfg.Dev {
		user = "dev"
	} else if ck, err := r.Cookie("sa_console"); err == nil {
		consoleSessions.Lock()
		s, ok := consoleSessions.m[tokenHash(ck.Value)]
		consoleSessions.Unlock()
		if ok && s.exp.After(time.Now()) {
			user = s.user
		}
	}
	if user == "" {
		writeJSON(w, 401, &apiError{401, "unauthorized", "Đăng nhập bằng tài khoản quản trị DSM"})
		return
	}
	if action == "whoami" {
		writeJSON(w, 200, map[string]any{"user": user, "mode": "server"})
		return
	}
	if action == "logout" {
		if ck, err := r.Cookie("sa_console"); err == nil {
			consoleSessions.Lock()
			delete(consoleSessions.m, tokenHash(ck.Value))
			consoleSessions.Unlock()
		}
		ok(w)
		return
	}
	c := &consoleCtx{cfg: a.cfg, db: a.db, user: user}
	if err := c.dispatch(w, r); err != nil {
		writeErr(w, err)
	}
}

func (a *App) consoleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password, OTP string }
	if err := readJSON(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := a.checkLoginAllowed(r, "console:"+in.Username); err != nil {
		writeJSON(w, 429, &apiError{429, "too_many", "Sai quá nhiều lần, thử lại sau 15 phút"})
		return
	}
	if err := dsmLogin(in.Username, in.Password, in.OTP); err != nil {
		a.loginFailed(r, "console:"+in.Username)
		writeJSON(w, 401, &apiError{401, "invalid_login", err.Error()})
		return
	}
	if !isDSMAdmin(in.Username) {
		writeJSON(w, 403, &apiError{403, "not_admin", "Tài khoản không thuộc nhóm administrators"})
		return
	}
	tok := randomToken(32)
	consoleSessions.Lock()
	consoleSessions.m[tokenHash(tok)] = consoleSession{in.Username, time.Now().Add(8 * time.Hour)}
	consoleSessions.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "sa_console", Value: tok, Path: "/api/console", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteStrictMode, MaxAge: 8 * 3600})
	writeJSON(w, 200, map[string]any{"user": in.Username})
}

// dsmLogin verifies DSM credentials against the local DSM Web API.
func dsmLogin(user, pass, otp string) error {
	form := url.Values{
		"api": {"SYNO.API.Auth"}, "version": {"6"}, "method": {"login"}, "account": {user}, "passwd": {pass},
		"session": {"StudioAlbumConsole"}, "format": {"sid"},
	}
	if otp != "" {
		form.Set("otp_code", otp)
	}
	cl := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	var lastErr error
	for _, base := range dsmBaseURLs() {
		resp, err := cl.PostForm(base+"/webapi/entry.cgi", form)
		if err != nil {
			lastErr = err
			continue
		}
		var out struct {
			Success bool `json:"success"`
			Data    struct {
				Sid string `json:"sid"`
			} `json:"data"`
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if out.Success {
			cl.PostForm(base+"/webapi/entry.cgi", url.Values{"api": {"SYNO.API.Auth"}, "version": {"6"}, "method": {"logout"},
				"session": {"StudioAlbumConsole"}, "_sid": {out.Data.Sid}})
			return nil
		}
		switch out.Error.Code {
		case 403:
			return fmt.Errorf("Tài khoản bật xác minh 2 bước: nhập mã OTP")
		case 404:
			return fmt.Errorf("Mã OTP không đúng")
		default:
			return fmt.Errorf("Sai tài khoản hoặc mật khẩu DSM")
		}
	}
	return fmt.Errorf("Không kết nối được DSM: %v", lastErr)
}

// dsmBaseURLs returns the local DSM web addresses, using the ports configured in DSM
// (Control Panel → Login Portal) instead of assuming 5000/5001.
func dsmBaseURLs() []string {
	httpPort, httpsPort := "5000", "5001"
	if b, err := os.ReadFile("/etc/synoinfo.conf"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
			if !ok {
				continue
			}
			v = strings.Trim(v, `"`)
			switch k {
			case "admin_port":
				if v != "" {
					httpPort = v
				}
			case "secure_admin_port":
				if v != "" {
					httpsPort = v
				}
			}
		}
	}
	return []string{"http://127.0.0.1:" + httpPort, "https://127.0.0.1:" + httpsPort}
}
