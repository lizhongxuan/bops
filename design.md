# AI 工作流工作台（Dify 风格）

## 1. 目标
- 将首页改为可拖拽的工作台。
- 右侧保留 AI 聊天抽屉，固定贴边并支持折叠。
- 支持 AI 根据自然语言生成节点与连线关系。
- 生成步骤时先给出整体规划，再对复杂步骤自动优化一次，避免步骤过长、质量不稳。
- 提供节点库与模板，可拖拽加入画布。
- 支持单节点 AI 重新生成。
- 支持沙盒校验（本地/容器/Agent），自动修复后重试直到无错或达上限。
- 工作台内直接执行工作流，展示 Dify 类似的运行状态与日志。
- 重新设计工作流步骤的yaml

## 2. 交互概览
### 2.1 布局
- **左侧**：节点画布（平移/缩放/拖拽/连线/选择）。
- **右侧**：AI 聊天抽屉（默认展开，可折叠）。
- **顶部**：工作流标题、环境选择、运行/校验、保存/导出。
- **侧边/底部**：节点库 + 模板面板（可折叠）。
- **布局参考**：画布与节点布局参考示例图片的“左侧节点库 + 中央画布 + 顶部状态/操作条”的结构，但 CSS 风格保持不变（沿用现有视觉体系，仅调整布局）。

### 2.1 页面布局,分为三个主要区域


#### 2.1.1 左侧：侧边栏与资源面板 (Sidebar & Asset Panel)
- 最左侧导航条: 一级菜单。
- 二级菜单/组件库: 这是一个可折叠的树形列表（Tree View）。
-- 顶部: 有“上传”按钮和“搜索目录”输入框。
-- 列表: 展开后显示具体的节点组件（如“shell脚步”、“cmd命令”等），供用户拖拽使用。

#### 2.1.2 中间：无限画布区域 (Infinite Canvas / Graph Editor)
- 功能: 这是核心操作区，采用网格（Grid）背景。
- 节点 (Nodes):
-- Python 代码输入: 标题栏，代表输入或源头。
-- 功能节点: 标题栏，内部包含参数列表（变量,节点类型等）。
- 连线 (Edges/Links): 使用贝塞尔曲线（Bezier Curves）连接节点，表示数据流向。连接点（Ports）清晰可见。
- 悬浮组件: 顶部有“运行进度”条和当前工作流名称。
- 交互: 鼠标光标（蓝色箭头）显示当前位置。

#### 2.1.3 顶部：全局工具栏 (Header / Toolbar)
- 左侧: Logo
- 中间: 创建工作流按钮、当前工作流ID。
- 右侧: 运行按钮等全局操作


### 2.2 聊天抽屉行为
- 固定在右边缘，展开态的宽度尽量占三分之一屏幕大小,高度比页面小一点带你。
- **折叠态**：窄条按钮 + 未读提示。
- **展开态**：对话、生成/修复/节点重生成入口。
- 当选中节点时，显示“重新生成该节点”的快捷动作。
 - 抽屉状态本地持久化，刷新仍保持折叠/展开。

## 3. 数据模型（工作台图模型）
工作台维护图模型，并映射到现有 YAML。

### 3.1 图模型（前端）
```json
{
  "graph": {
    "nodes": [
      {
        "id": "node-1",
        "type": "action",
        "name": "install nginx",
        "action": "pkg.install",
        "with": { "name": "nginx" },
        "targets": ["web"],
        "meta": {
          "template": "pkg.install",
          "aiGenerated": true
        },
        "ui": { "x": 120, "y": 80 }
      }
    ],
    "edges": [
      { "id": "edge-1", "source": "node-1", "target": "node-2" }
    ],
    "layout": { "direction": "LR" },
    "version": "v1"
  }
}
```

### 3.2 YAML 映射规则
- 节点映射到 `steps[]`（按连线拓扑顺序 + 布局稳定排序）。
- 多父节点时保持稳定拓扑序。
- `targets` 在节点中保存，但生成 AI YAML 时会剥离（沿用现有规则）。
 - **真值源**：YAML 为真值源，图为编辑缓存；当图与 YAML 不一致时，提示用户选择覆盖策略。

### 3.3 草稿存储
- 草稿中同时保存 **graph JSON** 与 YAML：
  - `aiworkflowstore.Draft` 新增 `Graph` 字段。
  - API 返回 `yaml + graph`。
 - 前端节点/布局变更后自动保存草稿（graph + YAML）。

## 4. 节点库与模板
- **节点库**内置默认节点/模板：
  - Actions：`cmd.run`、`pkg.install`、`template.render`、`service.ensure`
  - Scripts：`script.shell`、`script.python`
  - Env：`env.set`
- 模板初期使用本地 JSON，后续支持 API 管理。

