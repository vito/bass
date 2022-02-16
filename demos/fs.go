package demos

import "embed"

//go:embed *
var FS embed.FS

// FSID is the ID stamped on FSPaths using the FS above.
const FSID = "demos"
