## 1. Stop Hook Gate 与终止语义

- [x] 1.1 在 `internal/aiworkflow/agent_loop.go` 中为 `final` 动作增加完成前拦截（stop-hook gate）
- [x] 1.2 实现完成检查失败时的继续迭代逻辑，并将失败原因注入下一轮上下文
- [x] 1.3 定义并回传统一终止原因：`completed`、`max_iters`、`no_progress`、`budget_exceeded`、`context_canceled`

## 2. Completion Contract 与 Guardrails

- [x] 2.1 扩展请求与运行时配置结构，支持 `completion_token`、`completion_checks[]`、`loop_max_iters`、`no_progress_limit`、`per_iter_timeout_ms`、`max_tool_calls`、预算字段
- [x] 2.2 实现 completion evaluator：支持 token 与 objective checks 组合，且 objective checks 作为最终判定依据
- [x] 2.3 实现 guardrails 的统一判定与中断出口，确保各类超限都能给出确定性终止原因

## 3. 持久化 Loop Memory

- [x] 3.1 设计并接入 `LoopMemoryStore` 接口，支持按 `session_key`/`draft_id` 定位会话数据
- [x] 3.2 实现文件后端（file-backed）存储：初始化并维护 `prd.json`、`progress.txt`、checkpoint 快照
- [x] 3.3 实现持久化原子写（临时文件 + rename）与加载时格式校验，避免脏状态生效
- [x] 3.4 实现 durable backend 不可用时的 in-memory fallback，并输出 non-durable warning
- [x] 3.5 在迭代边界接入 load/save 与重启恢复逻辑，验证同会话可续跑

## 4. Ralph 模式路由与兼容性

- [x] 4.1 在 `internal/server/ai.go` 扩展 Ralph profile/mode 与 completion contract 入参解析
- [x] 4.2 实现模式隔离：仅 Ralph mode 启用 stop-hook/guardrails，非 Ralph 路径保持现有行为
- [x] 4.3 增加可控策略开关（默认 opt-in，支持按策略自动路由）并提供回滚开关

## 5. 观测与效果报告

- [x] 5.1 增加会话级结构化遥测字段：session id、mode/profile、terminal reason、iteration 次数、tool-call 统计、checks 结果
- [x] 5.2 实现 before/after 对比报告的数据契约与生成流程（含评估窗口与可比任务集）
- [x] 5.3 报告输出核心指标：completion rate、false-complete rate、handoff rate、迭代分位数、时延与成本
- [x] 5.4 报告输出任务类别与终止原因分解，并包含 rollout release-gate 的 pass/fail 结论
- [x] 5.5 处理数据不完整场景：基线缺失时返回明确的 data-completeness error

## 6. 测试、验证与文档

- [x] 6.1 单元测试：final 被拦截（checks fail）与通过（checks pass）两条主路径
- [x] 6.2 单元测试：max_iters、no_progress、timeout/预算等 guardrails 终止原因正确性
- [x] 6.3 单元测试：memory 初始化、更新、重载、重启恢复、原子写失败与损坏文件处理
- [x] 6.4 集成测试：Ralph 与非 Ralph 两种模式并存且互不回归
- [x] 6.5 更新文档（运行配置、completion checks、guardrails、报告解读）并补充示例
- [x] 6.6 执行并记录验证命令结果（至少 `go test ./internal/aiworkflow...` 与相关 server 测试）
