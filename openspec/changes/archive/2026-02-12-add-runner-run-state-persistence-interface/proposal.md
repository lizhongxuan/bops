## Why

Runner 当前运行状态主要保存在进程内存中，进程重启后无法保证运行记录和状态连续可查，这会影响长任务可观测性与运维追踪。需要引入基于唯一标识的运行状态持久化能力，并把存储实现与 Runner 解耦，避免把数据库等具体介质硬编码进 Runner。

## What Changes

- 新增运行状态持久化契约：为 Runner 定义统一的 `RunStateStore` 接口（创建/更新/查询），由集成方按业务场景实现具体持久化。
- 新增唯一运行标识模型：每次运行生成稳定 `run_id`，全链路使用 `run_id` 关联状态变更、回调和查询。
- 新增运行状态生命周期事件：标准化记录 `queued/running/success/failed/canceled` 及 step/host 级别进度，支持重启后恢复查询。
- 新增 Runner 回调与查询能力：Runner 负责状态生成、状态回调、按 `run_id` 查询，不直接绑定 MySQL/Redis/文件等存储。
- 新增默认实现与降级策略：提供最小默认 store（如内存或文件）用于本地开发；生产由业务程序注入实现，未注入时明确能力边界和错误提示。
- 补充接口文档与集成指引：说明业务方如何实现 store、如何保证幂等写入、如何处理并发更新和历史归档。

## Capabilities

### New Capabilities
- `runner-run-state-store-interface`: 定义可插拔运行状态存储接口与数据模型，支持 create/update/get/list 的最小能力集合。
- `runner-run-id-status-lifecycle`: 定义基于 `run_id` 的运行状态生命周期与状态迁移规则，保证一次运行可被完整追踪。
- `runner-run-status-callback-query`: 提供运行状态回调与查询能力，支持集成系统按 `run_id` 拉取或接收状态变化。

### Modified Capabilities
- 无（当前 `openspec/specs/` 下无既有 capability 需要变更）。

## Impact

- Affected code:
  - `runner/engine`：在运行生命周期节点写入/更新状态。
  - `runner/scheduler`：补充 host/task 级状态回传到运行状态模型。
  - `runner/examples/agent-server`、`runner/examples/web-ui`：接入 `run_id` 查询与持久化后的状态读取路径。
  - `runner/state`：从当前未集成的数据结构升级为可注入 store 的统一接口层。
- APIs:
  - 需要新增或扩展按 `run_id` 查询运行状态的接口。
  - 需要定义可选状态回调 payload（含 `run_id`、状态、时间戳、错误信息）。
- Dependencies:
  - Runner 核心不新增强依赖数据库；具体存储依赖由集成方实现并注入。
- Systems:
  - 集成系统需要实现并配置状态存储接口，明确数据保留策略、幂等策略与权限控制。
