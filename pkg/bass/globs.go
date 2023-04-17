package bass

// Globbable is a Path that supports include/exclude filters.
type Globbable interface {
	// Includes returns the glob patterns to include. If empty, all files are
	// included.
	Includes() []string

	// WithInclude appends to the list of glob patterns to include.
	WithInclude(...string) Globbable

	// Excludes returns the glob patterns to exclude from the included set.
	Excludes() []string

	// WithExclude appends to the list of glob patterns to exclude.
	WithExclude(...string) Globbable
}
