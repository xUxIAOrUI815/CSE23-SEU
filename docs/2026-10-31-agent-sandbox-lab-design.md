# 基于容器沙箱的轻量级编程 Agent 系统 — 设计方案

> 状态：草案（头脑风暴 + 调研结论，待确认）
> 日期：2026-10-31
> 定位：课程实践项目，先交付 MVP，之后长期维护

## 1. 一句话定位

一个**轻量、自托管的"实验室"**，把「**冻结 + 复现**」当核心：让开发者能冻结一次 agent 运行的完整世界——尤其是**代码之外**的依赖、提示词、模型、配置——然后回到那个世界重跑、对比、复现问题。

核心故事（卖点）：**沙箱（隔离）层正在被商品化，竞争壁垒上移到控制面。我们的差异化不在"又一个沙箱"，而在把「冻结 + 复现」做成一等公民的实验管理——这是当前轻量自托管领域的一块空白。**

## 2. 为什么做这个

### 课程层面
- 选题：基于容器沙箱的轻量级编程 Agent 系统
- 要求：与操作系统强相关；有完整性和实际应用价值；综合运用计算机知识
- 交付：先交 MVP，之后持续维护

### 个人层面
- 需要一个能证明「眼光」和「职业竞争力」的项目，而不是一个玩具
- 本人背景（仅作背景，不收敛为项目目标）：企业内部做 agent 可观测性

### 结论
「隔离」本身是已解决的问题、且正在商品化；「冻结 + 复现」的实验管理才是尚未被轻量占住的差异化价值，同时它天然落到操作系统原理上，一门两用。

## 3. 调研结论（摘要）

### 3.1 隔离强度光谱

| 方案 | 隔离强度 | 冷启动 | 适用 |
|---|---|---|---|
| Docker (runc) | 弱（共享内核） | <1s | 受信代码 |
| **namespaces + cgroups + seccomp（自建）** | 中 | 极快 | 半受信代码 ← 我们 |
| gVisor | 中（用户态 syscall 拦截） | ~1s | 半受控 |
| Firecracker 微虚拟机 | 强 | 100–300ms | 多租户 / 恶意代码 |
| 全虚拟机 | 最强 | 秒级 | 极端场景 |

### 3.2 沙箱层正在商品化

E2B、Daytona、Modal、Fly.io Sprites、Vercel Sandbox、Google K8s Agent Sandbox、Azure Container Apps 在做几乎趋同的隔离产品。结论：**单纯做沙箱 = 红海；价值在控制面（编排、策略、凭证、生命周期）。**

### 3.3 轻量 DIY 路线（OS-native，已成熟）

- kern（Rust，单二进制 ~1.6MB，~2ms 冷启动，rootless）
- NsJail（Google）、go-sandbox（在线判题常用）、sandlock（Landlock + seccomp-notify）
- 标准最小路径：`clone/unshare(namespaces)` → `pivot_root` → 丢 capabilities → seccomp-bpf → cgroup 限额 → `execve`

**结论：隔离原语是已解决的，直接站在它们肩膀上，不重写。**

### 3.4 「快照 / 冻结」是正在冒头、但没人做轻的前沿

Fly.io Sprites（亚秒 checkpoint/restore）、Google K8s Agent Sandbox（pod 快照）、Azure（快照 suspend/resume）、E2B（文件系统/内存 pause-resume）都在抢「快照」能力。**但它们全是云上、收费、重；轻量自托管的（kern/NsJail/go-sandbox）没有一个把「冻结」当一等公民。** 这就是缺口。

### 3.5 现有工具对照（我们 vs 它们）

| 工具 | 冻结了什么 | 缺什么 |
|---|---|---|
| git | 代码 | 不懂运行环境、提示词、模型、依赖 |
| Docker / Nix / devcontainer | 环境 | 重；不懂 prompt/模型/配置；不是「快照→A/B→丢弃」的实验节奏 |
| LangSmith / Langfuse / Braintrust / promptfoo | 提示词 / trace / eval | 只观察，不拥有执行环境——不能「回到那个世界重跑」 |
| E2B / Daytona / Modal | 执行环境 | 云上、收费、重；「冻结」是附带的快照，不是核心 |
| MLflow / DVC / W&B | 数据 / 模型 / 代码 | 重、ML 专用，不是「代码+提示词」的快实验手感 |
| **我们** | **世界 + 配方 + 轨迹（合体）** | —— |

空白点 = 上表六行的交集：一个轻量、自托管、OS-native 的「实验室」，把「一次可运行的实验」当版本单位。

## 4. 用户与痛点

### 用户
**独立开发者和小团队，正在做 LLM 应用或 agent、迭代很快的人。** 规模小到用不起、也不会去搭 LangSmith + E2B + Docker + MLflow 全家桶，现在靠 git + 截图 + 笔记硬撑。

（次要/重叠用户：想在本地安全跑 coding agent 的人——和方向 3 是同一批人。）

### 痛点（wince 时刻）

> 昨天改了一条提示词、升了一个依赖、换了模型、还顺手改了工具。今天崩了。你想知道是哪一样弄崩的——但你复现不出「昨天」，因为**代码之外的那个世界从来没和代码一起被冻结过**。git 有代码，却没有代码跑起来时的那个世界。

核心：**git 只冻结代码，而 agent 时代会变的不只是代码。**

### 更深一层：agent 的「概率性 + 自主性」

- **概率性**：采样、温度、模型权重静默更新
- **自主性**：agent 自己决定走哪条路、调哪个工具
- **后果**：冻结代码不足以复现。因此对 agent 来说，「复现」有两个不同的目标：

