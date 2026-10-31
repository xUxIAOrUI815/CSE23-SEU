package engine

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// syscallNums 是 MVP 常用的 syscall 名 → 编号映射（可后续扩展）。
var syscallNums = map[string]uint32{
	"read":         unix.SYS_READ,
	"write":        unix.SYS_WRITE,
	"exit":         unix.SYS_EXIT,
	"exit_group":   unix.SYS_EXIT_GROUP,
	"execve":       unix.SYS_EXECVE,
	"openat":       unix.SYS_OPENAT,
	"close":        unix.SYS_CLOSE,
	"mkdir":        unix.SYS_MKDIR,
	"mmap":         unix.SYS_MMAP,
	"brk":          unix.SYS_BRK,
	"mprotect":     unix.SYS_MPROTECT,
	"fstat":        unix.SYS_FSTAT,
	"rt_sigaction": unix.SYS_RT_SIGACTION,
	"rt_sigreturn": unix.SYS_RT_SIGRETURN,
	"ioctl":        unix.SYS_IOCTL,
	"access":       unix.SYS_ACCESS,
	"getpid":       unix.SYS_GETPID,
}

// buildBPF 生成 seccomp 白名单过滤器：命中白名单 → ALLOW，否则 → ERRNO(EPERM)。
func buildBPF(allow []string) ([]unix.SockFilter, error) {
	var f []unix.SockFilter
	// 加载 syscall 编号（seccomp_data.nr 在偏移 0）
	f = append(f, unix.SockFilter{
		Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS,
		K:    0,
	})
	for _, name := range allow {
		nr, ok := syscallNums[name]
		if !ok {
			return nil, fmt.Errorf("unknown syscall %q", name)
		}
		f = append(f, unix.SockFilter{
			Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K,
			K:    nr,
			Jt:   0, // 相等 → 下一条（RET ALLOW）
			Jf:   1, // 不等 → 跳过 RET ALLOW
		})
		f = append(f, unix.SockFilter{
			Code: unix.BPF_RET | unix.BPF_K,
			K:    unix.SECCOMP_RET_ALLOW,
		})
	}
	// 默认：ERRNO(EPERM)
	f = append(f, unix.SockFilter{
		Code: unix.BPF_RET | unix.BPF_K,
		K:    unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM),
	})
	return f, nil
}

// applySeccomp 把过滤器应用到当前进程。
func applySeccomp(f []unix.SockFilter) error {
	// 安装过滤器需要 CAP_SYS_ADMIN 或 no_new_privs；先设 no_new_privs 更稳妥，且本身就是沙箱应有的加固。
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	prog := &unix.SockFprog{
		Len:    uint16(len(f)),
		Filter: &f[0],
	}
	_, _, errno := unix.Syscall(unix.SYS_PRCTL, unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(prog)))
	if errno != 0 {
		return errno
	}
	return nil
}
