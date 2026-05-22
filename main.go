package main

import (
	"embed"
	"io/fs"

	"github.com/YufeiSun5/NodeBridge/internal/datasyncui"
)

// Embed UI. / 内嵌界面。 / UI同梱。
//
//go:embed all:frontend/dist
var embeddedAssets embed.FS

func main() {
	assets, err := fs.Sub(embeddedAssets, "frontend/dist")
	if err != nil {
		panic(err)
	}
	datasyncui.RunWithAssets(assets)
}
