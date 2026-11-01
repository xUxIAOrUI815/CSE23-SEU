package engine

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// syscallNums 是 syscall 名 → 编号映射。覆盖常见 shell/glibc 启动与文件操作所需，
// 使白名单既能拦截目标 syscall，又不至于让目标进程无法启动（可后续扩展）。
var syscallNums = map[string]uint32{
	"read":            unix.SYS_READ,
	"write":           unix.SYS_WRITE,
	"exit":            unix.SYS_EXIT,
	"exit_group":      unix.SYS_EXIT_GROUP,
	"execve":          unix.SYS_EXECVE,
	"openat":          unix.SYS_OPENAT,
	"close":           unix.SYS_CLOSE,
	"mkdir":           unix.SYS_MKDIR,
	"mmap":            unix.SYS_MMAP,
	"munmap":          unix.SYS_MUNMAP,
	"brk":             unix.SYS_BRK,
	"mprotect":        unix.SYS_MPROTECT,
	"fstat":           unix.SYS_FSTAT,
	"newfstatat":      unix.SYS_NEWFSTATAT,
	"fstatat":         unix.SYS_NEWFSTATAT, // fstatat 与 newfstatat 同号
	"stat":            unix.SYS_STAT,
	"lstat":           unix.SYS_LSTAT,
	"readlinkat":      unix.SYS_READLINKAT,
	"readlink":        unix.SYS_READLINK,
	"arch_prctl":      unix.SYS_ARCH_PRCTL,
	"rt_sigaction":    unix.SYS_RT_SIGACTION,
	"rt_sigprocmask":  unix.SYS_RT_SIGPROCMASK,
	"rt_sigreturn":    unix.SYS_RT_SIGRETURN,
	"ioctl":           unix.SYS_IOCTL,
	"access":          unix.SYS_ACCESS,
	"getpid":          unix.SYS_GETPID,
	"getppid":         unix.SYS_GETPPID,
	"getuid":          unix.SYS_GETUID,
	"getgid":          unix.SYS_GETGID,
	"geteuid":         unix.SYS_GETEUID,
	"getegid":         unix.SYS_GETEGID,
	"getcwd":          unix.SYS_GETCWD,
	"getrandom":       unix.SYS_GETRANDOM,
	"set_tid_address": unix.SYS_SET_TID_ADDRESS,
	"set_robust_list": unix.SYS_SET_ROBUST_LIST,
	"rseq":            unix.SYS_RSEQ,
	"prlimit64":       unix.SYS_PRLIMIT64,
	"dup":             unix.SYS_DUP,
	"dup2":            unix.SYS_DUP2,
	"dup3":            unix.SYS_DUP3,
	"pipe2":           unix.SYS_PIPE2,
	"fcntl":           unix.SYS_FCNTL,
	"chdir":           unix.SYS_CHDIR,
	"wait4":           unix.SYS_WAIT4,
	"getdents64":      unix.SYS_GETDENTS64,
	"uname":           unix.SYS_UNAME,
	"clock_gettime":   unix.SYS_CLOCK_GETTIME,
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

// applySeccomp 把过滤器应用到当前进程的所有线程。
// 用 SYS_SECCOMP + SECCOMP_FILTER_FLAG_TSYNC（而非 PR_SET_SECCOMP）：Go runtime 是多线程的，
// PR_SET_SECCOMP 只作用于调用线程，goroutine 可能在 exec 前迁移到未过滤线程导致逃逸。
func applySeccomp(f []unix.SockFilter) error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	prog := &unix.SockFprog{
		Len:    uint16(len(f)),
		Filter: &f[0],
	}
	_, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER, unix.SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(prog)))
	if errno != 0 {
		return errno
	}
	return nil
}
