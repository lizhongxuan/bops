# BOPS 智能运维引擎

将运维需求转成可执行的 YAML 工作流，并提供可视化编排、沙箱验证、AI 生成/修复与可追溯审计能力。

## 可以做什么

- 把自然语言需求转换成标准化工作流 YAML
- 在 Web 页面上可视化编排、查看步骤与执行流程
- 在容器/SSH/Agent 环境中验证工作流并定位失败步骤
- 使用 AI 进行“生成 → 校验 → 修复 → 审核”的闭环
- 复用脚本库与环境变量包，保持流程可移植

## 核心能力

- 工作流模型: 线性步骤编排，结构清晰、易扩展
- 可视化界面: 编排、流程视图与执行控制台
- 沙箱验证: 容器优先，支持 SSH/Agent
- AI 协作: 生成/修复/总结，支持高风险人工确认
- 审计记录: 验证执行日志可追溯

## 技术架构概览

- 后端: Go
- 前端: Vue 3 + Vite
- AI 编排: cloudwego/eino Graph
- 存储: 本地文件 (workflows / scripts / envs / validation_envs)

关键流程:

1) 需求输入 → AI 生成 YAML
2) 校验 / 沙箱验证 → 失败则 AI 修复
3) 风险评估 → 人工确认
4) 保存为工作流 → 编排与执行

## 目录结构

- cmd/             CLI 入口
- internal/        核心逻辑与服务端实现
- web/             前端页面
- docs/            使用与设计文档
- examples/        示例 YAML

## 快速开始

### 1) 启动后端

```bash
go run ./cmd/bops serve
```

### 2) 启动前端

```bash
cd web
npm install
npm run dev
```

默认地址:
- Web: `http://localhost:5173/`
- API: `http://localhost:7070/`

## AI 工作流助手

入口: `http://localhost:5173/`

功能流程:
- 输入需求，点击“生成方案”获取 YAML 与步骤预览
- 校验或沙箱验证确认结果
- 风险为 high 或校验失败时需人工确认并填写原因
- 保存后自动跳转到工作流编排页

示例 prompt:
```
在 web1/web2 上安装 nginx，渲染配置并启动服务
```

生成 YAML 样例:
```yaml
version: v0.1
name: deploy-nginx
steps:
  - name: install nginx
    targets: [web]
    action: pkg.install
    with:
      name: nginx
```

## CLI 示例

```bash
# 计划执行
bops plan -f examples/simple.yaml

# 执行工作流
bops apply -f examples/simple.yaml --verbose

# 查看状态
bops status
```

## 配置

默认读取 `bops.json` 或环境变量 `BOPS_CONFIG` 指定的配置文件。

```json
{
  "log_level": "info",
  "log_format": "json",
  "data_dir": "./data",
  "state_path": "./data/state.json",
  "server_listen": "127.0.0.1:7070",
  "agent_listen": "127.0.0.1:7071"
}
```

## 验证环境 (Validation Envs)

支持类型:
- container (Docker/Podman)
- ssh
- agent

相关接口:
- `GET /api/validation-envs`
- `GET /api/validation-envs/{name}`
- `PUT /api/validation-envs/{name}`
- `DELETE /api/validation-envs/{name}`

## 脚本库

用于复用 shell/python 脚本，支持 `script.shell` / `script.python` 动作。

相关接口:
- `GET /api/scripts`
- `GET /api/scripts/{name}`
- `PUT /api/scripts/{name}`
- `DELETE /api/scripts/{name}`

## 安全与审计

- 高风险动作默认进入人工确认
- 每次沙箱执行会写入 `data/validation_audit.log` (JSONL)

## 文档

- 设计说明: `docs/ai-features-design.md`
- 使用说明: `docs/usage.md`
- 验收清单: `docs/acceptance-checklist.md`

## 许可证

Apache-2.0
