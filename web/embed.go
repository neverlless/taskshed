// Package web holds the static assets compiled into the binary.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
)

//go:embed static
var FS embed.FS

// Version fingerprints the embedded assets. Asset URLs carry it, so browsers
// can cache them forever and still pick up a new build.
var Version = fingerprint()

func fingerprint() string {
	h := sha256.New()
	err := fs.WalkDir(FS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := FS.ReadFile(path)
		h.Write([]byte(path))
		h.Write(b)
		return err
	})
	if err != nil {
		panic(err) // embedded files are fixed at compile time
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// Asset is the cacheable URL of a file under static/, e.g. Asset("css/styles.css").
func Asset(path string) string { return "/assets/" + Version + "/" + path }
