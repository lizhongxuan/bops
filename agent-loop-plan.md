# bops 自主循环 Agent 改造计划（全量）

目标：将 bops 从“固定流水线（Pipeline）”升级为自主循环 Agent：

Prompt → [Loop: Think → Tool → Observe] → Result

同时保证体验平滑（实时步骤/思考可见）、稳定性可控、成本可控、易回滚。

---

## 0. 现状梳理（必读）

### 当前 bops 形态
- 结构：固定 Pipeline（normalize → intent_extract → question_gate → generator → validator → safety → executor → summarizer → human_gate）
- 优点：可控、确定性强
- 缺点：不具备自适应“多轮工具调用”能力、思考过长、流程不够灵活

### 目标形态
- 结构：Agent Loop（动态回合）
- 每轮：
  1) 模型判断下一步（工具/回答/结束）
  2) 执行工具
  3) 把结果喂回模型
  4) 继续下一轮，直到完成或触发终止条件

---

## 1. 架构改造总览

### 1.1 新增 Loop Orchestrator
新增文件：`internal/aiworkflow/agent_loop.go`
核心职责：
- 管理多轮 Think → Tool → Observe
- 维护 loop_state（step_idx / max_iters / tool_history / last_error / done）
- 控制退出条件（完成 / 超时 / 连续失败 / Token 超限）

核心函数：
```go
RunAgentLoop(ctx, prompt, opts) (state *State, err error)
```

### 1.2 工具调用协议升级（Loop-ready）
修改文件：`internal/aiworkflow/types.go`
- Event 增加：
  - LoopID
  - Iteration
  - AgentStatus（thinking / tool_call / tool_result / done）

### 1.3 模型 Prompt 改造（行动优先）
修改文件：`internal/ai/prompt.go`
- 新增 Loop Prompt 模板：
  - 强制“每轮只做一件事”
  - 必须输出：tool_call 或 final 或 need_more_info
  - 禁止长说明（减少等待）

---

## 2. Loop 流程设计（精细时序）

### 2.1 Loop 状态机
- INIT → THINK → TOOL_CALL → OBSERVE → THINK …
- DONE / FAIL / TIMEOUT / CANCEL 终止

### 2.2 每轮输出结构（示例）
新增文件：`internal/ai/prompt_loop_schema.md`
```json
{"action":"tool_call","tool":"search_file","args":{"pattern":"config/*.json"}}
```
或
```json
{"action":"final","result":"..."}
```

### 2.3 工具执行策略
修改文件：`internal/skills/mcp_client.go`
- Loop 内工具调用必须是非阻塞可中断
- 每轮结束写入 tool_history
- 输出：
  - tool_result（成功/失败）
  - stdout/stderr（若有）

---

## 3. 前端体验改造（Coze 风格）

### 3.1 体验目标
- 思考过程实时显示（reasoning_content）
- 工具步骤逐条出现（function_call/tool_response）
- 流式输出不卡顿
- 每轮结束时状态更新

### 3.2 UI 展示策略
修改文件：`web/src/views/HomeView.vue`
- 支持 loop_id / iteration 分组
- 同一轮逻辑聚合在同一组卡片内

修改文件：`web/src/components/FunctionCallPanel.vue`
- 增加 iteration 显示
- 每轮显示“正在第 N 轮”

---

## 4. 终止条件设计

### 4.1 强制退出条件
- 达到 max_iters（默认 6-8）
- Token 预算达到上限
- 连续失败次数 ≥ 2

### 4.2 软终止条件
- 模型明确输出 final
- 用户点击终止（前端控制）

---

## 5. 成本与性能控制

### 5.1 限制策略
- 每轮 max_tokens 限制
- 低成本模型用于决策（可选）
- 对工具调用结果做摘要（避免上下文膨胀）

### 5.2 规划模型 & 执行模型分离（可选）
- Planner：轻模型（小成本快速决策）
- Executor：强模型（复杂推理）

---

## 6. 可回滚机制

### 6.1 Pipeline 保留
- 保留现有 Pipeline 作为 fallback
- 增加 config 开关：
  - agent_mode = loop | pipeline

### 6.2 同一请求可强制走 pipeline
- 前端/后端参数支持 force_pipeline=true

---

## 7. 测试与验证

### 7.1 后端单测
新增文件：`internal/aiworkflow/agent_loop_test.go`
测试：
- 正常多轮执行
- 工具失败重试
- 达到 max_iters 自动结束

### 7.2 前端 E2E
修改文件：`web/scripts/stream-e2e.mjs`
- 模拟 2-3 轮 loop
- 确认步骤正确分组 + reasoning 流式追加

---

## 8. 观测与日志（体验优化关键）

### 8.1 日志字段
- loop_id
- iteration
- action
- tool_name
- duration

### 8.2 UI 可见反馈
- 正在第 N 轮
- 每轮完成 → 步骤变绿
- 失败 → 标红 + 重试提示

---

## 9. 交付优先级建议

### P0（1-2 周）
- Loop Orchestrator
- ToolCall JSON 协议
- SSE 事件（function_call/tool_response/verbose）

### P1（2-3 周）
- Loop UI 分组
- 轮次标记与状态显示

### P2（3-4 周）
- 多模型分工
- 评审模型
- 成本优化

---

## 10. 预期提升

- 思考时间显著减少（每轮短输出）
- 工具交互频率提升（更像“真人操作”）
- 用户感知更流畅（每轮都有可见步骤）
- 复杂需求更容易逐步解决

---

## 11. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| Loop 过长 | 成本高、用户等待 | max_iters + 策略控制 |
| 工具失败 | 死循环 | 连续失败退出 |
| 输出不稳定 | 体验差 | 强约束 prompt |

---

## 12. 下一步落地

请确认：
- Loop 迭代次数上限
- 是否引入多模型分工
- 是否保留 pipeline fallback

确认后可开始分阶段落地代码改造。
