# CryoLab MVP 实现计划（阶段 1）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付一个可演示的最小闭环——在 Linux 上用自家实现的 OS 沙箱引擎「冻结一个环境、重跑、diff 差异」，全程不依赖 Docker daemon。

**Architecture:** 一个 Go CLI（`cryo`），三个子命令 `snapshot / run / diff`。核心是一条亲手实现的 OS 隔离链（namespaces → pivot_root → drop caps → seccomp → cgroup → execve）加 overlayfs 快照；「配方」是一个 JSON manifest，版本化后做 diff；一次运行写入 JSONL 轨迹。隔离引擎用 runc 同款 reexec 模式：进程以 `__cryo_init` 参数重新 exec 自己，在 reexec 阶段完成 syscall 设置后再 `execve` 目标。

**Tech Stack:** Go 1.22+；唯一外部依赖 `golang.org/x/sys/unix`；Linux（kernel 5.x+，需支持 cgroup v2 / overlayfs / namespaces），Windows 开发走 WSL2；manifest 用标准库 `encoding/json`。

**Spec:** [2026-10-31-agent-sandbox-lab-design.md](2026-10-31-agent-sandbox-lab-design.md) + [2026-10-31-agent-sandbox-lab-roadmap.md](2026-10-31-agent-sandbox-lab-roadmap.md)

## Global Constraints

- 语言 Go 1.22+；模块路径 `cryolab`（可后续改名）。
- 唯一外部依赖：`golang.org/x/sys/unix`（seccomp BPF 手写、cgroup 直写 `/sys/fs/cgroup`，均不引第三方库）。
- 目标平台 Linux x86_64；开发用 WSL2。涉及 mount/unshare 的测试需 root 或已启用非特权 user namespace。
- 所有 commit message 结尾加 `Co-Authored-By: Claude Code <noreply@anthropic.com>`。
- 每块 OS 机制都要能**单独演示并讲清原理**（课程报告素材）。

## Review Focus

规格里暗含、但各任务测试未必覆盖、最可能咬到真实用户的五类输入/失败：

1. **manifest 缺少必填项（name / command）**：应返回明确错误、而非 panic。→ Task 1 测试 `TestParseMissingCommand`。
2. **快照名含路径分隔符（如 `../x`）**：必须拒绝，防路径穿越。→ Task 2 测试 `TestCreateRejectsUnsafeName`。
3. **目标程序不存在或 execve 失败**：应返回清晰错误 + 非零退出码，而非静默。→ Task 3 测试 `TestRunCommandNotFound`。
4. **超限进程未被真正杀死**：cgroup 限额必须实际生效（不是只写个数字）。→ Task 6 测试 `TestPidsLimitForcesForkFailure`。
5. **冻结层的完整性**：丢弃写层后，base 必须字节不变。→ Task 7 测试 `TestOverlayDiscardLeavesBaseIntact`。

## File Structure

```
cryolab/
  go.mod
  cmd/cryo/main.go          # CLI 入口：os.Exit(cli.Run(...))
  internal/manifest/        # 配方：解析/序列化/校验/diff
    manifest.go
    diff.go
  internal/store/           # 快照目录布局与元数据读写
    store.go
  internal/engine/          # OS 沙箱引擎（亲手实现）
    engine.go               # Run(spec) + reexec 模式编排
    namespace.go            # unshare namespaces
    rootfs.go               # bind-mount + pivot_root
    caps.go                 # drop capabilities
    seccomp.go              # 白名单 BPF 生成 + prctl 应用
    cgroup.go               # cgroup v2 限额
  internal/snapshot/        # overlayfs 冻结/丢弃/挂载
    snapshot.go
  internal/trace/           # 运行轨迹 JSONL 记录
    trace.go
  internal/cli/             # 子命令分发
    cli.go
```

---

### Task 1: 脚手架 + manifest 配方（解析/序列化/校验/diff）

**Files:**
- Create: `go.mod`, `internal/manifest/manifest.go`, `internal/manifest/diff.go`
- Test: `internal/manifest/manifest_test.go`, `internal/manifest/diff_test.go`

**Interfaces:**
- Produces:
```go
package manifest

type ModelSpec struct {
    ID          string   `json:"id"`
    Temperature *float64 `json:"temperature,omitempty"`
    Seed        *int64   `json:"seed,omitempty"`
}

type Manifest struct {
    Name    string            `json:"name"`
    Version string            `json:"version"`
    Command []string          `json:"command"`
    Env     map[string]string `json:"env,omitempty"`
    Deps    []string          `json:"deps,omitempty"`
    Prompt  string            `json:"prompt,omitempty"`
    Model   *ModelSpec        `json:"model,omitempty"`
}

func Parse(data []byte) (*Manifest, error)
func (m *Manifest) Marshal() ([]byte, error)

type Change struct {
    Field string
    Old   any // nil = 新增
    New   any // nil = 删除
}
func Diff(a, b *Manifest) []Change
```

