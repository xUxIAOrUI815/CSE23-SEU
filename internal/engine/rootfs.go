package engine

import (
	"os"
	"syscall"
)

// setupRootfs 若指定了 Rootfs 则 chroot 进去。
// MVP 用 chroot（更简单）；pivot_root 是更强的替代（配合 mount namespace 可彻底换根），
// 需要 CAP_SYS_ADMIN，等后续接入 user namespace（rootless）后再启用。
func setupRootfs(rootfs string) error {
	if rootfs == "" {
		return nil
	}
	if err := syscall.Chroot(rootfs); err != nil {
		return err
	}
	return os.Chdir("/")
}
