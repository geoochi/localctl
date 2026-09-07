// Package localctl — a web UI for managing macOS user LaunchAgents.
package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"localctl/internal/server"
)

// loadDotEnv reads key=value pairs into the process environment (existing env
// wins). Missing file is not an error.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8003", "listen address")
	flag.Parse()

	loadDotEnv(".env")
	if os.Getenv("LOCALCTL_ADDR") == "" {
		if exe, err := os.Executable(); err == nil {
			loadDotEnv(filepath.Join(filepath.Dir(exe), ".env"))
		}
	}

	listenAddr := os.Getenv("LOCALCTL_ADDR")
	if listenAddr == "" {
		listenAddr = *addr
	}

	srv := server.New()
	if err := srv.Serve(listenAddr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
