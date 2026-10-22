package main

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	thumbSize   = 480  // grid thumbnails ("s")
	previewSize = 2048 // full-screen viewer ("m")
)

type Thumbnailer struct {
	app      *App
	queue    chan int64
	bgSem    chan struct{}
	fgSem    chan struct{}
	mu       sync.Mutex
	inflight map[string]*flight
}

type flight struct {
	done chan struct{}
	err  error
}

func newThumbnailer(a *App) *Thumbnailer {
	n := runtime.NumCPU() / 2
	if n < 1 {
		n = 1
	}
	if n > 3 {
		n = 3
	}
	return &Thumbnailer{
		app:      a,
		queue:    make(chan int64, 100000),
		bgSem:    make(chan struct{}, n),
		fgSem:    make(chan struct{}, 2),
		inflight: map[string]*flight{},
	}
}

func (t *Thumbnailer) Start() {
	for i := 0; i < cap(t.bgSem); i++ {
		go t.worker()
	}
	// Resume photos that never got a thumbnail (e.g. after restart).
	go func() {
		rows, err := t.app.db.Query(`SELECT id FROM photos WHERE thumb_ok = 0 ORDER BY album_id DESC, id`)
		if err != nil {
			return
		}
		var ids []int64
		for rows.Next() {
			var id int64
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			t.Enqueue(id)
		}
	}()
}

func (t *Thumbnailer) Enqueue(photoID int64) {
	select {
	case t.queue <- photoID:
	default: // queue full: the startup sweep will pick it up later
	}
}

func (t *Thumbnailer) worker() {
	for id := range t.queue {
		var ok int
		if t.app.db.QueryRow(`SELECT thumb_ok FROM photos WHERE id = ?`, id).Scan(&ok) != nil || ok == 1 {
			continue
		}
		t.bgSem <- struct{}{}
		_, err := t.Ensure(id, "s", false)
		<-t.bgSem
		if err != nil {
			log.Printf("thumbnail %d: %v", id, err)
			t.app.db.Exec(`UPDATE photos SET thumb_ok = 2 WHERE id = ?`, id) // 2 = failed, do not retry
		}
	}
}

func (t *Thumbnailer) cachePath(albumID, photoID int64, size string) string {
	return filepath.Join(t.app.cfg.cacheDir(), strconv.FormatInt(albumID, 10), fmt.Sprintf("%d_%s.jpg", photoID, size))
}

type photoSrc struct {
	ID, AlbumID int64
	Share, Rel  string
	AlbumRel    string
}

func (t *Thumbnailer) lookup(photoID int64) (*photoSrc, error) {
	p := &photoSrc{ID: photoID}
	err := t.app.db.QueryRow(`SELECT p.album_id, p.rel, a.share, a.rel_path FROM photos p JOIN albums a ON a.id = p.album_id WHERE p.id = ?`,
		photoID).Scan(&p.AlbumID, &p.Rel, &p.Share, &p.AlbumRel)
	return p, err
}

// Ensure returns the cached thumbnail/preview path, generating it if needed.
// fg=true marks an interactive request (separate concurrency budget).
func (t *Thumbnailer) Ensure(photoID int64, size string, fg bool) (string, error) {
	src, err := t.lookup(photoID)
	if err != nil {
		return "", err
	}
	out := t.cachePath(src.AlbumID, photoID, size)
	if st, err := os.Stat(out); err == nil && st.Size() > 0 {
		if size == "m" && time.Since(st.ModTime()) > 24*time.Hour {
			now := time.Now()
			os.Chtimes(out, now, now) // keep recently viewed previews in the LRU cache
		}
		return out, nil
	}

	key := out
	t.mu.Lock()
	if f, ok := t.inflight[key]; ok {
		t.mu.Unlock()
		<-f.done
		return out, f.err
	}
	f := &flight{done: make(chan struct{})}
	t.inflight[key] = f
	t.mu.Unlock()

	if fg {
		t.fgSem <- struct{}{}
	}
	f.err = t.generate(src, size, out)
	if fg {
		<-t.fgSem
	}
	close(f.done)
	t.mu.Lock()
	delete(t.inflight, key)
	t.mu.Unlock()
	return out, f.err
}

