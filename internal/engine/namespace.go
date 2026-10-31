package engine

import "golang.org/x/sys/unix"

// initCloneFlags 指定 init 进程一出生就进入的所有命名空间（含 PID）。
// 用 clone flags 而非 unshare：unshare(CLONE_NEWUSER) 在多线程进程（Go 运行时）中会返回 EINVAL，
// 而 clone 创建新进程则无此限制。CLONE_NEWUSER 与其余命名空间同时使用时，内核先建 user namespace
// 再在其内建其余命名空间，因此非特权进程（已启用 userns）也能一次建齐。
// 注：uid 暂未映射（进程内 uid 为 nobody），MVP 够用；后续可加 uid_map 补正。
var initCloneFlags = uintptr(
	unix.CLONE_NEWUSER |
		unix.CLONE_NEWPID |
		unix.CLONE_NEWNS |
		unix.CLONE_NEWIPC |
		unix.CLONE_NEWUTS |
		unix.CLONE_NEWNET,
)
