// Package web holds the static assets and HTML pages compiled into the binary.
package web

import "embed"

//go:embed static templates
var FS embed.FS
