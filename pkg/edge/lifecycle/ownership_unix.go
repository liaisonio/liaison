//go:build linux || darwin

package lifecycle

import (
	"os"
	"syscall"
)

func ownedFile(info os.FileInfo, uid int, leaf bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	if leaf {
		return int(stat.Uid) == uid && stat.Nlink == 1
	}
	return stat.Uid == 0 || int(stat.Uid) == uid
}

func ownedDirectory(info os.FileInfo, uid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == uid
}
