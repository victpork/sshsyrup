package main

import (
	"os"
	"syscall"
	"time"
)

func getExtraInfo(info os.FileInfo) (uid, gid uint32, atime, mtime, ctime int64) {
	fileAttr := info.Sys().(syscall.Win32FileAttributeData)
	atime = time.Unix(0, fileAttr.LastAccessTime.Nanoseconds()).Unix()
	mtime = time.Unix(0, fileAttr.LastWriteTime.Nanoseconds()).Unix()
	ctime = time.Unix(0, fileAttr.CreationTime.Nanoseconds()).Unix()
	return
}
