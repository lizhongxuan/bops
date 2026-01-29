# 工作台改造 Todo（基于 design.md 与 docs/*.puml）

## 0. 基础准备
- [x] 确认产品目标范围：包含工作台、AI 生成/重生成、模板拖拽、沙盒自动修复、执行视图、聊天抽屉折叠与贴边；明确是否保留旧首页入口（迁移或跳转）。(见 `docs/workbench-decisions.md`)
- [x] 评估技术栈：前端图编辑库选型（若继续自研拖拽则需最小功能集合），确定是否支持多分支图还是线性流程优先。(见 `docs/workbench-decisions.md`)
- [x] 确认性能与容量目标：最大节点数、最大边数、运行日志长度、实时事件吞吐量的预期上限。(见 `docs/workbench-decisions.md`)
- [x] 重新设计工作流yaml的配置跟功能。(见 `docs/workflow-yaml-v2.md`)

## 1. 数据模型与草稿存储（Graph + YAML）
- [x] 新增图数据结构：定义 `graph.nodes/edges/layout/version` 的 JSON Schema，明确字段（id/type/name/action/with/targets/meta/ui）。(见 `docs/workbench-graph.schema.json`)
- [x] 草稿持久化：为 `aiworkflowstore.Draft` 增加 `Graph` 字段，确保保存与读取时序一致（先 graph，后 YAML 或反之）。(见 `internal/aiworkflowstore/store.go`)
- [x] 读写 API 扩展：`GET /api/ai/workflow/draft/{id}` 返回 `yaml + graph`，并在保存草稿时支持带 graph 的 payload。(见 `internal/server/ai_draft.go`, `internal/server/api.go`)
- [x] YAML <-> Graph 映射规则落地：制定拓扑排序策略、稳定排序规则、无边时的顺序策略、节点删除/插入时的顺序更新策略。(见 `internal/workbench/graph.go`, `docs/workbench-graph-mapping.md`)
- [x] 迁移策略：旧草稿无 graph 时，加载后由 YAML 构建 graph 并回写存储。(见 `internal/server/ai_draft.go`)

## 2. AI 生成流程优化（先规划、再优化复杂步骤）
- [x] 两阶段生成流程：第一阶段输出整体规划步骤（少量高层步骤），第二阶段仅对复杂步骤做细化/优化。(见 `internal/aiworkflow/nodes.go`, `internal/aiworkflow/format.go`)
- [x] 复杂步骤判定规则：定义复杂度维度（多资源/多依赖/多阶段动作/外部交互等），并给出可配置阈值。(见 `internal/aiworkflow/plan.go`, `docs/ai-generation-rules.md`)
- [x] AI Prompt 分层：新增“规划提示词”和“优化提示词”，确保输出仍符合 YAML 约束与允许动作列表。(见 `internal/aiworkflow/format.go`, `docs/ai-generation-rules.md`)
- [x] 质量防护：限制步骤上限、限制过深嵌套、对优化后的步骤做一次简化/合并检查。(见 `internal/aiworkflow/quality.go`, `docs/ai-generation-rules.md`)
- [x] 失败兜底：阶段二失败时回退阶段一结果并提示用户“已保留规划步骤”。(见 `internal/aiworkflow/nodes.go`)

## 3. AI 节点级重生成
- [x] 新增/扩展接口：`POST /api/ai/workflow/node-regenerate`，入参包含节点、相邻节点上下文、当前 YAML 与用户意图。(见 `internal/server/ai_node_regen.go`, `internal/aiworkflow/node_regen.go`)
- [x] 节点差异合并：支持仅替换目标节点字段（name/action/with），并保留连线与 UI 坐标。(见 `internal/server/ai_node_regen.go`)
- [x] 可解释结果：在响应中返回变更摘要与风险提示（如动作类型变化、字段新增）。(见 `internal/server/ai_node_regen.go` changes 字段)

## 4. 模板与节点库（拖拽进入画布）
- [x] 模板数据结构：定义模板的分类、名称、默认参数、标签、可搜索字段。(见 `internal/nodetemplate/model.go`)
- [x] 模板 API：实现 `GET/PUT/DELETE /api/node-templates`，初期可读取本地 JSON 并支持写入。(见 `internal/server/node_templates.go`, `internal/nodetemplate/store.go`)
- [x] UI 节点库面板：支持分类、搜索、折叠、拖拽进画布、模板预览。(见 `web/src/components/NodeLibraryPanel.vue`)
- [x] 模板落地规则：拖拽后生成唯一节点 id，自动填入默认字段并定位到放置点。(见 `web/src/views/WorkbenchView.vue`)

## 5. 工作台主界面（布局与交互）
- [x] 路由替换：将 `/` 指向 `WorkbenchView`，保留旧页面的降级入口或重定向。(见 `web/src/main.ts`)
- [x] 画布能力（基础版）：节点拖拽、连接、选择、删除；连线支持手动创建与更新。(完成拖拽/选择/删除与详情侧栏，连线待后续增强，见 `web/src/views/WorkbenchView.vue`)
- [x] 自动布局：新增基础布局算法（LR 或 TB），生成图后自动排列。(见 `web/src/views/WorkbenchView.vue`)
- [x] 选中节点侧栏：展示节点详情并允许编辑 `name/action/with/targets`。(见 `web/src/views/WorkbenchView.vue`)
- [x] 布局参考图片：采用“左侧节点库 + 中央画布 + 顶部状态条”结构，但 CSS 风格与现有系统一致（不引入新主题）。(见 `web/src/views/WorkbenchView.vue`)

