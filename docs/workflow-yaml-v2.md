# 工作流 YAML（v0.2 设想）

> 说明：用于“重新设计工作流 YAML 配置与功能”的初版草案。
> 目标：保持与现有 v0.1 兼容，同时为图编辑与执行反馈预留结构。

## 1. 版本与元信息
```yaml
version: v0.2
name: deploy-nginx
description: install nginx on web hosts
metadata:
  owner: ops-team
  tags: [web, nginx]
  created_by: ai
```

## 2. Inventory 与执行计划
```yaml
inventory:
  groups:
    web:
      hosts:
        - web1
        - web2

plan:
  mode: manual-approve
  strategy: sequential
  timeout: 600
```

## 3. Steps 扩展（兼容旧结构）
- 增加 `id` 用于图与执行事件关联。
- 增加 `depends_on` 支持弱依赖关系（仍保持顺序执行为默认策略）。

```yaml
steps:
  - id: step-1
    name: install nginx
    action: pkg.install
    with:
      name: nginx

  - id: step-2
    name: render config
    action: template.render
    depends_on: [step-1]
    with:
      src: nginx.conf.j2
      dest: /etc/nginx/nginx.conf
```

## 4. 执行反馈结构（可选）
```yaml
runtime:
  trace_id: run-2026-01-01-001
  last_status: success
```

## 5. 兼容策略
- v0.1 不包含 `metadata/steps.id/depends_on/runtime` 时，系统自动补默认值。
- `steps.targets` 继续由执行层处理，AI 生成阶段不写入。

## 6. 后续工作
- 在 workflow loader 中支持 `steps.id` 与 `depends_on` 的解析。
- 与图模型同步时使用 `steps.id` 作为节点主键。
