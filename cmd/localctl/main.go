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
	addr := flag.String("addr", "127.0.0.1:8003", "listen address")
	flag.Parse()

	if err := config.LoadDotEnvAnywhere(); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	cfg, generatedPassword, err := config.Ensure(*addr)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if generatedPassword != "" {
		fmt.Fprintf(os.Stderr, "\n未配置 LOCALCTL_PASSWORD，本次运行的随机密码: %s\n（建议在 .env 中固定密码与 LOCALCTL_SECRET）\n\n", generatedPassword)
	}

	srv := server.New(cfg)
	if err := srv.Serve(cfg.ListenAddr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
