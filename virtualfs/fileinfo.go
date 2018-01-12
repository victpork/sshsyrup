package virtualfs

import (
	"os"
	"time"
)

type FileInfo struct {
	path string
	n    *Node
}

func (fi *FileInfo) IsDir() bool { return fi.n.FileMode&os.ModeDir != 0 }

func (fi *FileInfo) ModTime() time.Time { return fi.n.modTime }

func (fi *FileInfo) Mode() os.FileMode { return fi.n.FileMode }

func (fi *FileInfo) Name() string {
	return fi.path
}

func (fi *FileInfo) Size() int64 {
	return fi.n.size
}

func (fi *FileInfo) Sys() interface{} {
	return nil
}
