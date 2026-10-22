// Studio Album — album proofing app for photo studios, packaged for Synology DSM 7.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

var Version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory (database, cache, logs)")
	port := fs.Int("port", 8686, "HTTP port")
	_ = fs.Parse(os.Args[2:])

	cfg := &Config{DataDir: *dataDir, Port: *port, Dev: os.Getenv("STUDIO_DEV") == "1"}
	cfg.SharesRoot = os.Getenv("STUDIO_SHARES_ROOT")

	switch cmd {
	case "serve":
		if err := runServer(cfg); err != nil {
			log.Fatal(err)
		}
	case "cgi":
		runCGI(cfg)
	case "reset-admin":
		resetAdminCLI(cfg, fs.Args())
	case "version":
		fmt.Println(Version)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `studioalbum <command> [flags]
  serve         run the web server
  cgi           console API (invoked by DSM as CGI)
  reset-admin   reset owner account: reset-admin -data DIR <username> [password]
  version       print version`)
}

func resetAdminCLI(cfg *Config, args []string) {
	if len(args) < 1 {
		log.Fatal("usage: reset-admin -data DIR <username> [password]")
	}
	db, err := openDB(cfg.dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	pw := ""
	if len(args) > 1 {
		pw = args[1]
	} else {
		fmt.Print("New password: ")
		pw, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		pw = strings.TrimSpace(pw)
	}
	if len(pw) < 6 {
		log.Fatal("password must be at least 6 characters")
	}
	if err := setOwner(db, args[0], pw); err != nil {
		log.Fatal(err)
	}
	fmt.Println("owner account updated:", args[0])
}
