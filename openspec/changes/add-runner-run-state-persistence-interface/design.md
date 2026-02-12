## Context

当前 Runner 的运行状态能力分散在多个位置：

- `runner/examples/web-ui/main.go` 用内存 `runHub` 管理流式日志，进程重启后状态丢失。
- `runner/examples/agent-server/main.go` 用内存 `tasks` 保存任务状态，重启后不可追踪历史任务。
- `runner/state` 已有 `RunState`/`StateFile` 模型，但当前未接入 `engine` 执行主链路。

同时，业务方希望：

- 每次运行有唯一标识，可长期查询状态；
- Runner 不直接绑定具体数据库或持久化介质；
- 存储实现由接入系统决定，Runner 只负责状态生成、回调和查询。

这是一项跨 `engine/scheduler/examples/state` 的横切改造，且涉及运行时语义与集成边界，必须先明确设计。

## Goals / Non-Goals

**Goals:**
- 定义统一可插拔的运行状态存储接口，由业务程序实现并注入。
- 为每次 workflow run 生成全局唯一 `run_id`，并建立稳定状态生命周期。
- 在执行过程中持续写入状态快照（run/step/host 级），支持按 `run_id` 查询。
- 提供可选状态回调机制，与存储更新解耦，便于业务系统订阅状态变化。
- 明确重启语义，保证“状态可追踪”和“历史可查”。

**Non-Goals:**
- 本期不规定唯一的持久化后端（MySQL/Redis/文件/KV 由集成方决定）。
- 本期不实现“进程重启后自动恢复继续执行中的任务”。
- 本期不改造 workflow YAML 语法。
- 本期不做分布式一致性调度（仅定义单 run 的状态模型与接口协议）。

## Decisions

### 1) 定义 `RunStateStore` 接口，由宿主业务注入实现

决策：
- 在 `runner/state` 新增面向单次运行的接口（示意）：
  - `CreateRun(ctx, snapshot) error`
  - `UpdateRun(ctx, patch) error`（或 `UpsertRunSnapshot`）
  - `GetRun(ctx, runID) (RunSnapshot, error)`
  - `ListRuns(ctx, filter) ([]RunSummary, error)`（可选）
- Runner 核心只依赖接口，不依赖数据库驱动。
- 保留开发友好的默认实现（`InMemoryStore`），可选保留 `FileStore` 适配器。

理由：
- 满足“持久化位置需协商”的集成诉求。
- 避免 Runner 核心耦合业务基础设施。

备选方案：
- 备选 A：Runner 内置 MySQL 实现并强绑定配置。
  - 不采用：增加部署复杂度，且不适用于多业务场景。
- 备选 B：继续只用现有 `state.Store(Load/Save)` 文件接口。
  - 不采用：粒度过粗，无法高频更新单个 run，且并发写风险高。

### 2) 引入 `RunTracker`，作为 engine 与 store/callback 的桥接层

决策：
- 在 `engine` 层引入 `RunTracker`（或 `RunLifecycle`）：
  - 负责创建 `run_id`；
  - 接收 `StepStart/StepFinish/HostResult` 事件；
  - 组装并提交状态更新到 `RunStateStore`；
  - 同步/异步触发回调 `RunStateNotifier`（可选）。
- `engine.WithRecorder` 机制保留，用组合方式接入 tracker（不破坏现有 recorder）。

理由：
- 不侵入 module/dispatcher 的执行逻辑；
- 状态写入责任集中，便于测试和后续扩展（审计、指标）。

备选方案：
- 在 `executor` 或每个 `module` 内部直接写 store。
  - 不采用：职责分散，重复逻辑多，难以保证状态一致。

### 3) 状态模型采用“快照 + 有限状态机”，主键为 `run_id`

决策：
- 复用并扩展 `state.RunState` 为统一快照模型：
  - `run_id`, `workflow_name`, `status`, `message`, `started_at`, `finished_at`
  - `steps[]`（step 状态）
  - `hosts`（每 step 下 host 状态和输出摘要）
- 定义运行状态机：
  - `queued -> running -> (success | failed | canceled | interrupted)`
