package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Editable settings and their defaults.
var settingDefaults = map[string]string{
	"studio_name":      "Studio",
	"primary_color":    "#b4846c",
	"default_lang":     "vi",
	"final_folder":     "Final",
	"public_url":       "",
	"welcome_text":     "",
	"contact_phone":    "",
	"use_syno_thumbs":  "1",
	"preview_cache_gb": "20",
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (a *App) settingsMap() map[string]string {
	out := map[string]string{}
	for k, d := range settingDefaults {
		out[k] = getSetting(a.db, k, d)
	}
	return out
}

func (a *App) hasLogo() bool {
	_, err := os.Stat(filepath.Join(a.cfg.brandingDir(), "logo"))
	return err == nil
}

func (a *App) handleBranding(w http.ResponseWriter, r *http.Request) error {
	s := a.settingsMap()
	writeJSON(w, 200, map[string]any{
		"name": s["studio_name"], "color": s["primary_color"], "lang": s["default_lang"],
		"welcome": s["welcome_text"], "phone": s["contact_phone"], "logo": a.hasLogo(), "version": Version,
		"public_url": s["public_url"],
	})
	return nil
}

func (a *App) handleLogo(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(a.cfg.brandingDir(), "logo")
	ct := getSetting(a.db, "logo_type", "image/png")
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeFile(w, r, p)
}

func (a *App) handleManifest(w http.ResponseWriter, r *http.Request) {
	s := a.settingsMap()
	m := map[string]any{
		"name": s["studio_name"], "short_name": s["studio_name"], "start_url": "/", "scope": "/",
		"display": "standalone", "background_color": "#ffffff", "theme_color": s["primary_color"],
		"icons": []map[string]string{
			{"src": "/icons/icon-192.png", "sizes": "192x192", "type": "image/png"},
			{"src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png"},
			{"src": "/icons/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	json.NewEncoder(w).Encode(m)
}

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request, p *Principal) error {
	s := a.settingsMap()
	writeJSON(w, 200, map[string]any{"settings": s, "logo": a.hasLogo(), "version": Version})
	return nil
}

func (a *App) handlePutSettings(w http.ResponseWriter, r *http.Request, p *Principal) error {
	var in map[string]string
	if err := readJSON(r, &in); err != nil {
		return err
	}
	for k, v := range in {
		if _, ok := settingDefaults[k]; !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k {
		case "primary_color":
			if !colorRe.MatchString(v) {
				return errBad("color", "Mã màu không hợp lệ (dạng #rrggbb)")
			}
		case "default_lang":
			if v != "vi" && v != "en" {
				return errBad("lang", "Ngôn ngữ không hợp lệ")
			}
		case "final_folder":
			if v == "" || strings.ContainsAny(v, `/\`) {
				return errBad("final", "Tên thư mục Final không hợp lệ")
			}
		case "preview_cache_gb":
			if f, err := strconv.ParseFloat(v, 64); err != nil || f < 0 {
				return errBad("cache", "Dung lượng cache không hợp lệ")
			}
		case "public_url":
			v = strings.TrimRight(v, "/")
		}
		if err := setSetting(a.db, k, v); err != nil {
			return err
		}
	}
	return ok(w)
}

func (a *App) handleUploadLogo(w http.ResponseWriter, r *http.Request, p *Principal) error {
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	f, _, err := r.FormFile("logo")
	if err != nil {
		return errBad("file", "File quá lớn hoặc không hợp lệ (tối đa 3MB)")
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	ct := http.DetectContentType(data)
	if ct != "image/png" && ct != "image/jpeg" && ct != "image/webp" && ct != "image/gif" {
		return errBad("type", "Chỉ hỗ trợ PNG, JPG, WEBP")
	}
	os.MkdirAll(a.cfg.brandingDir(), 0o750)
	if err := os.WriteFile(filepath.Join(a.cfg.brandingDir(), "logo"), data, 0o640); err != nil {
		return err
	}
	setSetting(a.db, "logo_type", ct)
	return ok(w)
}

func (a *App) handleDeleteLogo(w http.ResponseWriter, r *http.Request, p *Principal) error {
	os.Remove(filepath.Join(a.cfg.brandingDir(), "logo"))
	return ok(w)
}
