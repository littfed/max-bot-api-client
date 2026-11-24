package web

import "embed"

// DistFS contains the embedded frontend assets built from the React 18 SPA.
//
//go:embed all:dist
var DistFS embed.FS
