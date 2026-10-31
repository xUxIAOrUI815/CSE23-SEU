package engine

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// initCloneFlags 指定 init 进程一出生就进入的所有命名空间（含 PID）。
// 用 clone flags 而非 unshare：unshare(CLONE_NEWUSER) 在多线程进程（Go 运行时）中会返回 EINVAL，
// 而 clone 创建新进程则无此限制。CLONE_NEWUSER 与其余命名空间同时使用时，内核先建 user namespace
// 再在其内建其余命名空间，因此非特权进程（已启用 userns）也能一次建齐。
var initCloneFlags = uintptr(
	unix.CLONE_NEWUSER |
		unix.CLONE_NEWPID |
		unix.CLONE_NEWNS |
		unix.CLONE_NEWIPC |
		unix.CLONE_NEWUTS |
		unix.CLONE_NEWNET,
)

// writeChildIDMap 由父进程为刚 fork 出的子进程写 uid/gid 映射（把宿主 uid 映射为 userns 内的 root）。
// 子进程在 fd 3 上阻塞等待本函数完成。
func writeChildIDMap(pid, uid, gid int) error {
	writeFile := func(path, data string) error {
		f, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(data)
		return err
	}
	if err := writeFile(fmt.Sprintf("/proc/%d/uid_map", pid), fmt.Sprintf("0 %d 1\n", uid)); err != nil {
		return err
	}
	// 写 gid_map 前必须先 deny setgroups
	if err := writeFile(fmt.Sprintf("/proc/%d/setgroups", pid), "deny\n"); err != nil {
		return err
	}
	return writeFile(fmt.Sprintf("/proc/%d/gid_map", pid), fmt.Sprintf("0 %d 1\n", gid))
}

// waitForUIDMap 由 init 子进程调用：阻塞读取 fd 3，直到父进程写完 uid/gid map 并关闭写端。
func waitForUIDMap() {
	f := os.NewFile(3, "sync")
	if f == nil {
		return
	}
	_, _ = io.ReadAll(f)
	_ = f.Close()
}
