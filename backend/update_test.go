package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestPkgSource(t *testing.T) {
	var gh *httptest.Server
	gh = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/me/SynoStudio/releases/latest":
			json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v1.0.2", "body": "fixes",
				"assets": []map[string]any{
					{"name": "StudioAlbum-x86_64-1.0.2.spk", "browser_download_url": gh.URL + "/dl/a.spk", "size": 123},
					{"name": "release.json", "browser_download_url": gh.URL + "/dl/release.json", "size": 50},
				},
			})
		case "/dl/release.json":
			w.Write([]byte(`{"version":"1.0.2-0007","md5":"abc","size":123}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer gh.Close()
	githubAPI = gh.URL
	UpdateRepo = "me/SynoStudio"
	releaseCache.at = releaseCache.at.AddDate(-1, 0, 0)

	app, err := newApp(&Config{DataDir: filepath.Join(t.TempDir(), "d")})
	if err != nil {
		t.Fatal(err)
	}
	defer app.db.Close()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/pkg-source", "application/x-www-form-urlencoded", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Packages []map[string]any `json:"packages"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Packages) != 1 {
		t.Fatalf("packages = %v", out.Packages)
	}
	p := out.Packages[0]
	if p["package"] != "StudioAlbum" || p["version"] != "1.0.2-0007" || p["md5"] != "abc" || p["link"] != gh.URL+"/dl/a.spk" {
		t.Errorf("unexpected package entry: %v", p)
	}
}
