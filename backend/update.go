package main

// Package Center source: DSM (Package Center → Settings → Package Sources) calls
// http://127.0.0.1:8686/pkg-source and gets the latest GitHub release of this app,
// so new versions show up as a normal "Update" button.

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// UpdateRepo is "owner/repo" on GitHub; set at build time with -ldflags "-X main.UpdateRepo=...".
var UpdateRepo = ""

// githubAPI is overridable in tests.
var githubAPI = "https://api.github.com"

type releaseInfo struct {
	Version   string `json:"version"` // package version, e.g. 1.0.3-0012
	MD5       string `json:"md5"`
	Size      int64  `json:"size"`
	Changelog string `json:"changelog"`
	Link      string `json:"-"`
	Tag       string `json:"-"`
	Published string `json:"-"`
}

var releaseCache struct {
	sync.Mutex
	at   time.Time
	info *releaseInfo
	err  error
}

func updateRepo(db *sql.DB) string {
	if r := strings.TrimSpace(getSetting(db, "update_repo", "")); r != "" {
		return r
	}
	return UpdateRepo
}

// latestRelease asks GitHub for the newest release (cached 10 minutes: the anonymous API allows 60 calls/hour).
func latestRelease(repo string) (*releaseInfo, error) {
	releaseCache.Lock()
	defer releaseCache.Unlock()
	if time.Since(releaseCache.at) < 10*time.Minute && (releaseCache.info != nil || releaseCache.err != nil) {
		return releaseCache.info, releaseCache.err
	}
	info, err := fetchRelease(repo)
	releaseCache.at, releaseCache.info, releaseCache.err = time.Now(), info, err
	if err != nil {
		log.Printf("update check: %v", err)
	}
	return info, err
}

func fetchRelease(repo string) (*releaseInfo, error) {
	if repo == "" {
		return nil, fmt.Errorf("update repository not configured")
	}
	cl := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", githubAPI+"/repos/"+repo+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "StudioAlbum/"+Version)
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github: %s", resp.Status)
	}
	var rel struct {
		TagName     string `json:"tag_name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	info := &releaseInfo{Tag: rel.TagName, Changelog: rel.Body, Published: rel.PublishedAt}
	var metaURL string
	for _, as := range rel.Assets {
		switch {
		case strings.HasSuffix(as.Name, ".spk"):
			info.Link, info.Size = as.URL, as.Size
		case as.Name == "release.json":
			metaURL = as.URL
		}
	}
	if info.Link == "" || metaURL == "" {
		return nil, fmt.Errorf("release %s has no .spk or release.json", rel.TagName)
	}
	mresp, err := cl.Get(metaURL)
	if err != nil {
		return nil, err
	}
	defer mresp.Body.Close()
	var meta releaseInfo
	if err := json.NewDecoder(mresp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("release.json: %w", err)
	}
	info.Version, info.MD5 = meta.Version, meta.MD5
	if meta.Size > 0 {
		info.Size = meta.Size
	}
	if info.Version == "" {
		return nil, fmt.Errorf("release.json without version")
	}
	return info, nil
}

// handlePkgSource answers Package Center in the Synology package-source format.
func (a *App) handlePkgSource(w http.ResponseWriter, r *http.Request) {
	repo := updateRepo(a.db)
	packages := []map[string]any{}
	if info, err := latestRelease(repo); err == nil {
		icon := "https://raw.githubusercontent.com/" + repo + "/main/package/PACKAGE_ICON_256.PNG"
		packages = append(packages, map[string]any{
			"package":               PackageName,
			"version":               info.Version,
			"dname":                 "Studio Album",
			"desc":                  "Album proofing for photo studios: customers heart and pick photos, comment, chat and download.",
			"link":                  info.Link,
			"size":                  info.Size,
			"md5":                   info.MD5,
			"thumbnail":             []string{icon, icon},
			"thumbnail_retina":      []string{icon, icon},
			"snapshot":              []string{},
			"changelog":             info.Changelog,
			"maintainer":            repo,
			"maintainer_url":        "https://github.com/" + repo,
			"distributor":           repo,
			"distributor_url":       "https://github.com/" + repo,
			"support_url":           "https://github.com/" + repo + "/issues",
			"qinst":                 true,
			"qstart":                true,
			"qupgrade":              true,
			"start":                 true,
			"beta":                  false,
			"silent_install":        false,
			"silent_uninstall":      false,
			"silent_upgrade":        true,
			"deppkgs":               nil,
			"conflictpkgs":          nil,
			"depsers":               nil,
			"price":                 0,
			"download_count":        0,
			"recent_download_count": 0,
			"category":              0,
			"subcategory":           0,
			"type":                  0,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"packages": packages, "keyrings": []string{}})
}

// updateStatus is shown in the Console.
func updateStatus(cfg *Config, db *sql.DB, force bool) map[string]any {
	repo := updateRepo(db)
	if force {
		releaseCache.Lock()
		releaseCache.at = time.Time{}
		releaseCache.Unlock()
	}
	out := map[string]any{"current": Version, "repo": repo, "source_url": fmt.Sprintf("http://127.0.0.1:%d/pkg-source", cfg.Port)}
	info, err := latestRelease(repo)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	out["latest"] = info.Version
	out["tag"] = info.Tag
	out["published"] = info.Published
	out["changelog"] = info.Changelog
	out["has_update"] = info.Version != Version
	return out
}
