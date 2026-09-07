// Package frontend embeds the built Vite output (frontend/dist).
// Run `pnpm build` in frontend/ before `go build`, otherwise this fails.
package frontend

import "embed"

//go:embed all:dist
var Dist embed.FS
