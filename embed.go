package main

import (
	"embed"
	"io/fs"
)

//go:embed assets/dist
var embeddedAssets embed.FS

func assets() fs.FS {
	sub, _ := fs.Sub(embeddedAssets, "assets")
	return sub
}