1. **可比较**：同样配方重跑，得到一批结果，能 A/B、看分布 → 需冻结「输入世界」
2. **可复现**：把某次崩溃逐帧重放 → 不能靠重跑（agent 不会再做同样选择），只能靠冻结「它实际做了什么」

**方向性判断：不要试图让 agent 确定性化（跟概率性硬碰是错误方向）；正确做法是接受不确定性，然后把它冻结下来。**

## 5. 目标 / 非目标 / 反目标

### 目标
1. 轻量、自托管、单机可跑（无 Docker daemon，优先 rootless）
2. 以「冻结 + 复现」为核心，冻结代码之外的世界
3. OS 原理可讲（namespaces / cgroups / seccomp / overlayfs）
4. MVP 可交付、完整、有实际价值

### 非目标（明确不做）
- 多租户云服务、计费、控制台
- 强隔离微虚拟机（Firecracker / gVisor）——半受信场景不需要
- 复刻 LangSmith / E2B 的完整功能

### 反目标（做错方向 = 失败）
- 试图让 agent 确定性化
- 让「冻结」变成事后补丁（那就退化成又一个容器工具）
- 重写沙箱原语（重复造轮子）

## 6. 核心概念：三层冻结模型

| 层 | 冻结什么 | 机制 | 领域 | 对应能力 |
|---|---|---|---|---|
| **① 世界** | 代码 + 依赖 + 文件系统快照 | overlayfs / COW + namespaces + cgroups | OS | 让「回到那个世界重跑」成为可能（git 做不到的） |
| **② 配方** | prompt + 模型 ID/版本 + 采样参数(temperature/seed) + 配置 | lockfile 式 manifest | 数据 / 配置 | 让「agent 实验室」区别于「普通容器快照」 |
| **③ 轨迹** | 一次运行里 agent 的 tool call 序列 + 每次 LLM 输入/输出 | trace 记录 + replay | 可观测 | 压住「概率性 + 自主性」，实现逐帧复现 |

- ① + ② → 「可比较」（A/B、分布对比）
- ③ → 「可复现」（逐帧重放）

三层恰好切进三个领域，是这套方案的骨架：
- **① 世界 = OS 原理**（课程报告的抓手）
- **② 配方 = 数据/配置版本化**（差异化，也是「轻」的真正来源——不用 Docker，一个 manifest + 一个 COW 快照就够）
- **③ 轨迹 = 可观测性**（本人的主场）

## 7. 约束

- **OS 强相关**：冻结 = overlayfs/COW，隔离 = namespaces + cgroups + seccomp
- **轻量**：单机、无 daemon、rootless 优先
- **平台**：Linux（namespaces/cgroups/overlayfs 均为 Linux 特性）；Windows 开发环境走 WSL2

## 8. 架构决策（已定）

1. 站在 kern / go-sandbox 肩膀上，不重写沙箱原语
2. 隔离定位：半受信（自己/团队跑自己的代码），namespace + cgroup + seccomp 足够
3. MVP 脊柱 = ① + ②（世界 + 配方）；③ 先「能记录」，完整确定性 replay 留作进阶
4. 冻结机制：环境 = overlayfs/COW 快照；配方 = lockfile 式 manifest

## 9. MVP 范围（课程交付）

第一阶段：
- 创建 / 冻结一个环境快照（代码 + 依赖 + 文件系统，COW）
- 配方 manifest 版本化（prompt / 模型 / 采样 / 配置）
- 回到某个快照重跑，diff 两个版本
- 记录一次运行的轨迹（基础）
- CLI 形态：单命令「快照 / 重跑 / 对比」

## 10. 留待决定（open questions）

以下为本人（作者）的初步倾向，未最终拍板：

1. **技术栈** —— 未定。倾向 Go（沙箱/容器生态天然、单二进制、go-sandbox 先例）；备选 Rust（kern 先例、最轻）、Python（agent 生态、主场但不够轻）。
2. **MVP 里「Agent」的形态** —— 未定。倾向：MVP 先做「冻结可复现的执行环境」（对任意代码/脚本），LLM agent 作为实验室的「消费者」第二阶段接入；理由：先啃最难也最核心的「冻结世界」，agent 集成是乘法不是加法。
3. **运行形态** —— 倾向 CLI 优先（单命令快照/重跑/对比），daemon + API 后置。
4. **命名** —— 未定。

## 11. 参考资料

- [Sandbox Provider Landscape (2026)](https://github.com/yaalalabs/agent-kernel/blob/f11e3b2ebd22819e049988f73f2cb7b838d1ca1c/docs/specs/494-sandbox-capability/research/provider-landscape.md)
- [Sprites vs E2B vs Modal vs Daytona: Agent Sandboxes Compared](https://fly.io/learn/sprites-vs-e2b-vs-modal-vs-daytona/)
- [Commoditized: The Agent Sandbox Layer Has Become a Utility](https://tech.yahoo.com/ai/meta-ai/articles/commoditized-agent-sandbox-layer-become-202439275.html)
- [kern — daemonless rootless containers](https://github.com/getkern/kern)
- [NsJail](https://android.googleid.googlesource.com/platform/external/nsjail/)
- [go-sandbox](https://pkg.go.dev/github.com/criyle/go-sandbox)
- [sandlock](https://pypi.org/project/sandlock/)
- [OpenHands: A Sandbox for Your Coding Agent, Not Your Laptop](https://blog.elest.io/openhands-give-your-coding-agent-its-own-sandbox-not-your-laptop/)
- [Awesome AI Coding Sandboxes](https://fhiltscher.github.io/awesome-ai-coding-sandboxes/)
- [Build Zero-Trust Code Execution Environments on AWS](https://builder.aws.com/content/3LRg1grZcVpC0hENkMn7xeaIiyC/build-zero-trust-code-execution-environments-on-aws)
