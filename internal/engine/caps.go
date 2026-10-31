package engine

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// dropAllCaps 清空进程的 effective/permitted/inheritable capability 集，并禁止后续获得新特权。
// 注：rootless 下无法修改 bounding set（PR_CAPBSET_DROP 需要初始 user namespace 的 CAP_SETPCAP），
// 因此 uid 0 进程在 exec 时会从 bounding set 重新获得 capability——这是 rootless 的固有局限，
// 见 readCapEff 报告的「exec 前」CapEff 与目标进程「exec 后」CapEff 的差异。
func dropAllCaps() error {
	hdr := &unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	data := [2]unix.CapUserData{} // v3 用两个 32 位表示 64 位集合，全零 = 清空
	if err := unix.Capset(hdr, &data[0]); err != nil {
		return fmt.Errorf("capset: %w", err)
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no_new_privs: %w", err)
	}
	return nil
}

// readCapEff 读取当前进程 /proc/self/status 里的 CapEff 值。
func readCapEff() string {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "CapEff:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				return f[1]
			}
		}
	}
	return ""
}