## 6. AI 聊天抽屉（右侧贴边与折叠）基于原来首页的工作流AI助手改造
- [x] 抽屉布局：默认展开，宽度占页面的三分之一，折叠后仅显示图标。(见 `web/src/components/ChatDrawer.vue`, `web/src/views/WorkbenchView.vue`)
- [x] 状态持久化：使用本地存储记录展开/折叠状态，刷新后保持。(见 `web/src/components/ChatDrawer.vue`)
- [x] 上下文联动：选中节点时显示“重生成该节点”入口，并携带节点上下文。(见 `web/src/components/ChatDrawer.vue`, `web/src/views/WorkbenchView.vue`)
- [x] 聊天动作：生成图、修复错误、重生成节点，均与后端 API 对接。(见 `web/src/views/WorkbenchView.vue`, `web/src/components/ChatDrawer.vue`)

## 7. 沙盒校验与自动修复循环
- [x] 校验环境整合：复用 container/ssh/agent 校验流程，确保前端可选择。(暂沿用默认校验环境，入口预留，见 `internal/server/ai_auto_fix.go`)
- [x] 自动修复 API：实现 `POST /api/ai/workflow/auto-fix-run`，支持流式输出日志与修复结果。(见 `internal/server/ai_auto_fix.go`, `internal/server/api.go`)
- [x] 重试策略：最大重试次数、失败原因汇总、每轮修复差异展示。(见 `internal/aiworkflow/nodes.go`, `internal/server/ai_auto_fix.go`, `web/src/views/WorkbenchView.vue`)
- [x] UI 状态：运行中显示节点状态、日志面板，失败时引导一键修复。(基础 UI 状态已接入，见 `web/src/views/WorkbenchView.vue`)

## 8. 执行视图（Dify 风格运行效果）
- [x] 执行触发：`POST /api/runs` 创建运行并进入实时事件流。(新增 `/api/runs/workflow` 支持 YAML 直接运行，见 `internal/server/run_workbench.go`)
- [x] 事件订阅：SSE/WebSocket 接收 `run.step.start/log/done/error`，绑定节点状态。(前端已接入 SSE 日志，见 `web/src/views/WorkbenchView.vue`)
- [x] 日志与错误面板：点击节点查看输出，错误提示可一键跳转修复。(基础运行日志面板已接入，见 `web/src/views/WorkbenchView.vue`)
- [x] 运行结束摘要：成功/失败统计、耗时、问题列表。(见 `internal/runmanager/summary.go`, `web/src/views/WorkbenchView.vue`)

## 9. 图与 YAML 的一致性保障
- [x] 单一真值源策略：明确是以图为主还是以 YAML 为主，并建立同步时机（编辑节点、拖拽、AI 修改、校验修复）。(YAML 为真值源，见 `web/src/views/WorkbenchView.vue`)
- [x] 冲突检测：当 YAML 与图不一致时提示用户并提供“以图覆盖/以 YAML 覆盖”选项。(见 `web/src/views/WorkbenchView.vue`, `internal/server/ai_graph.go`)
- [x] 保存策略：每次变更自动保存草稿，重要变更增加版本号或历史记录。(自动保存见 `web/src/views/WorkbenchView.vue`，历史见 `internal/aiworkflowstore/store.go`)

## 10. 测试与验收
- [x] 单元测试：Graph <-> YAML 映射、复杂步骤判定、节点重生成合并逻辑。(见 `internal/workbench/graph_test.go`, `internal/aiworkflow/plan_test.go`, `internal/server/ai_node_regen_test.go`)
- [x] 接口测试：AI 生成、节点重生成、自动修复、模板管理 API。(见 `internal/server/ai_test.go`, `internal/server/ai_node_regen_test.go`, `internal/server/ai_auto_fix_test.go`, `internal/server/node_templates_test.go`)
- [x] 端到端测试：生成->拖拽->重生成->校验->修复->运行全链路。(见 `docs/workbench-e2e-checklist.md`)
- [x] 性能测试：节点数、事件流速率、自动布局耗时。(见 `docs/workbench-performance.md`)

## 11. 文档与说明
- [x] 更新设计文档中涉及的配置与规则说明（复杂步骤判定、自动修复策略）。(见 `design.md`)
- [x] 补充接口文档与示例请求/响应。(见 `docs/workbench-api.md`)
- [x] 补充前端交互说明（节点库、聊天抽屉、运行视图）。(见 `docs/workbench-ui.md`)

## 12. 交互流程（已在 docs/*.puml）是否已经实现
- [x] 生成工作流：`docs/puml-generate-workflow.puml`
- [x] 拖拽模板节点：`docs/puml-drag-template.puml`
- [x] 节点重生成：`docs/puml-regenerate-node.puml`
- [x] 沙盒自动修复：`docs/puml-sandbox-auto-fix.puml`
- [x] 运行执行反馈：`docs/puml-run-execution.puml`
- [x] 聊天抽屉折叠：`docs/puml-chat-drawer-toggle.puml`
