package bass

type Globbable interface {
	Include(paths ...FilesystemPath) Globbable
	Exclude(paths ...FilesystemPath) Globbable
}
