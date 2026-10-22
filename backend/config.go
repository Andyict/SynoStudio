package main

import (
	"os"
	"path/filepath"
)

const PackageName = "StudioAlbum"

type Config struct {
	DataDir    string
	Port       int
	Dev        bool   // STUDIO_DEV=1: serve console without DSM auth, relaxed cookies
	SharesRoot string // STUDIO_SHARES_ROOT: dev override; its subfolders are treated as shares
}

func (c *Config) dbPath() string      { return filepath.Join(c.DataDir, "studioalbum.db") }
func (c *Config) cacheDir() string    { return filepath.Join(c.DataDir, "cache") }
func (c *Config) brandingDir() string { return filepath.Join(c.DataDir, "branding") }
func (c *Config) logPath() string     { return filepath.Join(c.DataDir, "studioalbum.log") }

// defaultDataDir prefers the DSM package var dir; otherwise ./data next to the binary.
func defaultDataDir() string {
	if v := os.Getenv("SYNOPKG_PKGVAR"); v != "" {
		return v
	}
	dsm := "/var/packages/" + PackageName + "/var"
	if st, err := os.Stat(dsm); err == nil && st.IsDir() {
		return dsm
	}
	exe, err := os.Executable()
	if err != nil {
		return "data"
	}
	return filepath.Join(filepath.Dir(exe), "data")
}
