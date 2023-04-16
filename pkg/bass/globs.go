package bass

type Globbable interface {
	Includes() []string
	Excludes() []string
	WithInclude(...string) Globbable
	WithExclude(...string) Globbable
}
