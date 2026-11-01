# CryoLab MVP 验收清单

> 状态：MVP 完成后整理
> 日期：2026-11-01
> 关联：[设计方案](2026-10-31-agent-sandbox-lab-design.md) · [实现路线](2026-10-31-agent-sandbox-lab-roadmap.md) · [实现计划](2026-10-31-cryolab-mvp-plan.md)

每条验收项都给出**要执行的命令**和**预期结果**，用于交付/答辩时逐条确认。

## 0. 前置条件

在 **WSL2（Ubuntu）** 里，确认以下可用：

```bash
export PATH=$HOME/.local/go/bin:$PATH   # 必须，否则 make 找不到 go
go version          # 预期：go1.25.x linux/amd64
cc --version        # 预期：有 gcc/clang（build-essential）
export GOPROXY=https://goproxy.cn,direct   # 国内网络拉取 x/sys 依赖
```

## 1. 构建

```bash
make                # 预期：生成 nshelper（C 助手）和 cryo（CLI）两个二进制，无报错
```

## 2. 自动化测试

```bash
make test           # 预期：6 个包全部 ok，无 FAIL
```

| 测试 | 验证的 OS 机制 | 预期 |
|---|---|---|
| `TestRunNamespaceIsolation` | namespaces（PID 命名空间） | 子进程 `$$` 为 1 |
| `TestDropCaps` | capabilities（securebits + capset + bounding set） | 目标进程 exec 后 `CapEff` 为全零 |
| `TestSeccompBlocks` | seccomp-bpf | 白名单外的 syscall（execve）被拦截，退出码非零 |
| `TestMemoryLimit` | RLIMIT_AS | 地址空间上限 33554432 字节可见 |
| `TestOverlayMount` | overlayfs | 写落到 upper，lower 不变 |
| `TestOverlayMount` / `TestDropCapsAndOverlay` | overlayfs / capabilities+overlay 组合 | 写落 upper、lower 不变；降权后仍能挂载 overlay 且 `CapEff` 全零 |
| `TestParseValid` / `TestDiff*` | manifest 配方 | 缺 command 报错；diff 输出字段差异 |
| `TestCreateAndList` / `Test*UnsafeName` | 快照存储 + 路径穿越防护 | 建快照/列出正确；`../evil` 被拒 |
| `TestRecordReadBack` | 轨迹 JSONL | 两行事件可读回 |

## 3. 手动验收（CLI 端到端）

### 3.1 冻结快照

```bash
export CRYO_STORE=/tmp/s  CRYO_NSHELPER=$PWD/nshelper
cryo snapshot v1
```

**预期**：无报错；`/tmp/s/snapshots/v1/` 下存在 `manifest.json` 和 `base/`。

### 3.2 配方对比

```bash
rm -rf /tmp/s                        # 重置 store（v1 已在 §3.1 用过，重名会被拒）
cryo snapshot v1 m1.json     # m1.json、m2.json 是两份 prompt/command 不同的 manifest
cryo snapshot v2 m2.json
cryo diff v1 v2
```

**预期**：输出字段级差异，例如 `prompt: A -> B`、`version: 1 -> 2`。

### 3.3 冻结 + 复现（核心卖点）

```bash
echo frozen > /tmp/s/snapshots/v1/base/hello.txt
cryo run v1 -- sh -c "cat hello.txt"        # 预期输出：frozen（读到冻结层）
cryo run v1 -- sh -c "echo new > hello.txt" # 预期：无输出（写入可丢弃层）
cat /tmp/s/snapshots/v1/base/hello.txt      # 预期仍是：frozen（冻结层字节不变）
```

### 3.4 隔离执行（PID 命名空间）

```bash
cryo run v1 -- sh -c "echo pid=\$\$"
```

**预期**：输出 `pid=1`（目标进程是新 PID 命名空间的 1 号进程）。

### 3.5 不存在的快照

```bash
cryo run nope -- true
```

**预期**：输出 `snapshot nope: ... no such file or directory`，退出码非 0。

### 3.6 配方复现（env）+ 轨迹

```bash
rm -rf /tmp/s                        # 重置 store
echo '{"name":"v1","version":"1","command":["true"],"env":{"FOO":"bar"}}' > m1.json
cryo snapshot v1 m1.json
cryo run v1 -- sh -c 'echo FOO=$FOO'   # 预期：FOO=bar（manifest 的 env 被应用）
ls /tmp/s/runs/                        # 预期：有 v1-*.jsonl 轨迹文件（含 exec + exit 两条事件）
```

## 4. 课程要求对照

| 要求 | 落点 |
|---|---|
| 与操作系统强相关 | namespaces / capabilities / seccomp-bpf / RLIMIT / overlayfs，均为亲手调用 syscall 实现（Go + 单线程 C 助手 nshelper） |
| 完整性 + 实际应用价值 | 完整闭环：冻结 → 复现 → 对比 → 隔离运行，解决「代码之外的世界没被冻结」的痛点 |
| 综合运用 | Go（CLI/引擎）+ C（nshelper 命名空间助手）+ 文件系统（overlayfs）+ 进程隔离（namespaces） |
| 避免单一简单功能 | 三层冻结模型（世界/配方/轨迹）+ 5 种 OS 机制各自可独立演示 |

## 5. 已知局限（如实说明）

- **进程数/CPU 限额**用 RLIMIT 而非 cgroup v2（rootless 下 cgroupfs 归宿主 root，写不了），cgroup 记为 roadmap。
- **信号退出码语义**：目标进程被信号杀死时 nshelper 已正确返回 `128+sig`（如 SIGKILL→137）；仅当 nshelper 自身被信号杀死才出现 `-1`→255，属低优先级边缘情况。
- **seccomp 未做 arch 检查**（仅面向 x86_64）。
