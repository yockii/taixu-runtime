//go:build !windows

package selfupdate

import (
	"os"
	"syscall"
)

// reexecUnix 原地替换进程镜像（Linux/macOS）。成功不返回；失败返回 err。
func reexecUnix(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