func (t *Thumbnailer) generate(src *photoSrc, size, out string) error {
	abs, err := t.app.resolve(src.Share, filepath.ToSlash(filepath.Join(src.AlbumRel, src.Rel)))
	if err != nil {
		return err
	}
	maxDim, quality := previewSize, 85
	if size == "s" {
		maxDim, quality = thumbSize, 80
	}
	o := readOrientation(abs)

	var img *image.RGBA
	// Fast path: reuse the thumbnail Synology Photos already made (@eaDir).
	if size == "s" {
		img = t.fromSynologyThumb(abs, o)
	}
	if img == nil {
		f, err := os.Open(abs)
		if err != nil {
			return err
		}
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			f.Close()
			return err
		}
		if cfg.Width*cfg.Height > 150_000_000 {
			f.Close()
			return fmt.Errorf("image too large: %dx%d", cfg.Width, cfg.Height)
		}
		f.Seek(0, 0)
		full, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		img = orient(downscale(full, maxDim), o)
		w, h := cfg.Width, cfg.Height
		if o >= 5 {
			w, h = h, w
		}
		t.app.db.Exec(`UPDATE photos SET width = ?, height = ? WHERE id = ?`, w, h, src.ID)
	} else if img.Bounds().Dx() > maxDim || img.Bounds().Dy() > maxDim {
		img = downscale(img, maxDim)
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	tmp := out + ".tmp"
	wf, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(wf, img, &jpeg.Options{Quality: quality}); err != nil {
		wf.Close()
		os.Remove(tmp)
		return err
	}
	wf.Close()
	if err := os.Rename(tmp, out); err != nil {
		return err
	}
	if size == "s" {
		t.app.db.Exec(`UPDATE photos SET thumb_ok = 1 WHERE id = ?`, src.ID)
	}
	return nil
}

// fromSynologyThumb loads @eaDir/<file>/SYNOPHOTO_THUMB_XL.jpg if Synology Photos indexed the folder.
func (t *Thumbnailer) fromSynologyThumb(abs string, o int) *image.RGBA {
	if getSetting(t.app.db, "use_syno_thumbs", "1") != "1" {
		return nil
	}
	p := filepath.Join(filepath.Dir(abs), "@eaDir", filepath.Base(abs), "SYNOPHOTO_THUMB_XL.jpg")
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	im, err := jpeg.Decode(f)
	if err != nil {
		return nil
	}
	rgba := downscale(im, thumbSize)
	if o >= 5 {
		// Synology thumbs are normally upright; rotate only when the thumb still has the raw aspect.
		if of, err := os.Open(abs); err == nil {
			cfg, _, err := image.DecodeConfig(of)
			of.Close()
			b := rgba.Bounds()
			if err == nil && (cfg.Width > cfg.Height) == (b.Dx() > b.Dy()) {
				rgba = orient(rgba, o)
			}
		}
	}
	return rgba
}

// RemovePhoto deletes cached files of a photo (when the original changed or vanished).
func (t *Thumbnailer) RemovePhoto(albumID, photoID int64) {
	os.Remove(t.cachePath(albumID, photoID, "s"))
	os.Remove(t.cachePath(albumID, photoID, "m"))
}

func (t *Thumbnailer) RemoveAlbum(albumID int64) {
	os.RemoveAll(filepath.Join(t.app.cfg.cacheDir(), strconv.FormatInt(albumID, 10)))
}

// TrimCache keeps previews ("_m") under the configured limit, deleting the least recently used.
func (t *Thumbnailer) TrimCache() {
	limitGB, _ := strconv.ParseFloat(getSetting(t.app.db, "preview_cache_gb", "20"), 64)
	if limitGB <= 0 {
		return
	}
	limit := int64(limitGB * (1 << 30))
	type ent struct {
		path string
		size int64
		mod  time.Time
	}
	var list []ent
	var total int64
	filepath.WalkDir(t.app.cfg.cacheDir(), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".jpg" || len(p) < 6 || p[len(p)-6:] != "_m.jpg" {
			return nil
		}
		if info, err := d.Info(); err == nil {
			list = append(list, ent{p, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})
	if total <= limit {
		return
	}
	sort.Slice(list, func(i, j int) bool { return list[i].mod.Before(list[j].mod) })
	for _, e := range list {
		if total <= limit*9/10 {
			break
		}
		if os.Remove(e.path) == nil {
			total -= e.size
		}
	}
}

// cacheStats reports cache size for the console.
func cacheStats(dir string) (thumbs, previews int64, files int) {
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files++
		if len(p) > 6 && p[len(p)-6:] == "_m.jpg" {
			previews += info.Size()
		} else {
			thumbs += info.Size()
		}
		return nil
	})
	return
}
