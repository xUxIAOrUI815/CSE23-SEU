package engine

import "golang.org/x/sys/unix"

// applyMemoryLimit 通过 RLIMIT_AS（地址空间上限）限制内存。
// rootless 下用 setrlimit 而非 cgroup v2：cgroupfs 归宿主机 root，userns 内进程写不了（见 ledger ruling）。
// 进程数/CPU 限额需要 cgroup v2（root 模式），留作 roadmap。
func applyMemoryLimit(bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	lim := &unix.Rlimit{Cur: uint64(bytes), Max: uint64(bytes)}
	return unix.Setrlimit(unix.RLIMIT_AS, lim)
}