- [ ] **Step 1: 写失败测试**（manifest 解析 + 校验）

```go
func TestParseValid(t *testing.T) {
    m, err := Parse([]byte(`{"name":"v1","version":"1","command":["python","main.py"]}`))
    if err != nil || m.Name != "v1" || len(m.Command) != 2 { t.Fatalf(...) }
}
func TestParseMissingCommand(t *testing.T) {
    _, err := Parse([]byte(`{"name":"v1"}`)) // 缺 command
    if err == nil { t.Fatal("expected error for missing command") }
}
func TestRoundTrip(t *testing.T) { /* Parse(Marshal(m)) == m */ }
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/manifest/ -run 'TestParse|TestRoundTrip' -v`
Expected: FAIL（`manifest` 包尚未定义）

- [ ] **Step 3: 实现 `manifest.go`**

`Parse` 用 `encoding/json` 反序列化，校验 `Name` 与 `Command` 非空（`Command` 至少一个元素），否则返回带字段名的 error。`Marshal` 用 `json.MarshalIndent` 输出两空格缩进。

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/manifest/ -run 'TestParse|TestRoundTrip' -v`
Expected: PASS

- [ ] **Step 5: 写失败测试（diff）**

```go
func TestDiffChanged(t *testing.T) {
    // a.Prompt="A", b.Prompt="B" → 一个 Change{Field:"prompt", Old:"A", New:"B"}
}
func TestDiffAddedRemoved(t *testing.T) {
    // b 新增 Deps、a 的 Env 在 b 里消失 → 各自对应 Field 的 Change
}
```

- [ ] **Step 6: 运行验证失败**

Run: `go test ./internal/manifest/ -run 'TestDiff' -v`
Expected: FAIL（`Diff` 未定义）

- [ ] **Step 7: 实现 `diff.go`**

`Diff` 逐字段比较（Name/Version/Command/Env/Deps/Prompt/Model.ID/Temperature/Seed），`Old==New` 的跳过，输出稳定的 `[]Change`（按字段名排序）。

- [ ] **Step 8: 运行验证通过**

Run: `go test ./internal/manifest/ -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add go.mod internal/manifest/
git commit -m "feat(manifest): parse/validate/serialize recipe manifest with diff"
```

---

### Task 2: store 快照目录布局

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `manifest.Manifest`, `manifest.Parse`
- Produces:
```go
package store

type Store struct { Root string } // 已解析为绝对路径

