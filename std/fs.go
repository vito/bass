package std

import "embed"

//go:embed *.bass
var FS embed.FS

// FSID is the ID stamped on FSPaths using the FS above.
const FSID = "std"
