# 首页 AI 工作流助手 (Manus 风格) 任务清单


## M2 聊天与 AI 交互
- [x] 接入会话: 新建/恢复聊天会话，持久化消息。
- [x] Chat 发送触发 /api/ai/workflow/stream，注入可视化 context。
- [x] AI 回复生成需求总结卡片 + 缺失问题列表。
- [x] 快捷动作: 生成/校验/修复/执行。
- [x] 缺失问题以 chips 形式提示，可一键填入输入框。
- [x] 校验失败后触发 Fix 流程 (yaml + issues)。

## M3 可视化配置能力
- [x] 目标主机/分组选择器: 标签化 + inventory 建议 + 自由输入。
- [x] 步骤卡片状态: Draft/Validated/Failed/Risky。
- [x] 详情编辑表单支持:
  - cmd.run: cmd, dir, env
  - pkg.install: name(s)
  - template.render: src, dest, vars
  - service.ensure: name, state
  - script.shell/python: script 或 script_ref
  - env.set: env
- [x] 步骤复制 / 删除。

## M4 YAML 与可视化同步
- [x] 可视化 -> YAML 生成(保证 schema 正确)。
- [x] YAML -> 可视化解析(复用 parseSteps + 自定义补充信息)。
- [x] 步骤卡片点击弹框 YAML 对应片段,可编辑。
- [x] 自动同步开关 + 互相同步保护。

## M5 校验与执行
- [x] 接入 /api/workflows/_draft/validate，映射到步骤高亮。
- [x] 接入 /api/ai/workflow/summary 展示风险与摘要。
- [x] 接入 /api/ai/workflow/execute 展示沙箱输出。
- [x] 执行/校验结果同步到聊天时间线。

## M8 保存与跳转
- [x] 保存到 /api/workflows/{name} 并跳转编排页。
- [x] 支持名称校验与提示。

## M9 视觉与响应式
- [x] Manus 风格主题: 聊天气泡、背景、标签、面板层次。
- [x] 关键动效: 消息渐显、步骤更新过渡。

## M10 测试与验收
- [ ] 手工回归: 生成 -> 校验 -> 修复 -> 保存流程。
- [ ] 关键交互: 聊天追问、可视化编辑、YAML 同步、风险确认。
- [x] 更新验收清单或 README 说明。
