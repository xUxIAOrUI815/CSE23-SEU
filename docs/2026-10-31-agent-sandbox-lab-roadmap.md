# 实现路线（Roadmap）

> 状态：草案
> 日期：2026-10-31
> 关联：设计文档见 [2026-10-31-agent-sandbox-lab-design.md](2026-10-31-agent-sandbox-lab-design.md)

## 总体策略

**MVP 优先，先打地基，再提价值。** 分两阶段：

- **阶段 1（课程 MVP）**：交付一个切题、完整、可演示的最小闭环——亲手实现 OS 沙箱引擎 + 冻结/复现。这一阶段是**地基**，也是课程报告的全部素材。
- **阶段 2（价值提升）**：在 MVP 上做长期维护，接入 LLM agent、做真正的轨迹复现，兑现「实验室」的完整价值故事。

排序理由：OS 引擎 + 冻结是后面所有东西的底座；先把最硬、最切题的部分做扎实，价值层是乘法不是加法。

## 阶段 0：选型与地基（MVP 前置，很短）

- 技术栈 = **Go**（已定）
- 定项目命名
- 搭 Linux 环境（Windows 开发走 WSL2）
- 精读参考实现源码清单：kern、go-sandbox、NsJail、runc（读思路，不抄代码）

**完成标志**：能在 Linux 上 `clone`/`unshare` 起来一个带 namespace 的子进程，并跑通 seccomp / cgroup 的最小 demo。

## 阶段 1：MVP —— 课程交付

目标：**在 Linux 上，用自家引擎冻结一个环境、改配方重跑、diff 差异，全程不依赖 Docker daemon。** 每个 OS 机制都能独立演示并讲清原理。

按依赖顺序拆 5 块：

### 1.1 沙箱引擎（OS 核心，课程抓手）

亲手实现最小隔离栈：

1. 进程隔离：`clone` / `unshare` 创建 namespaces（PID / Mount / Network / UTS / IPC / User）
2. 根切换：`pivot_root` 进入准备的 rootfs
3. 权限收敛：drop capabilities
4. 系统调用过滤：seccomp-bpf 白名单
5. 资源限额：cgroup v2（CPU / memory / pids）
6. 启动目标：`execve`

每一环都要能单独跑通 + 单独讲解（这是课程报告的分节素材）。

### 1.2 环境冻结（overlayfs，OS 核心）

- 快照 = overlayfs：`lowerdir`（只读冻结层）+ `upperdir`（可丢弃写层）
- 三个操作：创建快照 / 丢弃写层 / 切回某个快照
- 复现 = 用不同 lowerdir 重新起一个环境

### 1.3 配方 manifest（② 数据层）

- lockfile 式清单：依赖、环境变量、启动命令、prompt、模型版本、采样参数
- 版本化 + diff

### 1.4 CLI 最小闭环

- 三个命令：`snapshot` / `run` / `diff`
- 端到端：冻结 → 修改 → 重跑 → 对比

### 1.5 轨迹记录（③ 基础版）

- 记录一次运行的关键事件（命令序列 / tool call），日志级
- **不做** ptrace replay（留阶段 2）

**MVP 完成标志（done）**：

- 上述 5 块端到端跑通
- 能讲一个完整演示：改了一个依赖/提示词 → 复现旧版本 → diff 出差异
- 报告素材齐备：namespaces / cgroups / seccomp / overlayfs 各有可展示的实现与原理

## 阶段 2：价值提升（长期维护）

在 MVP 上逐步兑现「实验室」的完整故事：

1. **LLM agent 接入**（方向 2 + 3 合体）：让一个编程 agent 作为实验室的「消费者」，在冻结环境里写码/跑码/迭代
2. **轨迹的确定性复现**（③ 进阶）：ptrace/syscall 记录 + 确定性 replay（rr / GDB record 路线），把「可比较」升级成「可复现」
3. **更强的可复现性**：随机种子固定、模型版本 pinning、外部状态（数据/API）快照
4. **运行形态扩展**：daemon + API、更友好的 UX（按需）

## 里程碑

| 里程碑 | 内容 | 状态 |
| --- | --- | --- |
| M0 | 技术栈 + 环境就绪，最小 syscall demo 跑通 | 未开始 |
| M1 | 沙箱引擎：进程在隔离环境里受限地 `execve` 起来 | 未开始 |
| M2 | 冻结：overlayfs 快照 + 切回 | 未开始 |
| M3 | 最小闭环：snapshot / run / diff 端到端 = **MVP** | 未开始 |
| M4 | 阶段 2：agent 接入 + 轨迹 replay | 未开始 |

## 风险与取舍

- **亲手实现 OS 引擎有工作量**，但这是课程核心，不能外包给现成库
- **rootless vs root**：rootless 更优雅但更复杂；MVP 可先 root（或 user namespace 简化），rootless 作后续优化
- **跨语言 syscall**：Go/Rust 直接，Python 需 ctypes/绑定（这也是选 Go 的一个理由）
- **平台**：namespaces/cgroups/overlayfs 是 Linux 特性，Windows 开发走 WSL2
- **overlayfs 依赖**：需要较新内核 + 挂载权限

## 留待决定

命名待定；技术栈已定 Go；MVP 里 agent 在阶段 2 接入（阶段 1 不做）。
