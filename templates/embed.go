package templates

import "embed"

// FS contains all HTML templates used by the web layer.
//
//go:embed base.html pages/*.html partials/*.html
var FS embed.FS
