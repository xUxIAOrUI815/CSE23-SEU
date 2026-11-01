package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cryolab/internal/engine"
	"cryolab/internal/manifest"
	"cryolab/internal/snapshot"
	"cryolab/internal/store"
	"cryolab/internal/trace"
)

const usage = `cryo — 轻量可复现的实验沙箱

用法:
  cryo snapshot <name> [manifest.json]   冻结一个快照
  cryo diff <nameA> <nameB>              对比两个快照的配方
  cryo run <name> -- <cmd...>            在快照沙箱里运行命令
`

// Run 分发子命令，返回进程退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "snapshot":
		return cmdSnapshot(args[1:], stderr)
	case "diff":
		return cmdDiff(args[1:], stdout, stderr)
	case "run":
		return cmdRun(args[1:], stderr)
	default:
		fmt.Fprintf(stderr, "未知命令 %q\n\n%s", args[0], usage)
		return 2
	}
}

func openStore() (*store.Store, error) {
	root := os.Getenv("CRYO_STORE")
	if root == "" {
		root = ".cryo"
	}
	return store.Open(root)
}

func cmdSnapshot(args []string, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "用法: cryo snapshot <name> [manifest.json]")
		return 2
	}
	name := args[0]
	m := &manifest.Manifest{Name: name, Version: "1", Command: []string{"true"}}
	if len(args) >= 2 {
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "读 manifest: %v\n", err)
			return 1
		}
		if m, err = manifest.Parse(data); err != nil {
			fmt.Fprintf(stderr, "解析 manifest: %v\n", err)
			return 1
		}
	}
	s, err := openStore()
	if err != nil {
		fmt.Fprintf(stderr, "open store: %v\n", err)
		return 1
	}
	if err := s.Create(name, m); err != nil {
		fmt.Fprintf(stderr, "snapshot: %v\n", err)
		return 1
	}
	return 0
}

func cmdDiff(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "用法: cryo diff <nameA> <nameB>")
		return 2
	}
	s, err := openStore()
	if err != nil {
		fmt.Fprintf(stderr, "open store: %v\n", err)
		return 1
	}
	a, err := s.LoadManifest(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "load %s: %v\n", args[0], err)
		return 1
	}
	b, err := s.LoadManifest(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "load %s: %v\n", args[1], err)
		return 1
	}
	changes := manifest.Diff(a, b)
	if len(changes) == 0 {
		fmt.Fprintln(stdout, "（无差异）")
		return 0
	}
	for _, c := range changes {
		fmt.Fprintf(stdout, "%s: %v -> %v\n", c.Field, c.Old, c.New)
	}
	return 0
}

func cmdRun(args []string, stderr io.Writer) int {
	// 形如: cryo run <name> -- <cmd...>
	if len(args) < 3 || args[1] != "--" {
		fmt.Fprintln(stderr, "用法: cryo run <name> -- <cmd...>")
		return 2
	}
	name := args[0]
	cmd := args[2:]
	s, err := openStore()
	if err != nil {
		fmt.Fprintf(stderr, "open store: %v\n", err)
		return 1
	}
	m, err := s.LoadManifest(name)
	if err != nil {
		fmt.Fprintf(stderr, "snapshot %s: %v\n", name, err)
		return 1
	}
	// 配方复现：应用 manifest 里的 env（代码之外的配置被一起冻结）
	env := make([]string, 0, len(m.Env))
	for k, v := range m.Env {
		env = append(env, k+"="+v)
	}
	base, err := s.BasePath(name)
	if err != nil {
		fmt.Fprintf(stderr, "base: %v\n", err)
		return 1
	}
	// 冻结 + 复现：overlay lower=快照 base（只读冻结层），upper/work/mnt=一次性可丢弃写层
	tmp, err := os.MkdirTemp("", "cryo-run-*")
	if err != nil {
		fmt.Fprintf(stderr, "temp: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	upper := filepath.Join(tmp, "upper")
	work := filepath.Join(tmp, "work")
	mnt := filepath.Join(tmp, "mnt")
	for _, d := range []string{upper, work, mnt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			fmt.Fprintf(stderr, "mkdir: %v\n", err)
			return 1
		}
	}
	// 轨迹记录（③ 轨迹层）：exec + exit 两条事件
	runsDir := filepath.Join(s.Root, "runs")
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "runs: %v\n", err)
		return 1
	}
	tf, err := os.Create(filepath.Join(runsDir, fmt.Sprintf("%s-%d.jsonl", name, time.Now().UnixNano())))
	if err != nil {
		fmt.Fprintf(stderr, "trace: %v\n", err)
		return 1
	}
	defer tf.Close()
	rec := trace.NewRecorder(tf)
	_ = rec.Record(trace.Event{Type: "exec", Detail: strings.Join(cmd, " ")})

	code, err := engine.Run(&engine.Spec{
		Command: cmd,
		Env:     env,
		Overlay: &snapshot.Overlay{LowerDir: base, UpperDir: upper, WorkDir: work, MountPoint: mnt},
		Workdir: mnt,
	})
	_ = rec.Record(trace.Event{Type: "exit", Detail: strconv.Itoa(code)})
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	return code
}
