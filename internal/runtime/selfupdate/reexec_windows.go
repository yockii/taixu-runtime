//go:build windows

package selfupdate

import "errors"

// reexecUnix Windows 走不到（ReExec 在 windows 分支先 spawn+exit）。占位保证编译。
func reexecUnix(string) error { return errors.New("unreachable on windows") }
