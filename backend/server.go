package main

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type App struct {
	cfg    *Config
	db     *sql.DB
	limit  *limiter
	hub    *Hub
	thumbs *Thumbnailer
	shares *ShareCache
}

func newApp(cfg *Config) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return nil, err
	}
	db, err := openDB(cfg.dbPath())
	if err != nil {
		return nil, err
	}
	a := &App{cfg: cfg, db: db, limit: newLimiter(), hub: newHub()}
	a.shares = &ShareCache{app: a}
	a.thumbs = newThumbnailer(a)
	return a, nil
}

func runServer(cfg *Config) error {
	a, err := newApp(cfg)
	if err != nil {
		return err
	}
	defer a.db.Close()
	a.thumbs.Start()
	go a.housekeeping()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           a.routes(),
		ReadHeaderTimeout: 20 * time.Second,
	}
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		log.Println("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()
	log.Printf("Studio Album %s listening on :%d (data: %s)", Version, cfg.Port, cfg.DataDir)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// housekeeping removes expired sessions and trims the preview cache.
func (a *App) housekeeping() {
	t := time.NewTicker(30 * time.Minute)
	defer t.Stop()
	for {
		a.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now())
		a.db.Exec(`DELETE FROM notifications WHERE created_at < ?`, now()-90*86400)
		a.thumbs.TrimCache()
		<-t.C
	}
}

// ---------- HTTP plumbing ----------

type apiError struct {
	Status int    `json:"-"`
	Code   string `json:"error"`
	Msg    string `json:"message"`
}

func (e *apiError) Error() string { return e.Msg }

func errBad(code, msg string) error       { return &apiError{400, code, msg} }
func errForbidden(msg string) error       { return &apiError{403, "forbidden", msg} }
func errNotFound() error                  { return &apiError{404, "not_found", "Không tìm thấy"} }
func errConflict(code, msg string) error  { return &apiError{409, code, msg} }
func errUnauthorized() error              { return &apiError{401, "unauthorized", "Chưa đăng nhập"} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, ae)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, 404, errNotFound())
		return
	}
	log.Printf("error: %v", err)
	writeJSON(w, 500, &apiError{500, "server_error", "Lỗi máy chủ"})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return errBad("bad_json", "Dữ liệu không hợp lệ")
	}
	return nil
}

func ok(w http.ResponseWriter) error {
	writeJSON(w, 200, map[string]bool{"ok": true})
	return nil
}

func pathID(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return n
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

type handlerFn func(w http.ResponseWriter, r *http.Request) error
type authedFn func(w http.ResponseWriter, r *http.Request, p *Principal) error

func (a *App) public(h handlerFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			writeErr(w, err)
		}
	}
}

func (a *App) customer(h authedFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := a.customerFromRequest(r)
		if p == nil {
			writeErr(w, errUnauthorized())
			return
		}
		if err := h(w, r, p); err != nil {
			writeErr(w, err)
		}
	}
}

// staff requires a staff session and, when perm != "", that permission.
func (a *App) staff(perm string, h authedFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := a.staffFromRequest(r)
		if p == nil {
			writeErr(w, errUnauthorized())
			return
		}
		if perm != "" && !p.Can(perm) {
			writeErr(w, errForbidden("Bạn không có quyền thực hiện thao tác này"))
			return
		}
		if err := h(w, r, p); err != nil {
			writeErr(w, err)
		}
	}
}

