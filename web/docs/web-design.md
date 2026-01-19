# Web 前端设计 (Workflow 可视化)

目标: 在 `web/` 下构建前端项目, 实现工作流模板的编写、执行、流程可视化与运行详情追踪。

## 设计方向
- 视觉主题: 工业控制台 + 精密仪表盘。强调清晰的数据密度与可追溯性。
- 基调: 明亮灰白底 + 深色文本 + 高对比状态色。避免紫色主色与暗黑模式偏好。
- 字体:
  - 标题: `Space Grotesk`
  - 正文: `IBM Plex Sans`
  - 代码: `JetBrains Mono`
- 颜色 (CSS 变量建议):
  - `--bg`: #F4F1EC
  - `--panel`: #FFFFFF
  - `--ink`: #1B1B1B
  - `--muted`: #6F6F6F
  - `--brand`: #E85D2A (橙红强调色)
  - `--ok`: #2A9D4B
  - `--warn`: #E6A700
  - `--err`: #D0342C
  - `--info`: #2E6FE3
  - `--grid`: #E1DDD6
- 背景: 轻微纸纹+几何网格 (低透明度), 左上角渐变光斑强化“控制台”感。
- 动效:
  - 页面加载: 轻微上浮 + 透明渐入 (120-180ms)。
  - 步骤状态变化: 色块进度条平滑过渡 (200-260ms)。
  - 日志新增: 行高亮闪烁一次 (80ms)。

## 术语与关系
- 工作流 (Workflow) = 模板。类似 Ansible Playbook, 可复用 YAML 流程定义。
- 运行 (Run) = 执行实例。由工作流模板 + 输入参数生成快照与日志。
- 运行控制台 = 单次 Run 详情页 (只读)。
- 执行入口仅在「流程视图」(Plan / Apply / Stop)。

## 信息架构 (IA)
- 左侧导航 (固定):
  - 总览
  - 工作流 (进入选择页)
  - 运行记录 (全局)
  - 设置
- 左侧「当前工作流」卡片:
  - 选中工作流后展示
  - 二级入口: 编排 / 流程视图 / 运行记录 / 运行控制台
- 顶部状态栏:
  - 页面标题 (选中工作流时显示名称)
  - 操作按钮: `切换工作流`
  - `Plan` / `Apply` / `Stop` 仅出现在「流程视图」

## 路由与跳转关系
- `/` 总览
- `/workflows` 工作流选择页
- `/workflows/:name` 编排页 (YAML)
- `/workflows/:name/flow` 流程视图 (Plan/Apply/Stop)
- `/workflows/:name/runs` 工作流运行记录
- `/runs` 全局运行记录
- `/runs/:id` 运行控制台 (单次运行详情)
- 执行后跳转: `flow` -> 创建 `run_id` -> 跳转 `/runs/:id`

## 页面设计

### 0) 工作流选择页
核心目标: 选择一个工作流模板开始编排与运行。

布局:
- 左侧: 最近使用 + 快速筛选
- 右侧: 工作流卡片库 (搜索、状态、标签)
- 点击卡片进入该工作流编排页

### 1) Workflow Studio (编写 YAML)
核心目标: 让用户在一个页面完成 YAML 编写 + 结构预览。

布局 (双列):
1. 左列: YAML 编辑器
   - 代码编辑器 (monospace, 16px)
   - 顶部工具条: `Format`, `Validate`, `Insert Snippet`
   - 状态条: `Valid` / `Invalid` + 错误行定位
2. 右列: 结构预览 + 变量/主机视图
   - Steps 列表 (折叠/展开)
   - Inventory 预览 (host/group)
   - 变量树 (vars/inventory vars)

关键交互:
- 保存: `Ctrl+S` 显示“保存成功”浮层。
- 校验: 错误高亮 + 右侧显示校验列表 (点击跳转行)。
- 插入片段: 弹窗选择片段 (例如 `cmd.run`, `template.render`)。

### 2) Flow View (流程视图与执行入口)
核心目标: 可视化编辑节点, 并作为唯一执行入口。

布局:
- 左侧: 节点画布 (线性/分支连接)
- 右侧: 节点配置面板
- 顶部: `Plan` / `Apply` / `Stop` + 同步状态

关键交互:
- `Plan/Apply` 创建 Run, 返回 `run_id`
- `Stop` 只在本页面出现, 调用停止接口
- 执行后跳转到 `/runs/:id`

