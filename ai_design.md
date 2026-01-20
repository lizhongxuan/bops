# AI 工作流助手 - 聊天优先首页设计 (Manus 风格)

## 1. 目标

- 以聊天为主线，持续追问与确认需求细节。
- 配置尽量可视化，减少用户手写 YAML 的负担。
- 快速闭环: 生成 -> 校验 -> 修复 -> 审核 -> 保存。
- 可追溯: 草稿历史、风险提示、验证输出清晰可见。

## 2. 非目标

- 不替代 Workflow Studio 的高级编辑能力。
- 不支持复杂多分支流程(先聚焦线性步骤)。

## 3. 体验方向

采用 Manus 式双栏结构:

- 左侧: 聊天时间线(需求、澄清、确认、建议)。
- 右侧: 工作流工作区(可视化步骤 + YAML 预览可切换)。

AI 负责引导收集信息，用户以可视化控件补充细节，AI 持续更新工作流草稿。

## 4. 页面

- 左侧: Chat Timeline + 输入框
- 右侧: Workflow Workspace (tabs)
  - 可视化构建(默认),每个步骤有个详情按钮,可以编辑内容
  - YAML 预览(可选)
  - 校验 / 执行日志
  

## 5. 核心交互流程

1) 用户在聊天中描述需求。
2) AI 返回需求总结卡片与缺失信息列表。
3) 用户在聊天中补充或用右侧控件填写。
4) AI 生成草稿并同步更新可视化步骤。
5) 用户调整步骤、目标主机、策略等。
6) 校验或沙箱执行，失败则 AI 修复。
7) 风险高或校验失败时要求人工确认与原因。
8) 保存为工作流，跳转编排页。

## 6. 聊天体验设计

### 消息类型

- 用户消息
- AI 回复
- 需求总结卡片
- 澄清问题
- 动作建议(生成/校验/修复/执行)
- 风险提示

### AI 对话策略

- 在保存前必须确认:
  - 目标主机/分组
  - 动作类型与关键参数
  - 验证环境
- 提供默认值并标注“可修改”。

### 推荐澄清问题

- 目标是哪些主机/分组? 
> 答: 都支持,有个选择框可以选.

- 需要的动作类型是什么?
> 答: 请你给出建议

- 是否需要沙箱验证? 使用哪个环境?
> 答: 可选,若选择了验证环境,就支持AI/手段验证. 

- 执行策略是 manual-approve 还是 auto?
> 答: manual-approve

## 7. 可视化配置 (降低负担)

### A. 需求总结卡片

- 从聊天中解析:
  - 标题、描述
  - 目标(主机/分组)
  - 动作清单
  - 约束(计划策略、重试、变量包)

### B. 目标选择器

- 标签化选择
- 支持从 inventory 读取建议
- 支持自由输入

### C. 步骤构建器

- 步骤列表卡片，显示状态: Draft / Validated / Failed / Risky
- 步骤详情表单(随 action 动态切换):
  - cmd.run: cmd, dir, env
  - pkg.install: name(s)
  - template.render: src, dest, vars
  - service.ensure: name, state
  - script.shell/python: script 或 script_ref

### D. 计划与执行参数

- Plan Mode: manual-approve / auto
- Validation Env: 下拉
- Max Retries
- Env Packages: 多选

### E. YAML 预览

- 可切换
- 点击步骤卡片定位 YAML 对应片段

## 8. 工作流迭代闭环

状态:

- Draft: AI 生成草稿
- Validated: 校验通过
- NeedsFix: 校验/执行失败
- Risky: 高风险命令
- Ready: 已确认可保存

动作:

- Generate: 生成草稿
- Validate: 校验
- Execute: 沙箱执行
- Fix: AI 修复
- Save: 保存

## 9. 流式反馈

- SSE 显示节点进度:
  normalize -> generator -> validator -> safety -> executor -> fixer -> summarizer
- 左侧聊天与右侧进度面板同步更新

## 10. 前端数据模型

```
state = {
  chat: {
    messages: [],
    pendingQuestions: [],
    summary: {}
  },
  draft: {
    yaml: "",
    steps: [],
    history: [],
    issues: [],
    riskLevel: "",
    needsReview: false,
    draftId: ""
  },
  config: {
    targets: [],
    planMode: "manual-approve",
    envPackages: [],
    validationEnv: "",
    maxRetries: 2
  }
}
```

## 11. API 对接

- POST `/api/ai/workflow/stream` (生成/修复 SSE)
- POST `/api/ai/workflow/summary`
- POST `/api/ai/workflow/execute`
- POST `/api/workflows/_draft/validate`
- PUT `/api/workflows/{name}`
- GET `/api/validation-envs`
- GET `/api/envs`

映射建议:

- Generate: 带视觉配置 context 的 stream
- Validate: workflow validate
- Execute: sandbox run
- Fix: stream with yaml + issues

## 12. 错误与风险处理

- 校验失败:
  - 高亮步骤卡片
  - 错误绑定到具体步骤
- 风险高:
  - 风险提示 Banner
  - 保存前必须人工确认 + 原因

## 13. 草稿历史与 Diff

- 每次生成/修复保存快照
- 显示 diff 概要
- 支持一键回滚

## 14. 组件拆分建议

- HomeView (双栏布局)
- ChatTimeline.vue
- RequirementSummaryCard.vue
- StepBuilder.vue
- StepDetailForm.vue
- ValidationPanel.vue
- YAMLPreview.vue
- RiskConfirmModal.vue

## 15. 待确认问题

- 聊天会话是否需要持久化?
> 需要

- 是否默认在生成后自动校验?
> 是校验什么?

- 没有验证环境时的默认策略?
> 你有什么好建议?
