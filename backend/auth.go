package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Permission keys for staff roles.
const (
	PermCustomers    = "customers.manage" // create/edit customers, reset passwords
	PermAlbumsAll    = "albums.view_all"  // see every album (otherwise only assigned ones)
	PermAlbumsManage = "albums.manage"    // create/edit/delete albums, settings, share links
	PermAlbumsUnlock = "albums.status"    // unlock selection, change album status
	PermChat         = "chat"             // reply in chat and photo comments
	PermEditWork     = "edit.work"        // editor view, mark photos as edited
	PermStaff        = "staff.manage"     // staff accounts and roles
	PermSettings     = "settings.manage"  // branding and system settings
)

var AllPerms = []string{PermCustomers, PermAlbumsAll, PermAlbumsManage, PermAlbumsUnlock, PermChat, PermEditWork, PermStaff, PermSettings}

const (
	cookieStaff    = "sa_staff"
	cookieCustomer = "sa_cust"
	staffTTL       = 7 * 24 * time.Hour
	customerTTL    = 30 * 24 * time.Hour
)

type Principal struct {
	Type    string // "staff" | "customer" | "guest"
	ID      int64
	Name    string
	IsOwner bool
	Perms   map[string]bool
	Album   int64 // guest: the only album this principal may see
}

func (p *Principal) Can(perm string) bool {
	return p != nil && p.Type == "staff" && (p.IsOwner || p.Perms[perm])
}

// readerKey identifies a reader for unread-message tracking.
func (p *Principal) readerKey() string {
	if p.Type == "staff" {
		return "staff"
	}
	return "c" + itoa(p.ID)
}

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	return string(b), err
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// randomPassword creates an easy-to-read password to hand to customers.
func randomPassword() string {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

func tokenHash(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// normalizePhone keeps digits and maps +84xxxxxxxxx to 0xxxxxxxxx.
func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	p := b.String()
	if strings.HasPrefix(p, "84") && len(p) >= 11 {
		p = "0" + p[2:]
	}
	return p
}

func (a *App) createSession(w http.ResponseWriter, r *http.Request, userType string, userID int64) error {
	tok := randomToken(32)
	ttl := staffTTL
	name := cookieStaff
	if userType == "customer" {
		ttl, name = customerTTL, cookieCustomer
	}
	exp := time.Now().Add(ttl)
	if _, err := a.db.Exec(`INSERT INTO sessions(token_hash, user_type, user_id, created_at, expires_at) VALUES(?,?,?,?,?)`,
		tokenHash(tok), userType, userID, now(), exp.Unix()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: tok, Path: "/", Expires: exp, HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (a *App) destroySession(w http.ResponseWriter, r *http.Request, cookieName string) {
	if c, err := r.Cookie(cookieName); err == nil {
		a.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// staffFromRequest resolves the staff session cookie.
func (a *App) staffFromRequest(r *http.Request) *Principal {
	c, err := r.Cookie(cookieStaff)
	if err != nil {
		return nil
	}
	var id int64
	err = a.db.QueryRow(`SELECT user_id FROM sessions WHERE token_hash = ? AND user_type = 'staff' AND expires_at > ?`,
		tokenHash(c.Value), now()).Scan(&id)
	if err != nil {
		return nil
	}
	return a.loadStaff(id)
}

func (a *App) loadStaff(id int64) *Principal {
	var name, username, perms sql.NullString
	var owner, active int
	err := a.db.QueryRow(`SELECT s.display_name, s.username, s.is_owner, s.active, r.perms
		FROM staff s LEFT JOIN roles r ON r.id = s.role_id WHERE s.id = ?`, id).
		Scan(&name, &username, &owner, &active, &perms)
	if err != nil || active == 0 {
		return nil
	}
	p := &Principal{Type: "staff", ID: id, Name: name.String, IsOwner: owner == 1, Perms: map[string]bool{}}
	if p.Name == "" {
		p.Name = username.String
	}
	var list []string
	json.Unmarshal([]byte(perms.String), &list)
	for _, k := range list {
		p.Perms[k] = true
	}
	return p
}

func (a *App) customerFromRequest(r *http.Request) *Principal {
	c, err := r.Cookie(cookieCustomer)
	if err != nil {
		return nil
	}
	var id int64
	var name string
	var active int
	err = a.db.QueryRow(`SELECT c.id, c.name, c.active FROM sessions s JOIN customers c ON c.id = s.user_id
		WHERE s.token_hash = ? AND s.user_type = 'customer' AND s.expires_at > ?`, tokenHash(c.Value), now()).
		Scan(&id, &name, &active)
	if err != nil || active == 0 {
		return nil
	}
	return &Principal{Type: "customer", ID: id, Name: name}
}

// guestFromToken resolves a public view-only share link.
func (a *App) guestFromToken(token string) *Principal {
	if token == "" {
		return nil
	}
	var id int64
	var exp, albumExp sql.NullInt64
	err := a.db.QueryRow(`SELECT id, share_expires_at, expires_at FROM albums WHERE share_token = ? AND share_enabled = 1`, token).
		Scan(&id, &exp, &albumExp)
	if err != nil {
		return nil
	}
	if (exp.Valid && exp.Int64 < now()) || (albumExp.Valid && albumExp.Int64 < now()) {
		return nil
	}
	return &Principal{Type: "guest", ID: 0, Name: "Guest", Album: id}
}

// ---- login rate limiting (in memory) ----

type limiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{fails: map[string][]time.Time{}} }

func (l *limiter) blocked(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := time.Now().Add(-window)
	list := l.fails[key][:0]
	for _, t := range l.fails[key] {
		if t.After(cut) {
			list = append(list, t)
		}
	}
	l.fails[key] = list
	return len(list) >= max
}

func (l *limiter) fail(key string) {
	l.mu.Lock()
	l.fails[key] = append(l.fails[key], time.Now())
	l.mu.Unlock()
}

func (l *limiter) reset(key string) {
	l.mu.Lock()
	delete(l.fails, key)
	l.mu.Unlock()
}

// clientIP honours X-Forwarded-For only when the direct peer is loopback (DSM reverse proxy).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if xr := r.Header.Get("X-Real-IP"); xr != "" {
			return xr
		}
	}
	return host
}

func isPrivateIP(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

var errLoginBlocked = errors.New("too many attempts")

func (a *App) checkLoginAllowed(r *http.Request, account string) error {
	ip := clientIP(r)
	if a.limit.blocked("ip:"+ip, 30, 15*time.Minute) || a.limit.blocked("acct:"+account, 8, 15*time.Minute) {
		return errLoginBlocked
	}
	return nil
}

func (a *App) loginFailed(r *http.Request, account string) {
	a.limit.fail("ip:" + clientIP(r))
	a.limit.fail("acct:" + account)
}

// setOwner creates or resets the owner (super admin) account.
func setOwner(db *sql.DB, username, password string) error {
	h, err := hashPassword(password)
	if err != nil {
		return err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM staff WHERE username = ?`, username).Scan(&id)
	if err == sql.ErrNoRows {
		_, err = db.Exec(`INSERT INTO staff(username, password_hash, display_name, is_owner, active, created_at)
			VALUES(?, ?, ?, 1, 1, ?)`, username, h, username, now())
		return err
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE staff SET password_hash = ?, is_owner = 1, active = 1 WHERE id = ?`, h, id)
	if err == nil {
		db.Exec(`DELETE FROM sessions WHERE user_type = 'staff' AND user_id = ?`, id)
	}
	return err
}
