package snapshot

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type Overlay struct {
	LowerDir   string
	UpperDir   string
	WorkDir    string
	MountPoint string
}

// optionString 生成 overlayfs 的挂载选项。
func optionString(o Overlay) string {
	return fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", o.LowerDir, o.UpperDir, o.WorkDir)
}

// Mount 挂载一个 overlayfs（lowerdir 为冻结只读层，upperdir 为可丢弃写层）。
func Mount(o Overlay) error {
	return unix.Mount("overlay", o.MountPoint, "overlay", 0, optionString(o))
}

// Unmount 卸载挂载点。
func Unmount(mountPoint string) error {
	return unix.Unmount(mountPoint, 0)
}