func Open(root string) (*Store, error)                      // 确保 <root>/snapshots 存在
func (s *Store) Create(name string, m *manifest.Manifest) error // 建 <root>/snapshots/<name>/{manifest.json, base/}
func (s *Store) List() ([]string, error)                    // 快照名，字典序
func (s *Store) LoadManifest(name string) (*manifest.Manifest, error)
func (s *Store) BasePath(name string) string                // <root>/snapshots/<name>/base
```

- [ ] **Step 1: 写失败测试**

```go
func TestCreateAndList(t *testing.T) {
    s, _ := Open(t.TempDir())
    _ = s.Create("v1", &manifest.Manifest{Name:"v1", Command:[]string{"echo"}})
    names, _ := s.List()
    // names == []string{"v1"}；BasePath("v1") 目录存在；LoadManifest("v1").Name == "v1"
}
func TestCreateRejectsUnsafeName(t *testing.T) {
    _, err := Open(t.TempDir()).Create("../evil", &manifest.Manifest{...})
    // err != nil（含路径分隔符被拒绝）
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/store/ -v`
Expected: FAIL

- [ ] **Step 3: 实现 `store.go`**

`Create` 校验 name 不含 `/`、`\`、`..`；`os.MkdirAll(base)`；manifest 用 `Marshal` 写到 `manifest.json`。`LoadManifest` 读回后 `Parse`。

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/store/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat(store): snapshot directory layout and manifest persistence"
```

---

### Task 3: 沙箱引擎 — 进程隔离（namespaces + rootfs + execve，reexec 模式）

**Files:**
- Create: `internal/engine/engine.go`, `internal/engine/namespace.go`, `internal/engine/rootfs.go`
- Test: `internal/engine/engine_test.go`（集成测试，需 root / 非特权 userns）

**Interfaces:**
- Produces:
```go
package engine

type Limits struct { // 先定义，Task 6 才强制
    MemoryBytes int64
    Pids        int64
}

type Spec struct {
    Rootfs       string   // pivot_root 目标；空 = 用宿主机根
    Command      []string
    Env          []string
    Workdir      string
    DropCaps     bool     // Task 4 强制
    SeccompAllow []string // Task 5 强制，空 = 不加 seccomp
    Limits       Limits   // Task 6 强制，零值 = 不限额
}

// Run 隔离执行 Command 并等待退出，返回退出码。
func Run(spec *Spec) (int, error)
```

- [ ] **Step 1: 写失败测试（隔离可观测）**

```go
func TestRunNamespaceIsolation(t *testing.T) {
    code, _ := Run(&Spec{Command: []string{"/bin/sh","-c","echo pid=$$; [ -f /etc/os-release ] && echo rootfs-ok"}})
    // code == 0；输出含 "pid=1"（PID 命名空间生效）与 "rootfs-ok"（rootfs 可读）
}
func TestRunCommandNotFound(t *testing.T) {
    code, err := Run(&Spec{Command: []string{"/no/such/bin"}})
    // err != nil 且 code != 0
}
```

（此测试捕获 stdout/stderr 断言，需在 Linux 上以 root 或启用非特权 userns 运行。）

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/engine/ -run TestRun -v`
Expected: FAIL

- [ ] **Step 3: 实现 reexec 模式**

`Run` 检测环境变量 `__CRYO_INIT`：未设置时，用 `exec.Command(os.Args[0], 原args...)` 带上 `__CRYO_INIT=1` 重新执行自身并等待；已设置时进入 init 路径：`namespace.go` 的 `unshare(CLONE_NEWNS|CLONE_NEWPID|CLONE_NEWUTS|CLONE_NEWIPC|CLONE_NEWNET)` → `rootfs.go` bind-mount `Rootfs`（空则宿主机根）并 `pivot_root`（pivot 失败则退化为 `chroot`）→ `syscall.Exec` 目标。子进程 stdio 直接透传父进程。

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/engine/ -run TestRun -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/
git commit -m "feat(engine): namespace + rootfs isolation with reexec pattern"
```

---

### Task 4: 沙箱引擎 — drop capabilities

**Files:**
- Create: `internal/engine/caps.go`
- Test: `internal/engine/caps_test.go`

**Interfaces:**
- Consumes: `engine.Spec`（新增 `DropCaps bool` 已在上一步定义）
- Produces: `func dropAllCaps() error`（包内）

- [ ] **Step 1: 写失败测试**

```go
func TestDropCaps(t *testing.T) {
    code, _ := Run(&Spec{DropCaps: true, Command: []string{"/bin/sh","-c","grep CapEff /proc/self/status"}})
    // code == 0；CapEff 为全 0（0000000000000000）
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/engine/ -run TestDropCaps -v`
Expected: FAIL（CapEff 仍非零）

- [ ] **Step 3: 实现 `caps.go`**

`dropAllCaps` 用 `unix.Prctl(unix.PR_CAPBSET_DROP, ...)` 逐个 drop 所有 capability，并 `unix.Capset` 清空 effective/permitted。在 init 路径中、`execve` 之前调用（仅当 `Spec.DropCaps` 为真）。

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/engine/ -run TestDropCaps -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/caps.go internal/engine/caps_test.go
git commit -m "feat(engine): drop capabilities before exec"
```

---

### Task 5: 沙箱引擎 — seccomp-bpf 白名单

**Files:**
- Create: `internal/engine/seccomp.go`
- Test: `internal/engine/seccomp_test.go`

**Interfaces:**
- Consumes: `engine.Spec`（`SeccompAllow []string`）
- Produces:
```go
// buildBPF 由 syscall 名白名单生成 SockFilter（纯函数，可单测）
func buildBPF(allow []string) ([]unix.SockFilter, error)
func applySeccomp(f []unix.SockFilter) error // prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, ...)
```

- [ ] **Step 1: 写失败测试（纯逻辑）**

```go
func TestBuildBPFKnown(t *testing.T) {
    f, err := buildBPF([]string{"read","write","exit_group"})
    // err == nil 且 len(f) > 0
}
func TestBuildBPFUnknown(t *testing.T) {
    _, err := buildBPF([]string{"not_a_syscall"})
    // err != nil
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/engine/ -run TestBuildBPF -v`
Expected: FAIL

- [ ] **Step 3: 实现 `seccomp.go`**

`buildBPF` 用 `golang.org/x/sys/unix` 的 `SYS_*` 常量做名字→编号映射（维护一张 MVP 常用 syscall 映射表），生成「加载 syscall 号 → 命中白名单则 ALLOW，否则 ERRNO(EPERM)」的 BPF。`applySeccomp` 用 `unix.Prctl` 带 `SECCOMP_MODE_FILTER` 应用。

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/engine/ -run TestBuildBPF -v`
Expected: PASS

- [ ] **Step 5: 写失败测试（集成：拦截）**

```go
func TestSeccompBlocks(t *testing.T) {
    // 白名单不含 mkdir
    code, _ := Run(&Spec{SeccompAllow: []string{"read","write","exit_group","openat"}, Command: []string{"/bin/sh","-c","mkdir /tmp/x"}})
    // mkdir 被 EPERM 拦截 → 命令失败，code != 0
}
```

- [ ] **Step 6: 运行验证失败 → 实现 → 通过**

Run: `go test ./internal/engine/ -run TestSeccompBlocks -v`（先 FAIL，接好 init 路径后 PASS）

- [ ] **Step 7: Commit**

```bash
git add internal/engine/seccomp.go internal/engine/seccomp_test.go
git commit -m "feat(engine): seccomp-bpf syscall allowlist"
```

---

### Task 6: 沙箱引擎 — cgroup v2 资源限额

**Files:**
- Create: `internal/engine/cgroup.go`
- Test: `internal/engine/cgroup_test.go`

**Interfaces:**
- Consumes: `engine.Spec`（`Limits`）、`engine.Limits`
- Produces: `func applyLimits(limits Limits) (cleanup func(), err error)`（包内；写入 `/sys/fs/cgroup`）

- [ ] **Step 1: 写失败测试（集成：pids 限额）**

```go
func TestPidsLimitForcesForkFailure(t *testing.T) {
    code, _ := Run(&Spec{Limits: Limits{Pids: 3}, Command: []string{"/bin/sh","-c","for i in $(seq 1 10); do sleep 1 & done; wait"}})
    // 第 4 个 fork 起失败 → sh 报错，code != 0
}
```

- [ ] **Step 2: 运行验证失败**

Run: `go test ./internal/engine/ -run TestPidsLimit -v`
Expected: FAIL（无限额，10 个 fork 都成功，code==0）

- [ ] **Step 3: 实现 `cgroup.go`**

`applyLimits` 在 `/sys/fs/cgroup` 下建一个专属 cgroup 目录，把当前进程 PID 写入 `cgroup.procs`；有 `MemoryBytes` 写 `memory.max`，有 `Pids` 写 `pids.max`；返回 cleanup（删目录）。init 路径在 `execve` 前调用（限额非零时）。

- [ ] **Step 4: 运行验证通过**

Run: `go test ./internal/engine/ -run TestPidsLimit -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/engine/cgroup.go internal/engine/cgroup_test.go
git commit -m "feat(engine): cgroup v2 memory and pids limits"
```

---

### Task 7: overlayfs 快照（冻结/丢弃/挂载）

**Files:**
- Create: `internal/snapshot/snapshot.go`
- Test: `internal/snapshot/snapshot_test.go`

**Interfaces:**
- Produces:
```go
package snapshot

type Overlay struct {
    LowerDir   string
    UpperDir   string
    WorkDir    string
    MountPoint string
}

func optionString(o Overlay) string // 纯函数："lowerdir=...,upperdir=...,workdir=..."
func Mount(o Overlay) error         // unix.Mount("overlay", ...)
func Unmount(mountPoint string) error
```

- [ ] **Step 1: 写失败测试（纯逻辑）**

```go
func TestOptionString(t *testing.T) {
    s := optionString(Overlay{LowerDir:"/l",UpperDir:"/u",WorkDir:"/w"})
    // s == "lowerdir=/l,upperdir=/u,workdir=/w"
}
```

- [ ] **Step 2: 运行验证失败 → 实现 → 通过**

Run: `go test ./internal/snapshot/ -run TestOptionString -v`（FAIL → 实现 `optionString` → PASS）

- [ ] **Step 3: 写失败测试（集成：丢弃写层）**

```go
func TestOverlayDiscardLeavesBaseIntact(t *testing.T) {
    base := t.TempDir()  // 放一个 "keep.txt"
    // Mount(overlay, lower=base, upper=temp, mount=temp2)；在 mount 里写 "new.txt"
    // Unmount 后：base 仍只有 keep.txt；upper 里有 new.txt
}
```

- [ ] **Step 4: 运行验证失败 → 实现 `Mount`/`Unmount` → 通过**

Run: `go test ./internal/snapshot/ -run TestOverlayDiscard -v`（FAIL → 实现 `unix.Mount("overlay", ...)` → PASS；需 root/overlayfs 支持）

- [ ] **Step 5: Commit**

```bash
git add internal/snapshot/
git commit -m "feat(snapshot): overlayfs freeze and discard"
```

---

### Task 8: 运行轨迹记录（JSONL）

**Files:**
- Create: `internal/trace/trace.go`
- Test: `internal/trace/trace_test.go`

**Interfaces:**
- Produces:
```go
package trace

type Event struct {
    Time   time.Time `json:"time"`
    Type   string    `json:"type"`   // "exec" / "exit" / "output"
    Detail string    `json:"detail,omitempty"`
}

type Recorder struct { w io.Writer } // JSONL，每行一个 Event
func NewRecorder(w io.Writer) *Recorder
func (r *Recorder) Record(ev Event) error
```

- [ ] **Step 1: 写失败测试**

```go
func TestRecordReadBack(t *testing.T) {
    var buf bytes.Buffer
    r := NewRecorder(&buf)
    _ = r.Record(Event{Type:"exec", Detail:"python main.py"})
    _ = r.Record(Event{Type:"exit", Detail:"0"})
    // 逐行 json.Unmarshal：两行，第一行 Type=="exec"
}
```

- [ ] **Step 2: 运行验证失败 → 实现 → 通过**

Run: `go test ./internal/trace/ -v`（FAIL → 实现 `NewRecorder`/`Record`（`json.NewEncoder` 追加写）→ PASS）

- [ ] **Step 3: Commit**

```bash
git add internal/trace/
git commit -m "feat(trace): JSONL run event recorder"
```

---

### Task 9: CLI 编排 + 端到端闭环（snapshot / run / diff）

**Files:**
- Create: `internal/cli/cli.go`, `cmd/cryo/main.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `store.Store`, `manifest.*`, `snapshot.*`, `engine.Run`, `trace.*`
- Produces:
```go
package cli
func Run(args []string, stdout, stderr io.Writer) int
```

- [ ] **Step 1: 写失败测试（子命令分发 + 参数校验）**

```go
func TestUnknownCommand(t *testing.T) { // Run(["nope"]) 返回非零，stderr 含用法
}
func TestDiffMissingArgs(t *testing.T) { // Run(["diff","onlyOne"]) 返回非零
}
```

- [ ] **Step 2: 运行验证失败 → 实现 `cli.go` + `main.go` → 通过**

Run: `go test ./internal/cli/ -v`（FAIL → 实现三个子命令分发与参数个数校验 → PASS）
`main.go`：`func main(){ os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }`

- [ ] **Step 3: 端到端验证（手动演示，课程报告主素材）**

在 WSL2 里建一个 demo 工作区，依次执行并记录输出：

```bash
# 准备 demo 工作区：一个 recipe + 一个 hello.py
cryo snapshot v1          # 冻结 v1：base 层 + manifest.json
# 修改 hello.py 和 manifest 里的 prompt
cryo snapshot v2          # 冻结 v2
cryo diff v1 v2           # 输出 prompt/command 变化
cryo run v1 -- python hello.py   # 在 v1 的冻结环境里重跑，输出 v1 行为 + 写轨迹
```

Expected: `diff` 打印出字段级差异；`run v1` 输出 v1 的旧行为（证明「回到那个世界重跑」），轨迹文件生成。

- [ ] **Step 4: Commit**

```bash
git add internal/cli/ cmd/cryo/
git commit -m "feat(cli): wire snapshot/run/diff end-to-end"
```

---

## Self-Review（作者自查）

- **Spec 覆盖**：设计文档第 6 节三层模型——①世界 → Task 3/7；②配方 → Task 1/2；③轨迹基础版 → Task 8；第 8 节「亲手实现 OS 引擎」→ Task 3–6；第 9 节 MVP 五条 → 逐条对应 Task 1/2/3/7/9。无缺口。
- **步骤扫描**：每个 step 都给出精确签名/测试名/命令与期望输出；OS 部分用集成测试而非空泛「加校验」。
- **类型一致**：`Spec` 在 Task 3 一次性定义全字段，Task 4/5/6 只补行为，不改签名；`Limits` 在 Task 3 定义、Task 6 使用。
- **Review Focus**：5 条各绑定到 Task 1/2/3/6/7 的具体测试。
- **比例**：计划长于规格属正常（规格是愿景，计划含接口与测试），但每步只决定「实现者无法自行决定的」内容，不含代码正文。
