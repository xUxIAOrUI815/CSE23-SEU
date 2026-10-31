package engine

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type Limits struct {
	MemoryBytes int64 // 经 RLIMIT_AS 实现；进程数/CPU 限额需 cgroup v2（root 模式，roadmap）
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
	initArg     = "__cryo-init"
	rootfsEnv   = "__CRYO_ROOTFS"
	workdirEnv  = "__CRYO_WORKDIR"
	dropCapsEnv = "__CRYO_DROP_CAPS"
	seccompEnv  = "__CRYO_SECCOMP_ALLOW"
	memoryEnv   = "__CRYO_MEMORY_LIMIT"
)

// init 检测 reexec 进入的 init 进程，直接走 initRun，避免落入测试/主程序入口。
// 这是 runc 同款的 reexec 模式。
func init() {
	if len(os.Args) >= 2 && os.Args[1] == initArg {
		spec := &Spec{
			Command:  os.Args[2:],
			Rootfs:   os.Getenv(rootfsEnv),
			Workdir:  os.Getenv(workdirEnv),
			DropCaps: os.Getenv(dropCapsEnv) == "1",
		}
		if s := os.Getenv(seccompEnv); s != "" {
			spec.SeccompAllow = strings.Split(s, ",")
		}
		if s := os.Getenv(memoryEnv); s != "" {
			spec.Limits.MemoryBytes, _ = strconv.ParseInt(s, 10, 64)
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

	// 同步管道：父进程写完 uid/gid map 后关闭写端，子进程读到 EOF 才继续
	syncR, syncW, err := os.Pipe()
	if err != nil {
		return -1, err
	}
	defer syncR.Close()
	cmd.ExtraFiles = []*os.File{syncR}

	dropCaps := "0"
	if spec.DropCaps {
		dropCaps = "1"
	}
	cmd.Env = append(os.Environ(),
		rootfsEnv+"="+spec.Rootfs,
		workdirEnv+"="+spec.Workdir,
		dropCapsEnv+"="+dropCaps,
		seccompEnv+"="+strings.Join(spec.SeccompAllow, ","),
		memoryEnv+"="+strconv.FormatInt(spec.Limits.MemoryBytes, 10),
	)
	cmd.Env = append(cmd.Env, spec.Env...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		syncW.Close()
		return -1, err
	}
	if err := writeChildIDMap(cmd.Process.Pid, os.Getuid(), os.Getgid()); err != nil {
		syncW.Close()
		_ = cmd.Wait()
		return -1, err
	}
	syncW.Close() // 通知子进程继续

	if err := cmd.Wait(); err != nil {
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
	waitForUIDMap() // 等待父进程写完 uid/gid map（此后进程即 userns 内的 root）
	if err := setupRootfs(spec.Rootfs); err != nil {
		return -1, fmt.Errorf("rootfs: %w", err)
	}
	if spec.Workdir != "" {
		if err := os.Chdir(spec.Workdir); err != nil {
			return -1, fmt.Errorf("workdir: %w", err)
		}
	}
	if spec.Limits.MemoryBytes > 0 {
		if err := applyMemoryLimit(spec.Limits.MemoryBytes); err != nil {
			return -1, fmt.Errorf("memory limit: %w", err)
		}
	}
	if spec.DropCaps {
		if err := dropAllCaps(); err != nil {
			return -1, fmt.Errorf("drop caps: %w", err)
		}
		fmt.Fprintf(os.Stderr, "cryo drop-caps-eff=%s\n", readCapEff())
	}
	if len(spec.SeccompAllow) > 0 {
		f, err := buildBPF(spec.SeccompAllow)
		if err != nil {
			return -1, fmt.Errorf("seccomp: %w", err)
		}
		if err := applySeccomp(f); err != nil {
			return -1, fmt.Errorf("seccomp apply: %w", err)
		}
	}
	// init 已是新 PID 命名空间的 PID 1，直接 exec 替换自身即可让目标成为 PID 1
	if err := syscall.Exec(spec.Command[0], spec.Command, os.Environ()); err != nil {
		return -1, fmt.Errorf("exec: %w", err)
	}
	return 0, nil // unreachable
}
