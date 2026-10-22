package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanRel(t *testing.T) {
	cases := map[string]string{"": "", "a/b": "a/b", "/a//b/": "a/b", `a\b`: "a/b", "./a": "a", "a/../b": "b"}
	for in, want := range cases {
		got, ok := cleanRel(in)
		if !ok || got != want {
			t.Errorf("cleanRel(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"..", "../x", "a/../../x"} {
		if got, ok := cleanRel(bad); ok && got != "" && got[0] == '.' {
			t.Errorf("cleanRel(%q) should not escape: %q", bad, got)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	cases := map[string]string{"0912 345 678": "0912345678", "+84 912-345-678": "0912345678", "84912345678": "0912345678", "0912.345.678": "0912345678"}
	for in, want := range cases {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q want %q", in, got, want)
		}
	}
}

func jpegWithOrientation(t *testing.T, w, h, o int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	if o == 1 {
		return buf.Bytes()
	}
	var tf bytes.Buffer
	tf.WriteString("II*\x00")
	binary.Write(&tf, binary.LittleEndian, uint32(8))
	binary.Write(&tf, binary.LittleEndian, uint16(1))
	binary.Write(&tf, binary.LittleEndian, uint16(0x0112))
	binary.Write(&tf, binary.LittleEndian, uint16(3))
	binary.Write(&tf, binary.LittleEndian, uint32(1))
	binary.Write(&tf, binary.LittleEndian, uint16(o))
	binary.Write(&tf, binary.LittleEndian, uint16(0))
	binary.Write(&tf, binary.LittleEndian, uint32(0))
	payload := append([]byte("Exif\x00\x00"), tf.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	jpg := buf.Bytes()
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func TestExifOrientationAndThumb(t *testing.T) {
	data := jpegWithOrientation(t, 1200, 800, 6)
	if o := exifOrientation(bytes.NewReader(data)); o != 6 {
		t.Fatalf("orientation = %d, want 6", o)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	out := orient(downscale(img, 480), 6)
	if b := out.Bounds(); b.Dx() != 320 || b.Dy() != 480 {
		t.Fatalf("rotated thumb = %dx%d, want 320x480", b.Dx(), b.Dy())
	}
}

// ---------- end-to-end over HTTP ----------

type client struct {
	t    *testing.T
	base string
	c    *http.Client
}

func newClient(t *testing.T, base string) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t, base, &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	req.Header.Set("X-SA", "1")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.c.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	json.Unmarshal(raw, &m)
	return resp.StatusCode, m
}

func TestSelectionFlow(t *testing.T) {
	dir := t.TempDir()
	shares := filepath.Join(dir, "shares")
	albumDir := filepath.Join(shares, "photo", "job1")
	os.MkdirAll(filepath.Join(albumDir, "Final"), 0o755)
	os.MkdirAll(filepath.Join(shares, "secret"), 0o755)
	for _, n := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		os.WriteFile(filepath.Join(albumDir, n), jpegWithOrientation(t, 64, 48, 1), 0o644)
	}
	os.WriteFile(filepath.Join(albumDir, "Final", "a_edit.jpg"), jpegWithOrientation(t, 64, 48, 1), 0o644)

	cfg := &Config{DataDir: filepath.Join(dir, "data"), Port: 0, SharesRoot: shares}
	app, err := newApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer app.db.Close()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	setOwner(app.db, "boss", "secret123")
	app.db.Exec(`INSERT INTO shares(name, path, enabled) VALUES('photo', ?, 1), ('secret', ?, 0)`,
		filepath.Join(shares, "photo"), filepath.Join(shares, "secret"))

	staff := newClient(t, srv.URL)
	if code, _ := staff.do("POST", "/api/s/login", map[string]string{"username": "boss", "password": "secret123"}); code != 200 {
		t.Fatalf("staff login: %d", code)
	}
	// disabled share and traversal are rejected
	if code, _ := staff.do("GET", "/api/s/browse?share=secret&path=", nil); code != 403 {
		t.Errorf("browse disabled share: %d, want 403", code)
	}
	_, cust := staff.do("POST", "/api/s/customers", map[string]any{"phone": "0900000001", "name": "Khách A"})
	pw := cust["password"].(string)
	custID := cust["id"].(float64)
	code, alb := staff.do("POST", "/api/s/albums", map[string]any{"title": "Job 1", "share": "photo", "rel_path": "job1",
		"max_edit": 2, "allow_download": true, "customer_ids": []float64{custID}})
	if code != 200 {
		t.Fatalf("create album: %d %v", code, alb)
	}
	albumID := int64(alb["id"].(float64))

	c := newClient(t, srv.URL)
	if code, _ := c.do("POST", "/api/c/login", map[string]string{"phone": "+84 900 000 001", "password": pw}); code != 200 {
		t.Fatalf("customer login: %d", code)
	}
	code, a := c.do("GET", "/api/c/albums/"+itoa(albumID), nil)
	if code != 200 {
		t.Fatalf("get album: %d", code)
	}
	photos := a["photos"].([]any)
	if len(photos) != 4 {
		t.Fatalf("photos = %d, want 4 (3 orig + 1 final)", len(photos))
	}
	var ids []int64
	for _, p := range photos {
		pm := p.(map[string]any)
		if pm["kind"] == "orig" {
			ids = append(ids, int64(pm["id"].(float64)))
		}
	}
	pick := func(id int64, body map[string]bool) int {
		code, _ := c.do("POST", "/api/c/albums/"+itoa(albumID)+"/photos/"+itoa(id)+"/pick", body)
		return code
	}
	for _, id := range ids {
		if code := pick(id, map[string]bool{"fav": true}); code != 200 {
			t.Fatalf("fav: %d", code)
		}
	}
	if pick(ids[0], map[string]bool{"edit": true}) != 200 || pick(ids[1], map[string]bool{"edit": true}) != 200 {
		t.Fatal("edit picks failed")
	}
	if code := pick(ids[2], map[string]bool{"edit": true}); code != 409 {
		t.Fatalf("third edit pick: %d, want 409 (limit)", code)
	}
	if code, _ := c.do("POST", "/api/c/albums/"+itoa(albumID)+"/submit", nil); code != 200 {
		t.Fatalf("submit: %d", code)
	}
	if code := pick(ids[2], map[string]bool{"fav": false}); code != 409 {
		t.Fatalf("pick after submit: %d, want 409 (locked)", code)
	}

	// another customer cannot see the album
	other := newClient(t, srv.URL)
	_, oc := staff.do("POST", "/api/s/customers", map[string]any{"phone": "0900000002"})
	other.do("POST", "/api/c/login", map[string]string{"phone": "0900000002", "password": oc["password"].(string)})
	if code, _ := other.do("GET", "/api/c/albums/"+itoa(albumID), nil); code != 404 {
		t.Errorf("foreign album: %d, want 404", code)
	}
	resp, _ := other.c.Get(srv.URL + "/img/" + itoa(ids[0]) + "/s")
	if resp.StatusCode != 403 {
		t.Errorf("foreign image: %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// thumbnails and zip download
	resp, _ = c.c.Get(srv.URL + "/img/" + itoa(ids[0]) + "/s")
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("thumb: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	resp.Body.Close()
	resp, _ = c.c.Get(srv.URL + "/dl/c/" + itoa(albumID) + "?set=edit")
	zipData, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || len(zipData) < 100 {
		t.Errorf("zip: %d (%d bytes)", resp.StatusCode, len(zipData))
	}

	// CSRF header is required for mutations
	req, _ := http.NewRequest("POST", srv.URL+"/api/c/albums/"+itoa(albumID)+"/note", bytes.NewReader([]byte(`{"text":"x"}`)))
	resp, _ = c.c.Do(req)
	if resp.StatusCode != 403 {
		t.Errorf("missing X-SA: %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()

	// staff sees the submission notification
	_, n := staff.do("GET", "/api/s/notifications", nil)
	if n["unseen"].(float64) < 1 {
		t.Errorf("expected a notification, got %v", n)
	}
}
