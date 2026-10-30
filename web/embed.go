// Package web holds the static assets compiled into the binary.
package web

import "embed"

//go:embed static
var FS embed.FS
