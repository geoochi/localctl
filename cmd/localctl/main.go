// Package localctl — a web UI for managing macOS user LaunchAgents.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"localctl/internal/config"
	"localctl/internal/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7788", "listen address")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	cfg, generatedPassword, err := config.Ensure(*addr)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if generatedPassword != "" {
		fmt.Fprintf(os.Stderr, "\n首次启动已生成随机密码: %s\n(保存在 ~/.localctl/config.json)\n\n", generatedPassword)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	if err := srv.Serve(cfg.ListenAddr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
