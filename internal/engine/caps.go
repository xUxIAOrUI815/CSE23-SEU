package engine

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// dropAllCaps 清空进程的 effective/permitted/inheritable capability 集，并禁止后续获得新特权。
//
// rootless 局限（已确认）：clone(CLONE_NEWUSER) 出的子进程在写 uid_map 前后都没有 CAP_SETPCAP
// （effective 集因 uid 未映射而空），因此无法用 PR_SET_SECUREBITS 的 SECBIT_NOROOT 阻止 uid 0
// 在 exec 时从 bounding set 重新获得 capability；bounding set 本身也需要初始 userns 的 CAP_SETPCAP
// 才能改动。所以本函数只能清空 exec 前的 capability，exec 后目标进程会重新获得完整 capability。
// 彻底降权需要 C nsexec 辅助（runc 方案）或真实 root——见 roadmap。
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
