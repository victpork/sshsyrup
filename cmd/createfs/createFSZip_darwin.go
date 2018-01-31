package main

import (
	"os"
	"syscall"
)

func getExtraInfo(info os.FileInfo) (uid, gid uint32, atime, mtime, ctime int64) {
	stat := info.Sys().(*syscall.Stat_t)
	uid = stat.Uid
	gid = stat.Gid
	atime = stat.Atimespec.Sec
	mtime = stat.Mtimespec.Sec
	ctime = stat.Ctimespec.Sec

	return
}
