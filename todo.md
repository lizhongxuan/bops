# AI 工作流助手实现任务清单

## M0 需求与规格对齐
- [x] 对齐 ai_design.md 与现有 HomeView 交互，确认保留/替换当前“聊天+模板生成”流程。
- [x] 明确 Eino 落地方式：是否引入 Eino Graph 作为主编排器并替换现有生成/修复接口实现。
- [x] 明确沙箱验证环境优先级：容器/SSH/Agent 的选择与默认策略。

## M1 后端 - Eino 编排与核心能力
- [x] 引入 Eino 依赖并封装统一的 LLM 调用层 (复用 internal/ai client)。
- [x] 新建 AI workflow pipeline 包 (如 internal/aiworkflow)，定义 AIState 与节点接口。
- [x] 实现节点：InputNormalizer、Generator、Validator、SafetyCheck、Executor、Fixer、Summarizer、HumanGate。
- [x] Generator/Fixer 使用结构化输出 (JSON -> YAML)，确保 schema 完整。
- [x] Validator 复用 workflow.Load + Validate 产出 Issues。
- [x] SafetyCheck 增加高危命令规则与风险级别 (riskLevel: low/medium/high)。
- [x] Executor 调用 validationrun.Run 执行沙箱验证，解析失败步骤与日志。
- [x] Fixer 记录 History，避免重复修复循环，限制 MaxRetries。

## M2 后端 - API 与存储
- [x] 现有接口兼容：保留 /api/ai/workflow/generate 与 /api/ai/workflow/fix，内部接入 Eino pipeline。
- [x] 新增接口：/api/ai/workflow/validate、/api/ai/workflow/execute、/api/ai/workflow/summary。
- [x] 新增 SSE/stream 接口，流式返回节点状态 (生成/验证/修复/执行)。
- [x] 扩展存储：记录 AI 工作流草稿、修复历史、diff 摘要 (可新建 store 或复用 aistore)。
- [x] 为接口添加请求/响应结构定义与错误码规范。

## M3 前端 - 首页 AI 工作流助手重构
- [x] 重构 `web/src/views/HomeView.vue` 为新信息架构 (Hero / 输入 / 结果 / 验证 / CTA)。
- [x] 增加高级约束输入：目标环境、执行策略、变量包、重试次数。
- [x] 增加步骤列表与 YAML 双栏视图，支持点击同步定位。
- [x] 加入 Diff 历史与修复时间轴，支持回滚到某次修复版本。
- [x] 展示风险提示与人工确认入口 (HumanGate)。
- [x] 支持 SSE 进度流式更新 (状态、节点、日志)。

## M4 工作流联动
- [x] 生成结果一键保存为工作流 (/api/workflows/:name)，并跳转编排页。
- [x] 对接工作流校验接口，失败时高亮具体步骤。
- [x] 复用 WorkflowStudioView 的步骤解析逻辑，抽成通用 util。

## M5 安全与合规
- [x] 完成高危命令黑名单与白名单规则。
- [x] 高风险操作二次确认与原因输入。
- [x] 沙箱执行完成后自动销毁，记录执行审计日志。

## M6 测试与质量
- [x] 单测：Eino 节点行为、风险检测、YAML 结构化输出。
- [x] 集成测：生成 -> 验证 -> 修复 -> 执行完整链路。
- [x] 前端回归：生成、修复、保存与跳转流程。

## M7 文档与示例
- [x] 更新 README 与 docs，补充 AI 工作流助手使用说明。
- [x] 增加示例 prompt 与生成 YAML 样例。
- [x] 输出错误场景案例与修复示例。
