# 设计方案：对话生成工作流步骤（首页 AI 助手 + Eino）

## 背景
首页 AI 助手已经通过 `/api/ai/workflow/stream` 进行流式生成，并使用 Eino 图执行流程（`internal/aiworkflow/pipeline.go`）。目标是让对话直接产出具体的步骤，同时形成“澄清 -> 生成 -> 校验 -> 修复”的闭环。

本文给出一套与现有 Eino 架构对齐的最小方案：在必要时主动追问，再生成完整 YAML（非仅步骤）。

## 目标
- 将自由对话转换为结构化、可执行的步骤。
- 在缺失关键信息时先追问再生成，避免不完整/不安全的草稿。
- 输出完整 YAML（version、name、description、inventory、plan、steps）。
- 流式推送进度，保持聊天与草稿同步。

## 非目标
- 不新增与现有首页行为冲突的 UI 规范。
- 不引入新的执行引擎或工作流 schema 变更。

## 用户流程（首页 AI 助手）
1) 用户输入目标需求。
2) 助手提取意图并识别缺失信息。
3) 缺失信息存在则提问。
4) 用户补充后重新生成步骤并更新 YAML 草稿。
5) 校验 -> 修复 -> 总结 -> 可选沙箱执行。
6) 用户保存为工作流。

## 输出结构（YAML 合同）
必须输出完整 YAML（非仅 steps）：

```yaml
version: v0.1
name: <slug>
description: <text>
inventory:
  hosts:
    local:
      address: 127.0.0.1
plan:
  mode: manual-approve
  strategy: sequential
env_packages: []        # 可选
vars: {}                # 可选
steps:
  - name: ...
    action: ...
    with:
      ...
```

步骤规则：
- 必须包含 `name`、`action`、`with`。
- 步骤内避免 `targets`（目标在校验/执行时解析）。
- 仅允许动作白名单：`cmd.run`、`pkg.install`、`template.render`、
  `service.ensure`、`script.shell`、`script.python`、`env.set`。

## Eino 图设计
保留现有图结构，补充“澄清节点”：

```
normalize -> intent_extract -> question_gate -> generate -> validate
    -> (fix loop if issues) -> safety -> execute -> summarize -> human_gate
```

### 节点职责
- `normalize`：
  - 清理输入、应用默认值（plan mode、max retries）。
- `intent_extract`（LLM 或轻量解析）：
  - 构建结构化意图：
    - `goal`、`targets`（可选）、`constraints`、`resources`、`actions`。
  - 识别缺失字段 `missing[]`。
- `question_gate`：
  - 若 `missing[]` 非空，输出问题并短路生成。
  - 否则进入 `generate`。
- `generate`：
  - 输出包含 `workflow` 的 JSON（见下文）。
- `validate`：
  - 复用现有 YAML 校验并收集 issues。
- `fix`：
  - 复用现有修复提示词修正 YAML。
- `summarize` + `human_gate`：
  - 沿用风险评估与人工确认逻辑。

## LLM 输出合同
为避免污染 YAML，要求返回 wrapper JSON：

```json
{
  "workflow": {
    "version": "v0.1",
    "name": "deploy-nginx",
    "description": "install nginx and start service",
    "inventory": { "hosts": { "local": { "address": "127.0.0.1" } } },
    "plan": { "mode": "manual-approve", "strategy": "sequential" },
    "steps": [
      { "name": "install nginx", "action": "pkg.install", "with": { "name": "nginx" } }
    ]
  },
  "questions": [
    "Which hosts should this run on?",
    "Do you need a custom config file path?"
  ]
}
```

实现要点：
- `extractWorkflowYAML()` 优先解析 `workflow`。
- 将 `questions[]` 暴露到 stream 结果，供前端显示。

## Prompt 策略
System prompt 要求：
- 仅返回 JSON；不确定时将问题写入 `questions[]`。
- 强制 schema + 动作白名单。
- 不允许 markdown 和额外字段（仅 `workflow`、`questions`）。

用户 prompt 模板（后端生成）：
```
Context:
- env packages: [...]
- validation env: ...
- plan mode: ...

User request:
<user message>

Return JSON only with keys: workflow, questions.
```

## API / Streaming 行为
扩展 stream 结果包含问题：
```json
{
  "yaml": "...",
  "issues": ["..."],
  "risk_level": "low",
  "questions": ["..."],
  "history": ["..."],
  "draft_id": "..."
}
```

前端行为：
- `questions[]` 存在时展示 chips。
- 点击 chips 追加到输入框并重新生成。

## 数据/状态变更
后端：
- `State` 增加 `Questions []string`。
- `generate` 节点写入 `Questions`。
- `stream` 输出 `questions`。

前端：
- `applyResult()` 将 `questions` 写入 `pendingQuestions`。
- 校验 issues 作为 fallback 的 questions。

## 安全与约束
- 高风险动作强制 `plan.mode=manual-approve`。
- 步骤数量上限（如 20），超限要求确认。
- 不输出 `targets`。
- 禁止破坏性动作，除非用户明确提出（如 `rm -rf`、`shutdown`）。

## 上线步骤
1) 生成与解析支持 wrapper JSON。
2) stream 输出 questions。
3) 前端展示 questions chips。
4) 观察失败案例并优化提示词与风险规则。

## 待确认问题
- 问题应在生成前完全澄清，还是允许“先出草稿再提问”？
- 是否需要区分 “plan” / “apply” 两类 prompt？
- 如何在不全量重写 YAML 的情况下合并新答案？
