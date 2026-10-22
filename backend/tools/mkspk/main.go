// mkspk assembles the Synology package (.spk) with correct file modes, so it can be built on Windows.
// Usage: go run ./tools/mkspk -root .. -bin ../dist/studioalbum-linux-amd64 -version 1.0.0-0001 -out ../dist/StudioAlbum.spk
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type entry struct {
	name string
	mode int64
	data []byte
	dir  bool
}

func main() {
	root := flag.String("root", "..", "repository root")
	bin := flag.String("bin", "", "linux/amd64 binary")
	version := flag.String("version", "1.0.0-0001", "package version")
	out := flag.String("out", "", "output .spk")
	flag.Parse()
	if *bin == "" || *out == "" {
		log.Fatal("-bin and -out are required")
	}
	pkg := filepath.Join(*root, "package")

	// ---- package.tgz (installed to /var/packages/StudioAlbum/target) ----
	var inner []entry
	binData := must(os.ReadFile(*bin))
	inner = append(inner, entry{name: "bin", dir: true, mode: 0o755}, entry{name: "bin/studioalbum", mode: 0o755, data: binData})
	inner = append(inner, entry{name: "app", dir: true, mode: 0o755},
		entry{name: "app/StudioAlbum.sc", mode: 0o644, data: must(os.ReadFile(filepath.Join(pkg, "app", "StudioAlbum.sc")))})
	uiRoot := filepath.Join(pkg, "ui")
	filepath.WalkDir(uiRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(uiRoot, p)
		name := "ui"
		if rel != "." {
			name = "ui/" + filepath.ToSlash(rel)
		}
		if d.IsDir() {
			inner = append(inner, entry{name: name, dir: true, mode: 0o755})
			return nil
		}
		mode := int64(0o644)
		if strings.HasSuffix(name, ".cgi") {
			mode = 0o755
		}
		inner = append(inner, entry{name: name, mode: mode, data: must(os.ReadFile(p))})
		return nil
	})
	var tgz bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&tgz, gzip.BestCompression)
	writeTar(gz, inner)
	gz.Close()
	sum := md5.Sum(tgz.Bytes())
	var extract int64
	for _, e := range inner {
		extract += int64(len(e.data))
	}

	// ---- INFO ----
	info := string(must(os.ReadFile(filepath.Join(pkg, "INFO.tmpl"))))
	info = strings.NewReplacer("{{VERSION}}", *version, "{{CHECKSUM}}", hex.EncodeToString(sum[:]),
		"{{EXTRACTSIZE}}", fmt.Sprint(extract/1024+1)).Replace(info)
	info = strings.ReplaceAll(info, "\r\n", "\n")

	// ---- outer .spk (plain tar) ----
	outer := []entry{
		{name: "INFO", mode: 0o644, data: []byte(info)},
		{name: "package.tgz", mode: 0o644, data: tgz.Bytes()},
		{name: "PACKAGE_ICON.PNG", mode: 0o644, data: must(os.ReadFile(filepath.Join(pkg, "PACKAGE_ICON.PNG")))},
		{name: "PACKAGE_ICON_256.PNG", mode: 0o644, data: must(os.ReadFile(filepath.Join(pkg, "PACKAGE_ICON_256.PNG")))},
		{name: "conf", dir: true, mode: 0o755},
		{name: "conf/privilege", mode: 0o644, data: must(os.ReadFile(filepath.Join(pkg, "conf", "privilege")))},
		{name: "conf/resource", mode: 0o644, data: must(os.ReadFile(filepath.Join(pkg, "conf", "resource")))},
		{name: "scripts", dir: true, mode: 0o755},
	}
	scripts, _ := os.ReadDir(filepath.Join(pkg, "scripts"))
	for _, s := range scripts {
		data := must(os.ReadFile(filepath.Join(pkg, "scripts", s.Name())))
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")) // shell scripts must have LF endings
		outer = append(outer, entry{name: "scripts/" + s.Name(), mode: 0o755, data: data})
	}
	os.MkdirAll(filepath.Dir(*out), 0o755)
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	writeTar(f, outer)
	f.Close()
	st, _ := os.Stat(*out)
	fmt.Printf("%s (%.1f MB)\n", *out, float64(st.Size())/1e6)
}

func writeTar(w interface{ Write([]byte) (int, error) }, entries []entry) {
	tw := tar.NewWriter(w)
	now := time.Now()
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Mode: e.mode, ModTime: now, Uid: 0, Gid: 0, Uname: "root", Gname: "root", Format: tar.FormatGNU}
		if e.dir {
			h.Typeflag = tar.TypeDir
			h.Name += "/"
		} else {
			h.Typeflag = tar.TypeReg
			h.Size = int64(len(e.data))
			if strings.HasSuffix(e.name, ".cgi") || strings.HasPrefix(e.name, "scripts/") {
				e.data = bytes.ReplaceAll(e.data, []byte("\r\n"), []byte("\n"))
				h.Size = int64(len(e.data))
			}
		}
		if err := tw.WriteHeader(h); err != nil {
			log.Fatal(err)
		}
		if !e.dir {
			tw.Write(e.data)
		}
	}
	tw.Close()
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}
