# 多 Agent 工作流创建方案（Coordinator → Coder → Reviewer）

## 目标
- 用户体验：先有 Plan，再逐步生成/修改步骤，卡片实时出现。
- 质量保障：Reviewer 检查 + 可选执行校验（失败重试 3 次）。
- 结构隔离：AI 只处理 steps；其它 YAML 块来自默认模板，仅人工可改。

## 方案设计（概要）
### 角色与职责
- Coordinator
  - 理解需求/意图；缺信息时先提问。
  - 生成 Plan（步骤列表）。
  - Plan 输出后立即推给前端。
- Coder
  - 按 Plan 逐条产出 step JSON。
  - 每产出 1 条：推前端 + 入 Reviewer 队列。
  - Coder 不等待 Reviewer 完成即可继续下一步（异步）。
- Reviewer
  - 消费 Reviewer 队列，检查 step 是否合理。
  - 若开启校验且有主机：执行 step 脚本并根据结果修复，最多 3 次。
  - 审查通过后回写 step（更新状态/摘要）。

### 关键数据结构
- PlanStep: { id, step_name, description, dependencies[], status }
- StepPatch: { step_id, step_name, action, targets, with, summary, source("coder"|"reviewer") }
- ReviewTask: { step_id, patch, attempt, validation_host?, status }
- DraftState: { draft_id, plan[], steps_map, review_queue, review_results, metrics }

### 事件流 / UI 卡片
- plan_step_start / plan_step_done：Plan 生成过程（每条 Plan step 出卡片）
- step_patch_created：Coder 生成 step JSON → 立刻更新步骤卡
- review_start / review_update / review_done：Reviewer 校验状态与摘要
- validation_start / validation_done：校验开关开启时的执行结果
- final_validation / finalize_success / finalize_failed：最终校验与完成

### 什么时候真正修改 YAML
- 只在 **step patch 被接纳** 时更新 steps 结构体。
- YAML 全文由「默认模板 + steps 结构体」渲染生成。
- AI 只看到 steps 片段，绝不处理其它字段。

---

## 实现任务清单（按执行顺序）

### 1. 基础结构与契约
- [x] 新增多 Agent 创建模式配置（mode=multi_create）。
- [x] 定义 PlanStep / StepPatch / ReviewTask / DraftState 结构体与序列化（internal/aiworkflow/*）。
- [x] 新增 steps-only 的 JSON Schema 校验与 normalize（保证字段齐全/安全）。
- [x] 建立 per-draft_id 状态存储（plan/steps/review_queue/metrics）。

### 2. Coordinator：Plan 生成 + 缺信息提问
- [x] 新增 Coordinator prompt（意图确认 + Plan 输出 JSON）。
- [x] 解析 Plan 并 emit plan_step_start/done 事件。
- [x] 若缺信息：直接进入问题补全（question_gate），并暂停后续。
- [x] Plan 结果持久化到 DraftState。

### 3. Coder：逐步生成 step JSON
- [x] 新增 Coder prompt（只生成单条 step JSON）。
- [x] Coder 循环：按 Plan 顺序生成 step patch（逐条）。
- [x] 每条 patch：emit step_patch_created → 更新前端步骤卡。
- [x] 每条 patch：enqueue 到 Reviewer 队列（异步，不阻塞 Coder）。
- [x] Coder 完成后输出 "coder_done" 事件。

### 4. Reviewer：队列校验 + 可选执行
- [x] 建立 Reviewer worker（per draft_id 或全局队列）。
- [x] reviewer 校验逻辑：
  - 静态审查：字段完整性/危险命令/语义冲突。
  - 若开启校验且填写验证主机：构造单步 YAML → 执行 → 失败则修复（最多 3 次）。
- [x] reviewer 修复成功后回写 steps 结构体并 emit review_update/done。
- [x] reviewer 失败超过 3 次 → 标记失败并保留最后版本（需要人工）。

### 5. Finalize：完整 YAML 校验与收尾
- [x] Coder+Reviewer 都完成后，渲染完整 YAML。
- [x] 执行 validator（一次性）；失败则交给 AI 修正（仅 1 次，不重试）。
- [x] 成功则 emit finalize_success；失败 emit finalize_failed。

### 6. 事件与流式输出
- [x] 扩展 SSE/WS 事件类型（plan/step/review/finalize）。
- [x] 统一 event → card 映射：只保留「状态 + step_name + 摘要」。
- [x] 前端卡片点击打开右侧详情（已有，补充新事件数据）。

### 7. UI 体验（最小改动）
- [x] Plan 先展示（步骤占位卡片）。
- [x] 步骤卡实时更新：状态/摘要变化即可。
- [ ] 长对话保持输入框在底部（已有，检查滚动逻辑）。

### 8. 测试与校验
- [x] Plan JSON 解析测试。
- [x] StepPatch normalize + merge 测试。
- [x] Reviewer 重试/回写测试。
- [x] DraftState per-draft_id 隔离测试。

### 9. 监控与日志
- [x] 记录每个 agent 的 prompt/response（已有基础日志）。
- [x] 记录 review/validation 重试次数与耗时。