- `run_id` 生成规则使用时间有序唯一 ID（优先 ULID；若不引入依赖则用时间戳+随机后缀）。

理由：
- 快照查询最直接，满足“按唯一标识查看一次运行状态”；
- 有限状态机可防止非法状态回写。

备选方案：
- 仅存事件日志，不存快照。
  - 不采用：查询最新状态需要事件回放，复杂度高。

### 4) 明确重启语义：保证“可查”，不承诺“自动续跑”

决策：
- 重启后必须满足：
  - 已落库/落盘 run 状态可继续查询；
  - 历史完成态不丢失。
- 对重启前 `running` 状态 run，启动时可通过 `MarkInterrupted` 归档为 `interrupted`（携带重启原因）。

理由：
- 与当前进程内执行模型一致；
- 对用户“重启不影响”给出可执行定义：不丢运行轨迹，状态可解释。

备选方案：
- 要求首版支持运行恢复。
  - 不采用：需要任务检查点、幂等回放、模块级恢复语义，超出本变更范围。

### 5) 统一查询与回调契约，Runner 负责标准化 payload

决策：
- 对外查询 API 统一按 `run_id`：
  - `GetRunStatus(run_id)` 返回快照与最近错误摘要。
- 回调 payload 固定字段：
  - `run_id`, `workflow_name`, `status`, `step`, `host`, `timestamp`, `error`, `version`
- 回调失败不阻塞主执行链路，采用“best-effort + retry policy”。

理由：
- 保障接入系统与 Runner 的契约稳定；
- 降低回调系统故障对执行主链路影响。

备选方案：
- 各入口（web-ui/agent-server）自定义返回结构。
  - 不采用：协议分裂，跨入口整合困难。

## Risks / Trade-offs

- [Risk] 状态更新频率高导致存储压力和写放大。  
  → Mitigation: 采用增量 patch + 可配置节流（例如 host 输出摘要截断、关键节点写入）。

- [Risk] 并发更新同一 `run_id` 可能引发覆盖。  
  → Mitigation: store 接口要求乐观锁版本号（`version`）或原子 compare-and-set 语义。

- [Risk] 回调系统不稳定影响状态传播。  
  → Mitigation: 回调异步化，失败重试并记录死信，不影响执行结果判定。

- [Risk] 重启后 `running` run 被标记 `interrupted` 可能与真实外部任务状态不一致。  
  → Mitigation: 在状态中记录 `interrupted_reason`，并允许上层系统发起二次对账。

- [Risk] 业务方未实现 store 时行为不一致。  
  → Mitigation: 明确默认行为（仅内存、进程级可见）并在启动日志中警告“未启用持久化存储”。

## Migration Plan

1. 引入新抽象：
   - 在 `runner/state` 定义 `RunStateStore`、`RunStateNotifier`、状态机校验与模型转换。
2. 接入执行链路：
   - 在 `engine.Apply` 增加可选 `RunOptions`（含外部 `run_id`、store、notifier）。
   - 新增 `ApplyWithRun` 返回 `run_id` 与最终状态。
3. 集成 recorder：
   - 实现 `RunTracker` 并与现有 recorder 组合，捕获 step/host 事件写入 store。
4. 示例改造：
   - `runner/examples/web-ui` 与 `runner/examples/agent-server` 增加按 `run_id` 状态查询接口，移除仅内存单点状态依赖。
5. 默认实现与文档：
   - 提供 `InMemoryStore`（开发）与 `FileStoreAdapter`（可选），文档说明业务如何注入自定义实现。
6. 回滚策略：
   - 若集成异常，可关闭 tracker 注入，回退到当前无持久化状态模式，不影响核心执行链路。

## Open Questions

- 持久化的生产默认建议是否提供（例如 SQLite）还是完全交给业务程序实现？
- `ListRuns` 是否属于 Runner 核心接口，还是仅要求 `GetRun(run_id)` 最小集？
- 回调投递是否需要“至少一次”保证（持久化重试队列），还是首版 best-effort 即可？
- `run_id` 是否允许由调用方传入（便于跨系统链路追踪），还是必须由 Runner 生成？
