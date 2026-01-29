# AI 生成两阶段规则与提示词设计

## 1. 两阶段生成策略
- **阶段一（规划）**：输出高层次、短步骤的工作流计划，目标步数 <= 8。
- **阶段二（优化）**：仅对“复杂步骤”做细化/优化，避免整条流程过长。
- **失败兜底**：阶段二失败则保留阶段一结果，并附加提示说明。

## 2. 复杂步骤判定规则
满足以下条件中的两项及以上视为“复杂步骤”：
- 动作类型属于 `script.shell` / `script.python` / `template.render`。
- `with` 参数数量 >= 3。
- `cmd` 字段过长（> 80 字符）或包含多命令分隔（`&&`、`;`、换行）。
- `src` 路径较长且包含多级路径（> 40 字符）。

> 判定实现位于 `internal/aiworkflow/plan.go`。

## 3. Prompt 设计
### 3.1 规划 Prompt（阶段一）
目标：
- 仅生成必要的高层步骤（<= 8）。
- 不写 `targets` 字段。
- 输出 JSON（workflow + questions）。

### 3.2 优化 Prompt（阶段二）
目标：
- 只优化“复杂步骤”。
- 保持其它步骤不变。
- 总步骤数不超过 `maxWorkflowStepCount`。

## 4. 可配置参数（未来）
- `plan_step_limit`（默认 8）
- `complex_score_threshold`（默认 2）
- `complex_step_max_candidates`（默认 6）
- `max_with_depth`（默认 3）
- `max_workflow_step_count`（沿用全局上限）

## 5. 质量防护（实现）
- 超过最大步骤数会截断，并记录提示信息。
- 步骤缺失名称会自动补齐。
- 参数嵌套过深会被简化（深层结构转为字符串）。
- 连续重复步骤会被合并。
