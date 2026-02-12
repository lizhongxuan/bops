## 1. 状态接口与模型

- [x] 1.1 在 `runner/state` 定义 `RunStateStore` 与 `RunStateNotifier` 接口，提供以 `run_id` 为键的单次运行创建/更新/查询能力。
- [x] 1.2 增加运行状态机常量与状态迁移校验（`queued -> running -> success/failed/canceled/interrupted`），并补充单元测试。
- [x] 1.3 重构 `runner/state` 模型，支持运行快照读取与 step/host 级进度的增量更新。
- [x] 1.4 提供默认降级存储实现（内存，及可选文件适配器），并在未配置持久化存储时输出启动告警。
- [x] 1.5 增加 `run_id` 生成与校验工具，并补充唯一性与格式约束测试。

## 2. Engine 生命周期集成

- [x] 2.1 在 `runner/engine` 增加 `ApplyWithRun`（或等价接口），在保持现有 `Apply` 行为不变的前提下返回 `run_id` 与最终运行状态。
- [x] 2.2 实现 `RunTracker` 记录器，捕获 `StepStart`、`HostResult`、`StepFinish` 事件并通过 `RunStateStore` 持久化运行快照。
- [x] 2.3 持久化终态运行状态（`success`、`failed`、`canceled`），并记录消息、时间戳和最近错误摘要。
- [x] 2.4 增加重启对账钩子，将已持久化但仍为 `running` 的运行标记为 `interrupted` 并写入中断原因。

## 3. 回调与查询契约

- [x] 3.1 定义统一回调载荷结构（`run_id`、`workflow_name`、`status`、`step`、`host`、`timestamp`、`error`、`version`）。
- [x] 3.2 实现非阻塞回调投递与重试策略，确保回调失败不会改变运行结果状态。
- [x] 3.3 在服务入口暴露按 `run_id` 查询运行状态能力（至少覆盖 `runner/examples/web-ui` 与 `runner/examples/agent-server`）。
- [x] 3.4 对未知 `run_id` 查询实现明确的 not-found 行为，并补充接口层测试。

## 4. 集成接线

- [x] 4.1 增加注入点，支持业务程序接入自定义 `RunStateStore` 与 `RunStateNotifier` 实现。
- [x] 4.2 更新示例启动路径，统一初始化 tracker/store/notifier，并在执行响应中透传 `run_id`。
- [x] 4.3 确保 scheduler/engine 关键日志带有 `run_id`，便于跨分发、主机结果、终态收敛全链路追踪。

## 5. 文档与验证

- [x] 5.1 更新 `runner_info.md` 与 `runner/README.md`，补充接口契约、降级行为、按 `run_id` 查询与回调用法。
- [x] 5.2 增加集成测试，覆盖生命周期持久化、非法状态迁移拒绝、回调失败隔离。
- [x] 5.3 增加重启场景测试，验证已持久化的 `running` 运行会被对账为 `interrupted` 且仍可按 `run_id` 查询。
- [x] 5.4 运行受影响包的定向测试，并在变更说明中记录验证结果。

## 验证记录

- 2026-02-12：`go test ./runner/...` 全部通过。
