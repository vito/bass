package proto

import "path/filepath"

func (fsp *FilesystemPath) Slash() string {
	if fsp.GetFile() != nil {
		return fsp.GetFile().GetPath()
	} else {
		return fsp.GetDir().GetPath()
	}
}

func (fsp *FilesystemPath) FromSlash() string {
	return filepath.FromSlash(fsp.Slash())
}

func (fp *FilePath) FromSlash() string {
	return filepath.FromSlash(fp.GetPath())
}

func (dp *DirPath) FromSlash() string {
	return filepath.FromSlash(dp.GetPath())
}