建议 API：
- `GET /api/node-templates`
- `PUT /api/node-templates/{name}`
- `DELETE /api/node-templates/{name}`

## 5. AI 能力

### 5.1 从对话生成图
- 使用 `POST /api/ai/workflow/generate` 生成 YAML（返回 `yaml + draft_id`）。
- 图由 YAML 推导（必要时通过 `POST /api/ai/workflow/graph-from-yaml` 生成）。
- 生成流程分两段：
  1) 先输出“整体规划步骤”（高层次、少步骤）。
  2) 对复杂步骤自动执行一次“细化/优化”，降低步骤冗长与质量波动。
- 复杂步骤判定：基于动作类型/参数数量/复杂命令/模板路径等规则（见 `docs/ai-generation-rules.md`）。

### 5.2 节点级重生成
新增/扩展接口：
- `POST /api/ai/workflow/node-regenerate`
```json
{
  "node": { "id": "node-2", "action": "template.render", "with": {...} },
  "neighbors": { "prev": [...], "next": [...] },
  "workflow": { "yaml": "..." },
  "intent": "User wants nginx config rendered"
}
```
返回：
- 更新后的节点字段 + YAML/graph。

### 5.3 聊天抽屉动作
- **生成**：Prompt -> AI -> 图。
- **节点重生成**：选中节点 + 上下文 -> AI -> 更新节点。
- **错误修复**：校验失败后触发 AI 修复。

## 6. 沙盒校验与自动修复
### 6.1 校验环境
- 复用已有校验环境：
  - container / ssh / agent。

### 6.2 自动修复循环
可由前端编排或后端统一接口：
1) 沙盒运行验证。
2) 失败 -> 传错误与 YAML 给 AI 修复。
3) 更新图与 YAML。
4) 再次校验。
5) 成功或达到重试上限。

建议新增接口：
- `POST /api/ai/workflow/auto-fix-run`
```json
{
  "yaml": "...",
  "validation_env": "local-container",
  "max_retries": 2
}
```
返回：
- 流式事件（日志、错误、修复后的 YAML）。
 - 当前实现以 YAML 规则校验 + AI 修复为主，环境执行入口已预留。

## 7. 执行视图（Dify 风格）
- 节点状态：`pending -> running -> success/failed`
- 运行日志在右侧抽屉或底部面板。
- 错误可点击，进入 AI 修复。
 - 运行结束显示摘要（成功/失败统计、耗时、问题列表）。

### 7.1 事件
- 使用 SSE/WebSocket 传输运行事件：
  - `workflow_start`、`step_start`、`step_end`、`step_failed`、`agent_output`、`workflow_end`
- UI 根据事件高亮节点与日志。

## 8. 前端改造

### 8.1 路由
- `/` 替换为 **WorkbenchView**。
- `/workflows/:name/flow` 保持只读或重定向。

### 8.2 新组件
- `WorkbenchView.vue`
  - 画布
  - 节点库
  - AI 聊天抽屉（右侧）
  - 运行/校验控制
  - 状态与日志面板
- `NodeDetailsPanel.vue`
  - 节点参数编辑
  - “重新生成节点”入口

### 8.3 图编辑器能力
- 拖拽节点
- 连接边
- 选择/删除节点与边
- 自动布局

## 9. 后端改造

### 9.1 草稿结构
- `aiworkflowstore.Draft.Graph` 新增图字段（JSON）。

### 9.2 API 扩展
- `GET /api/ai/workflow/draft/{id}` 返回 YAML + graph
- `POST /api/ai/workflow/graph-from-yaml`
- `POST /api/ai/workflow/node-regenerate`
- `POST /api/ai/workflow/auto-fix-run`
- `GET /api/node-templates`
 - `POST /api/runs/workflow`
 - `GET /api/runs/{id}/stream`

## 10. 分阶段落地
1) **Phase 1**：工作台 UI + 图存储 + AI 生成图
2) **Phase 2**：模板库 + 拖拽导入
3) **Phase 3**：节点级 AI 重生成
4) **Phase 4**：自动修复循环 + 执行视图

## 11. 风险与对策
- **YAML 与图不同步**：统一真值源 + 稳定映射规则。
- **节点重生成冲突**：差异合并 + 变化提示。
- **自动修复抖动**：限制重试次数 + 人工确认门槛。
- **复杂图形**：阶段性限制为线性/弱分支结构。

## 12. 交互流程图（PUML 文件清单）
- `docs/puml-generate-workflow.puml`
- `docs/puml-drag-template.puml`
- `docs/puml-regenerate-node.puml`
- `docs/puml-sandbox-auto-fix.puml`
- `docs/puml-run-execution.puml`
- `docs/puml-chat-drawer-toggle.puml`