### 3) Runs 列表页 (历史任务)
核心目标: 查看任务执行情况与定位失败, 支持全局与单工作流视图。

布局:
- 顶部筛选: 时间范围 / 状态 / 工作流名称
- 列表行:
  - 运行 ID / 工作流名
  - 状态徽标 (success/failed/running)
  - 耗时 / 开始时间
  - 失败步骤 (如有)
  - “查看详情”按钮

### 4) Run Console (单次运行详情)
核心目标: 只读展示运行状态、步骤输出与主机结果。

布局 (上下分屏):
- 上部: 状态与筛选
  - 目标主机过滤
  - 跳转到流程视图
- 下部: 实时输出
  - 左: 步骤列表 (顺序)
  - 右: 输出区 (stdout/stderr 分 tab)
  - 输出高亮 + 搜索过滤

状态:
- Running: 步骤旁显示 “旋转指示” + 进度条。
- Failed: 步骤标红, 自动定位到错误输出。
- Success: 步骤标绿, 输出区显示完整日志。

## 组件设计
- WorkflowPickerCard: 工作流选择卡片 (状态/标签/描述)。
- StatusBadge: success/warn/fail/running 颜色一致。
- StepCard: 左侧小圆点 + 状态条 + 运行时间。
- HostSelector: 多选下拉 + group 标签。
- OutputPanel: 代码风格输出区, 支持搜索。
- Timeline: 线性步骤视图 (强调线性流执行)。

## 数据模型 (前端)
- WorkflowSummary:
  - `name`, `description`, `tags`, `status`, `updated_at`, `last_run_status`
- WorkflowTemplate:
  - `name`, `version`, `description`, `yaml`, `inventory`, `vars`, `steps`, `updated_at`, `updated_by`
- Run:
  - `id`, `workflow_name`, `mode`, `status`, `started_at`, `finished_at`, `duration`, `operator`
- RunStep:
  - `name`, `action`, `status`, `started_at`, `finished_at`, `duration`, `hosts`
- HostResult:
  - `name`, `status`, `stdout`, `stderr`, `exit_code`
- OutputChunk (实时日志):
  - `run_id`, `step`, `host`, `stream`, `message`, `ts`

## 联调 API 约定 (建议)

### Workflow
- `GET /api/workflows?search=&tag=&status=&page=`
  - 返回: `{ items: WorkflowSummary[], total }`
- `GET /api/workflows/{name}`
  - 返回: `WorkflowTemplate`
- `PUT /api/workflows/{name}`
  - 请求: `{ yaml, version?, comment? }`
  - 返回: `{ ok: true }`
- `POST /api/workflows/{name}/validate`
  - 请求: `{ yaml }`
  - 返回: `{ ok, issues[] }`

### Run
- `POST /api/workflows/{name}/plan`
  - 请求: `{ vars?, inventory_override? }`
  - 返回: `{ run_id, status }`
- `POST /api/workflows/{name}/apply`
  - 请求: `{ vars?, inventory_override? }`
  - 返回: `{ run_id, status }`
- `POST /api/runs/{id}/stop`
  - 返回: `{ ok: true }`
- `GET /api/runs?workflow=&status=&from=&to=`
  - 返回: `{ items: Run[], total }`
- `GET /api/workflows/{name}/runs?status=&from=&to=`
  - 返回: `{ items: Run[], total }`
- `GET /api/runs/{id}`
  - 返回: `{ run: Run, steps: RunStep[] }`
- `GET /api/runs/{id}/stream` (SSE/WS)
  - 消息: `OutputChunk`

## 状态枚举
- `run.status`: `queued` / `running` / `success` / `failed` / `stopped`
- `step.status`: `queued` / `running` / `success` / `failed` / `skipped`
- `host.status`: `queued` / `running` / `success` / `failed`

## 响应式设计
- Desktop: 双列/三列布局完整显示。
- Tablet: YAML 编辑器为主, 右侧预览可收起。
- Mobile:
  - YAML 编辑器全屏
  - 步骤与输出切换 tab
  - 执行控制仅在流程视图底部固定栏

## 页面文案建议
- “Plan 将生成变更预览, Apply 将执行实际修改。”
- “当前运行: step=render config / host=web1”
- “失败定位: step=restart nginx, host=web2”
