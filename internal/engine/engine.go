package engine

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type Limits struct {
	MemoryBytes int64
	Pids        int64
}

type Spec struct {
	Rootfs       string
	Command      []string
	Env          []string
	Workdir      string
	DropCaps     bool
	SeccompAllow []string
	Limits       Limits
}

const (
	initArg    = "__cryo-init"
	rootfsEnv  = "__CRYO_ROOTFS"
	workdirEnv = "__CRYO_WORKDIR"
)

// init 检测 reexec 进入的 init 进程，直接走 initRun，避免落入测试/主程序入口。
// 这是 runc 同款的 reexec 模式。
func init() {
	if len(os.Args) >= 2 && os.Args[1] == initArg {
		spec := &Spec{
			Command: os.Args[2:],
			Rootfs:  os.Getenv(rootfsEnv),
			Workdir: os.Getenv(workdirEnv),
		}
		code, err := initRun(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cryo init: %v\n", err)
			os.Exit(127)
		}
		os.Exit(code)
	}
}

// Run 隔离执行 Command 并等待退出，返回退出码。
func Run(spec *Spec) (int, error) {
	return parentRun(spec)
}

func parentRun(spec *Spec) (int, error) {
	self, err := os.Executable()
	if err != nil {
		return -1, err
	}
	argv := append([]string{initArg}, spec.Command...)
	cmd := exec.Command(self, argv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: initCloneFlags}
	cmd.Env = append(os.Environ(), rootfsEnv+"="+spec.Rootfs, workdirEnv+"="+spec.Workdir)
	cmd.Env = append(cmd.Env, spec.Env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

func initRun(spec *Spec) (int, error) {
	if len(spec.Command) == 0 {
		return -1, fmt.Errorf("engine: empty command")
	}
	// 已在 init 命名空间内（clone flags 建立），这里只处理 rootfs/workdir
	if err := setupRootfs(spec.Rootfs); err != nil {
		return -1, fmt.Errorf("rootfs: %w", err)
	}
	if spec.Workdir != "" {
		if err := os.Chdir(spec.Workdir); err != nil {
			return -1, fmt.Errorf("workdir: %w", err)
		}
	}
	// init 已是新 PID 命名空间的 PID 1（clone flags 建立），直接 exec 替换自身即可让目标成为 PID 1
	if err := syscall.Exec(spec.Command[0], spec.Command, os.Environ()); err != nil {
		return -1, fmt.Errorf("exec: %w", err)
	}
	return 0, nil // unreachable
}
