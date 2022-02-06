package pkg

import "embed"

//go:embed **/*.go
var FS embed.FS
