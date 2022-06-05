package testdata

import "embed"

//go:embed *
var FS embed.FS

const FSID = "bass-testdata"
