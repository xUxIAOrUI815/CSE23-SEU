package engine

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"cryolab/internal/snapshot"
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
	Overlay      *snapshot.Overlay // overlayfs 冻结挂载（可选）
}

const (
	initArg    = "__cryo-init"
	initEnv    = "__CRYO_INIT"
	rootfsEnv  = "__CRYO_ROOTFS"
	workdirEnv = "__CRYO_WORKDIR"
	seccompEnv = "__CRYO_SECCOMP_ALLOW"
	memoryEnv  = "__CRYO_MEMORY_LIMIT"

	// nshelper（C）读取的环境变量
	dropCapsEnv     = "CRYO_DROP_CAPS"
	overlayLowerEnv = "CRYO_OVERLAY_LOWER"
	overlayUpperEnv = "CRYO_OVERLAY_UPPER"
	overlayWorkEnv  = "CRYO_OVERLAY_WORK"
	overlayMntEnv   = "CRYO_OVERLAY_MNT"
	nshelperEnv     = "CRYO_NSHELPER"
)

// init 检测由 nshelper exec 回来的 Go init 进程，直接走 initRun。
func init() {
	if os.Getenv(initEnv) == "1" && len(os.Args) >= 2 && os.Args[1] == initArg {
		spec := &Spec{
			Command: os.Args[2:],
			Rootfs:  os.Getenv(rootfsEnv),
			Workdir: os.Getenv(workdirEnv),
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
// 命名空间创建、uid/gid 映射、降权、overlay 挂载交给 C 助手 nshelper（单线程才能 unshare userns）。
func Run(spec *Spec) (int, error) {
	helper := os.Getenv(nshelperEnv)
	if helper == "" {
		helper = "nshelper"
	}
	self, err := os.Executable()
	if err != nil {
		return -1, err
	}
	argv := append([]string{initArg}, spec.Command...)
	cmd := exec.Command(helper, append([]string{self}, argv...)...)
	cmd.Env = append(os.Environ(), initEnv+"=1")
	cmd.Env = append(cmd.Env,
		rootfsEnv+"="+spec.Rootfs,
		workdirEnv+"="+spec.Workdir,
		seccompEnv+"="+strings.Join(spec.SeccompAllow, ","),
		memoryEnv+"="+strconv.FormatInt(spec.Limits.MemoryBytes, 10),
	)
	if spec.DropCaps {
		cmd.Env = append(cmd.Env, dropCapsEnv+"=1")
	}
	if spec.Overlay != nil {
		cmd.Env = append(cmd.Env,
			overlayLowerEnv+"="+spec.Overlay.LowerDir,
			overlayUpperEnv+"="+spec.Overlay.UpperDir,
			overlayWorkEnv+"="+spec.Overlay.WorkDir,
			overlayMntEnv+"="+spec.Overlay.MountPoint,
		)
	}
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
	// 命名空间/uid_map/降权/overlay 已由 nshelper 完成，这里只做剩余加固与执行
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
	if len(spec.SeccompAllow) > 0 {
		f, err := buildBPF(spec.SeccompAllow)
		if err != nil {
			return -1, fmt.Errorf("seccomp: %w", err)
		}
		if err := applySeccomp(f); err != nil {
			return -1, fmt.Errorf("seccomp apply: %w", err)
		}
	}
	// 裸命令名需先经 PATH 解析（execve 不搜 PATH）
	argv0 := spec.Command[0]
	if !strings.Contains(argv0, "/") {
		if p, err := exec.LookPath(argv0); err == nil {
			argv0 = p
		}
	}
	if err := syscall.Exec(argv0, spec.Command, os.Environ()); err != nil {
		return -1, fmt.Errorf("exec: %w", err)
	}
	return 0, nil // unreachable
}
