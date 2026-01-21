# 对话生成步骤任务清单 (基于 design.md)

## M11 对话生成工作流步骤 (Eino)
- [x] 后端: State 增加 `Questions []string`，stream 结果输出 `questions`。
- [x] 后端: 新增 `intent_extract` 节点，输出结构化意图与 `missing[]`。
- [x] 后端: 新增 `question_gate` 节点，缺失信息时返回问题并短路生成。
- [x] 后端: 更新 Eino 图顺序为 `normalize -> intent_extract -> question_gate -> generate -> validate -> fix -> safety -> execute -> summarize -> human_gate`。
- [x] 后端: 更新生成/修复 prompt，强制 JSON wrapper（`workflow` + `questions`）与动作白名单。
- [x] 后端: 更新 `extractWorkflowYAML()`，优先解析 `workflow`，保留 `questions`。
- [x] 后端: 生成 YAML 默认补全 `version/name/description/inventory/plan`。
- [x] 后端: Guardrails（高风险强制 manual-approve、步骤数上限、禁用破坏性动作）。

- [x] 前端: `applyResult()` 支持 `questions` 写入 `pendingQuestions`。
- [x] 前端: 问题 chips 点击追加到输入并重新触发生成。
- [x] 前端: 缺失信息优先显示 `questions`，issues 作为 fallback。

- [x] 文档: 更新 API/stream payload 说明与示例请求响应。
- [x] 测试: Pipeline 单测（intent/question gate、JSON wrapper 抽取）。
- [ ] 验收: 手工回归对话 -> 生成 -> 校验 -> 修复 -> 保存流程。

## 新增模块/文件建议
- [x] `internal/aiworkflow/intent.go`: 意图结构与缺失字段提取。
- [x] `internal/aiworkflow/questions.go`: question gate 节点逻辑。
- [x] `internal/aiworkflow/contract.go`: JSON wrapper 解析与 schema 校验。
- [x] `web/src/lib/ai-questions.ts`（可选）: questions 处理与展示辅助。
