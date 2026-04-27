package static

import "embed"

//go:embed all:dist
var distFS embed.FS
