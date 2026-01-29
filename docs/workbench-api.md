# 工作台 API 说明（示例）

> 说明：以下接口为工作台核心能力的摘要示例，字段会随实现逐步扩展。

## 1. 生成工作流
`POST /api/ai/workflow/generate`

请求：
```json
{
  "prompt": "安装 nginx 并渲染配置"
}
```

响应：
```json
{
  "yaml": "version: v0.1\nname: ...",
  "draft_id": "draft-xxx",
  "message": "生成完成"
}
```

## 2. 生成图（由 YAML 推导）
`POST /api/ai/workflow/graph-from-yaml`

请求：
```json
{
  "yaml": "version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run"
}
```

响应：
```json
{
  "graph": {
    "version": "v1",
    "layout": { "direction": "LR" },
    "nodes": [],
    "edges": []
  }
}
```

## 3. 草稿读取/保存
`GET /api/ai/workflow/draft/{id}`

响应：
```json
{
  "draft": {
    "id": "draft-xxx",
    "yaml": "...",
    "graph": { "version": "v1", "nodes": [], "edges": [] }
  }
}
```

`PUT /api/ai/workflow/draft/{id}`

请求：
```json
{
  "id": "draft-xxx",
  "yaml": "...",
  "graph": { "version": "v1", "nodes": [], "edges": [] }
}
```

## 4. 节点重生成
`POST /api/ai/workflow/node-regenerate`

请求：
```json
{
  "node": { "id": "node-2", "index": 1, "name": "render", "action": "template.render" },
  "neighbors": { "prev": [{"name": "install", "action": "pkg.install"}], "next": [] },
  "workflow": { "yaml": "..." },
  "intent": "换成 Nginx 配置模板"
}
```

响应：
```json
{
  "yaml": "...",
  "graph": { "version": "v1", "nodes": [], "edges": [] },
  "changes": ["action", "with"]
}
```

## 5. 自动修复（流式）
`POST /api/ai/workflow/auto-fix-run`

请求：
```json
{
  "yaml": "...",
  "max_retries": 2
}
```

响应（SSE）：
```
event: status
data: {"status":"start","message":"auto-fix start"}

event: status
data: {"node":"validator","status":"done","message":"..."}

event: result
data: {"yaml":"...","issues":[],"summary":"...","diffs":["+3/-1"]}
```

## 6. 运行执行
`POST /api/runs/workflow`

请求：
```json
{
  "yaml": "version: v0.1\nname: demo\nsteps:\n  - name: step1\n    action: cmd.run"
}
```

响应：
```json
{
  "run_id": "run-xxx",
  "status": "running"
}
```

`GET /api/runs/{id}/stream`

响应（SSE）：
```
event: workflow_start
data: {"run_id":"run-xxx","data":{"status":"running"}}

event: step_start
data: {"step":"step1","data":{"targets":[]}}

event: workflow_end
data: {"data":{"status":"success","total_steps":1,"success_steps":1,"failed_steps":0,"duration_ms":1200}}
```

## 7. 模板库
`GET /api/node-templates`

响应：
```json
{
  "items": [
    { "name": "pkg_install", "category": "actions", "tags": ["pkg"] }
  ]
}
```
