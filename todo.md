# 首页 AI 工作流助手 (Manus 风格) 任务清单

## M0 需求对齐
- [ ] 确认聊天会话持久化方式 (复用 /api/ai/chat/sessions 或新建草稿会话 store)。
- [ ] 明确“生成后自动校验”范围: schema 校验 + 风险摘要 + 是否自动执行沙箱。
- [ ] 明确无验证环境默认策略: 只校验 / 使用默认容器 / 提示用户选择。
- [ ] 确认步骤详情入口形态: 卡片内编辑 / 抽屉 / 弹窗。

## M1 页面布局与组件拆分
- [ ] HomeView 双栏布局: 左侧聊天，右侧工作区。
- [ ] ChatTimeline.vue: 消息列表、AI 状态、滚动到底。
- [ ] RequirementSummaryCard.vue: 需求总结 + 缺失字段提示。
- [ ] StepBuilder.vue: 步骤列表 + 状态标签。
- [ ] StepDetailForm.vue: 按 action 动态表单。
- [ ] YAMLPreview.vue: 预览 + 步骤定位。
- [ ] ValidationPanel.vue: 校验/执行结果展示。
- [ ] RiskConfirmModal.vue: 人工确认 + 原因输入。
- [ ] ProgressPanel.vue: SSE 进度时间线。

## M2 聊天与 AI 交互
- [ ] 接入会话: 新建/恢复聊天会话，持久化消息。
- [ ] Chat 发送触发 /api/ai/workflow/stream，注入可视化 context。
- [ ] AI 回复生成需求总结卡片 + 缺失问题列表。
- [ ] 快捷动作: 生成/校验/修复/执行。
- [ ] 缺失问题以 chips 形式提示，可一键填入输入框。
- [ ] 校验失败后触发 Fix 流程 (yaml + issues)。

## M3 可视化配置能力
- [ ] 目标主机/分组选择器: 标签化 + inventory 建议 + 自由输入。
- [ ] 步骤卡片状态: Draft/Validated/Failed/Risky。
- [ ] 详情编辑表单支持:
  - cmd.run: cmd, dir, env
  - pkg.install: name(s)
  - template.render: src, dest, vars
  - service.ensure: name, state
  - script.shell/python: script 或 script_ref
  - env.set: env
- [ ] 批量应用目标到全部步骤。
- [ ] 步骤复制 / 删除。

## M4 YAML 与可视化同步
- [ ] 可视化 -> YAML 生成(保证 schema 正确)。
- [ ] YAML -> 可视化解析(复用 parseSteps + 自定义补充信息)。
- [ ] 步骤卡片点击弹框 YAML 对应片段,可编辑。
- [ ] 自动同步开关 + 互相同步保护。

## M5 校验与执行
- [x] 接入 /api/workflows/_draft/validate，映射到步骤高亮。
- [x] 接入 /api/ai/workflow/summary 展示风险与摘要。
- [x] 接入 /api/ai/workflow/execute 展示沙箱输出。
- [x] 执行/校验结果同步到聊天时间线。

## M6 草稿历史与 Diff
- [ ] 展示历史快照与 diff 概要。
- [ ] 一键回滚到指定版本。
- [ ] 展示 draft_id 与更新时间。

## M7 风险与人工确认
- [ ] 高风险动作展示风险 Banner。
- [ ] 风险为 high 时必须填写确认原因。
- [ ] 未确认时禁止保存。

## M8 保存与跳转
- [ ] 保存到 /api/workflows/{name} 并跳转编排页。
- [ ] 支持名称校验与提示。

## M9 视觉与响应式
- [ ] Manus 风格主题: 聊天气泡、背景、标签、面板层次。
- [ ] 移动端: 聊天优先 + 工作区抽屉。
- [ ] 关键动效: 消息渐显、步骤更新过渡。

## M10 测试与验收
- [ ] 手工回归: 生成 -> 校验 -> 修复 -> 保存流程。
- [ ] 关键交互: 聊天追问、可视化编辑、YAML 同步、风险确认。
- [ ] 更新验收清单或 README 说明。