// middleware: panic recovery, CSRF header check on mutations, security headers.
func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "internal error", 500)
			}
		}()
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		if r.Method != http.MethodGet && r.Method != http.MethodHead && strings.HasPrefix(r.URL.Path, "/api/") {
			// Browsers cannot send custom headers cross-site without CORS, which we never grant.
			if r.Header.Get("X-SA") != "1" {
				writeJSON(w, 403, &apiError{403, "csrf", "Yêu cầu không hợp lệ"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- gzip for JSON/static ----------

var gzPool = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(io.Discard, 5); return w }}

type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }

func gzipped(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() { gz.Close(); gzPool.Put(gz) }()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
		next.ServeHTTP(&gzipWriter{w, gz}, r)
	})
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	api := http.NewServeMux() // JSON endpoints, gzipped
	mux.Handle("/api/", gzipped(api))

	// public
	api.HandleFunc("GET /api/public/branding", a.public(a.handleBranding))
	mux.HandleFunc("GET /branding/logo", a.handleLogo)
	mux.HandleFunc("GET /manifest.webmanifest", a.handleManifest)

	// customer
	api.HandleFunc("POST /api/c/login", a.public(a.handleCustomerLogin))
	api.HandleFunc("POST /api/c/logout", a.public(a.handleCustomerLogout))
	api.HandleFunc("GET /api/c/me", a.customer(a.handleCustomerMe))
	api.HandleFunc("POST /api/c/lang", a.customer(a.handleCustomerLang))
	api.HandleFunc("POST /api/c/password", a.customer(a.handleCustomerPassword))
	api.HandleFunc("GET /api/c/albums", a.customer(a.handleCustomerAlbums))
	api.HandleFunc("GET /api/c/albums/{id}", a.customer(a.handleCustomerAlbum))
	api.HandleFunc("POST /api/c/albums/{id}/photos/{pid}/pick", a.customer(a.handleCustomerPick))
	api.HandleFunc("POST /api/c/albums/{id}/viewed", a.customer(a.handleCustomerViewed))
	api.HandleFunc("POST /api/c/albums/{id}/submit", a.customer(a.handleCustomerSubmit))
	api.HandleFunc("POST /api/c/albums/{id}/note", a.customer(a.handleCustomerNote))
	api.HandleFunc("GET /api/c/albums/{id}/comments", a.customer(a.handleListComments))
	api.HandleFunc("POST /api/c/albums/{id}/comments", a.customer(a.handlePostComment))
	api.HandleFunc("POST /api/c/albums/{id}/read", a.customer(a.handleMarkRead))
	mux.HandleFunc("GET /dl/c/{id}", a.customer(a.handleCustomerZip))

	// guest share link
	api.HandleFunc("GET /api/g/{token}", a.public(a.handleGuestAlbum))
	mux.HandleFunc("GET /dl/g/{token}", a.public(a.handleGuestZip))

	// images (any principal with access)
	mux.HandleFunc("GET /img/{pid}/{size}", a.handleImage)

	// staff
	api.HandleFunc("POST /api/s/login", a.public(a.handleStaffLogin))
	api.HandleFunc("POST /api/s/logout", a.public(a.handleStaffLogout))
	api.HandleFunc("GET /api/s/me", a.staff("", a.handleStaffMe))
	api.HandleFunc("POST /api/s/password", a.staff("", a.handleStaffPassword))
	api.HandleFunc("GET /api/s/dashboard", a.staff("", a.handleDashboard))
	api.HandleFunc("GET /api/s/notifications", a.staff("", a.handleNotifications))
	api.HandleFunc("POST /api/s/notifications/seen", a.staff("", a.handleNotificationsSeen))
	api.HandleFunc("GET /api/s/shares", a.staff(PermAlbumsManage, a.handleStaffShares))
	api.HandleFunc("GET /api/s/browse", a.staff(PermAlbumsManage, a.handleBrowse))

	api.HandleFunc("GET /api/s/albums", a.staff("", a.handleStaffAlbums))
	api.HandleFunc("POST /api/s/albums", a.staff(PermAlbumsManage, a.handleCreateAlbum))
	api.HandleFunc("GET /api/s/albums/{id}", a.staff("", a.handleStaffAlbum))
	api.HandleFunc("PUT /api/s/albums/{id}", a.staff(PermAlbumsManage, a.handleUpdateAlbum))
	api.HandleFunc("DELETE /api/s/albums/{id}", a.staff(PermAlbumsManage, a.handleDeleteAlbum))
	api.HandleFunc("POST /api/s/albums/{id}/status", a.staff("", a.handleAlbumStatus))
	api.HandleFunc("POST /api/s/albums/{id}/rescan", a.staff("", a.handleRescan))
	api.HandleFunc("POST /api/s/albums/{id}/share", a.staff(PermAlbumsManage, a.handleShareLink))
	api.HandleFunc("POST /api/s/albums/{id}/photos/{pid}/done", a.staff(PermEditWork, a.handleEditDone))
	api.HandleFunc("GET /api/s/albums/{id}/comments", a.staff("", a.handleListComments))
	api.HandleFunc("POST /api/s/albums/{id}/comments", a.staff(PermChat, a.handlePostComment))
	api.HandleFunc("POST /api/s/albums/{id}/read", a.staff("", a.handleMarkRead))
	mux.HandleFunc("GET /dl/s/{id}", a.staff("", a.handleStaffZip))
	mux.HandleFunc("GET /dl/s/{id}/list", a.staff("", a.handleExportList))

	api.HandleFunc("GET /api/s/customers", a.staff("", a.handleListCustomers))
	api.HandleFunc("POST /api/s/customers", a.staff(PermCustomers, a.handleCreateCustomer))
	api.HandleFunc("PUT /api/s/customers/{id}", a.staff(PermCustomers, a.handleUpdateCustomer))
	api.HandleFunc("DELETE /api/s/customers/{id}", a.staff(PermCustomers, a.handleDeleteCustomer))
	api.HandleFunc("POST /api/s/customers/{id}/reset-password", a.staff(PermCustomers, a.handleResetCustomerPassword))

	api.HandleFunc("GET /api/s/staff", a.staff("", a.handleListStaff))
	api.HandleFunc("POST /api/s/staff", a.staff(PermStaff, a.handleCreateStaff))
	api.HandleFunc("PUT /api/s/staff/{id}", a.staff(PermStaff, a.handleUpdateStaff))
	api.HandleFunc("DELETE /api/s/staff/{id}", a.staff(PermStaff, a.handleDeleteStaff))
	api.HandleFunc("GET /api/s/roles", a.staff("", a.handleListRoles))
	api.HandleFunc("POST /api/s/roles", a.staff(PermStaff, a.handleSaveRole))
	api.HandleFunc("PUT /api/s/roles/{id}", a.staff(PermStaff, a.handleSaveRole))
	api.HandleFunc("DELETE /api/s/roles/{id}", a.staff(PermStaff, a.handleDeleteRole))

	api.HandleFunc("GET /api/s/settings", a.staff("", a.handleGetSettings))
	api.HandleFunc("PUT /api/s/settings", a.staff(PermSettings, a.handlePutSettings))
	api.HandleFunc("POST /api/s/settings/logo", a.staff(PermSettings, a.handleUploadLogo))
	api.HandleFunc("DELETE /api/s/settings/logo", a.staff(PermSettings, a.handleDeleteLogo))

	// Package Center source (Package Center → Settings → Package Sources)
	mux.HandleFunc("/pkg-source", a.handlePkgSource)

	// realtime
	mux.HandleFunc("GET /ws", a.handleWS)

	// console hosted on the app port (LAN only, DSM admin login)
	mux.Handle("/api/console", gzipped(http.HandlerFunc(a.handleConsoleServer)))

	// SPA static files
	mux.Handle("/", a.staticHandler())

	return a.middleware(mux)
}
