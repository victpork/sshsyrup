// +build !windows

package main

import (
	"os"
	"syscall"
)

func getExtraInfo(info os.FileInfo) (uid, gid uint32, atime, mtime, ctime int64) {
	stat := info.Sys().(*syscall.Stat_t)
	uid = stat.Uid
	gid = stat.Gid
	atime, _ = stat.Atim.Unix()
	mtime, _ = stat.Mtim.Unix()
	ctime, _ = stat.Ctim.Unix()

	return
}
