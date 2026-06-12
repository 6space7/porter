package templates

import "embed"

//go:embed *.yaml
var files embed.FS

func FS() embed.FS {
	return files
}
