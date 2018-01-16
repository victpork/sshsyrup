// +build !windows

package main

import (
	"os"
	"syscall"
)

func getExtraInfo(info os.FileInfo) (uid, gid uint32) {
	stat := info.Sys().(*syscall.Stat_t)
	uid = stat.Uid
	gid = stat.Gid

	return
}
