# Runner 轻量分层设计草案

## 目标

- AI/agent 只看 steps，不暴露 inventory 真实配置（IP/凭据/vars）
- 仍保持单次运行体验（用户操作不复杂）
- 与当前 runner YAML 尽量兼容，逐步迁移

## 非目标

- 不引入 Ansible 级别的多层变量优先级体系
- 不做复杂 role/collection 生态

## 结构分层

### 存储分层

- Steps（AI 可见）：`steps.yaml` 或 DB 字段 `steps`
- Inventory（AI 不可见）：`inventory.yaml` 或 DB 字段 `inventory`

### 运行时合并

执行时由服务端合并为完整 workflow：

```
workflow = {
  version, name (from UI),
  inventory (人工填写),
  steps (AI 生成/更新)
}
```

AI 永远只接触 `steps`，不接触 `inventory` 内容。

## YAML 拆分规范

### steps.yaml（AI 可见）

```yaml
version: v0.1
name: <自动填工作流名>
steps:
  - name: install_nginx
    targets: [web]
    action: shell.run
    args:
      script: |
        echo "install nginx"
```

### inventory.yaml（AI 不可见）

```yaml
inventory:
  groups:
    web:
      hosts: [web1, web2]
  hosts:
    web1:
      address: "10.0.0.1"
      vars:
        ansible_user: ubuntu
    web2:
      address: "10.0.0.2"
      vars:
        ansible_user: ubuntu
  vars:
    env: prod
```

## AI 交互策略（强制隔离）

### AI 可见内容

- steps（当前版本）
- 允许的 target 名称列表（仅组名/host 别名，不含地址与凭据）

### AI 不可见内容

- inventory 完整内容
- host 真实地址/凭据/vars

### Prompt 规则

- 系统提示明确：仅修改 steps，不得询问或生成 inventory 信息
- 缺少 target 信息时，只能提示“请用户在 Inventory 页面补充”

## 变量收集与优先级（更新）

### 变量来源与顺序

优先级从高到低：

1. 运行时 vars（用户输入/临时注入）
2. inventory host vars
3. inventory group vars
4. inventory vars
5. system defaults

### 覆盖规则

- 仅允许当前 step 的 `args` 覆盖 vars
- 下一步开始时会丢弃上一步 `args` 的覆盖效果
- `export_vars` 写回全局 vars（供后续步骤使用）

## API/存储改动

### 后端接口

- `GET /workflows/:id/steps`
- `PUT /workflows/:id/steps`（AI 或用户修改）
- `GET /workflows/:id/inventory`
- `PUT /workflows/:id/inventory`（仅人工配置）

### 执行入口

- `POST /workflows/:id/run`
  - 服务端内部合并 `inventory + steps`

## UI 改动

- 工作流编辑页：
  - Steps 编辑区（AI/用户都可用）
  - Inventory 配置区（仅手动编辑）
- 运行按钮默认使用分层合并结果

## 迁移策略

- 旧 workflow.yaml 拆分：
  - 把 `inventory` 拆出存入 `inventory.yaml`
  - `steps` 单独保留
- 如果用户仍上传全量 YAML：
  - 系统自动分拆
  - 后续 AI 只更新 steps 部分

## 风险与防护

- AI 生成 target=“web1”，但 inventory 不存在 → 执行前校验报错
- 只暴露 target alias/组名，绝不暴露地址/凭据

## 最小实现顺序（MVP）

1. 存储拆分：steps + inventory
2. 执行合并
3. AI Prompt 限制：只处理 steps
4. UI 分区：步骤编辑区 + inventory 编辑区

---

# Runner Steps 补充（must_vars）

## 目的

在执行 `steps.yaml` 前，先校验必须变量是否存在，避免“跑到中途才发现变量缺失”的体验。

## 字段定义（steps.yaml）

```yaml
steps:
  - name: restore_pg
    action: shell.run
    targets: [db]
    must_vars:
      - PG_HOST
      - PG_PORT
      - BACKUP_PATH
    args:
      script: |
        echo "restore from ${BACKUP_PATH}"
```

> 约束：`must_vars` 必须是 list（不支持 map）。

## 执行时校验规则（更新）

### 执行前一次性校验

- 仅校验 `must_vars`
- `must_vars` 变量来源：inventory + 之前步骤的 `expect_vars`
- 缺失或为空值（`""` / `null`）直接失败
- 报错信息明确缺失变量列表

### step 执行后校验（expect_vars）

- 每个 step 执行完必须检查 `expect_vars`
- 若没有输出期望变量，立即报错并终止后续执行

## 失败提示示例

```
step restore_pg missing required vars: PG_HOST, BACKUP_PATH
```

## 原始 YAML 保留策略（更新）

- 不保留原始全量 YAML
- 仅保存拆分后的 `steps` 与 `inventory`

## 测试与验证（必须补齐）

- steps-only 执行流程测试
- must_vars 缺失报错测试
- export_vars 对下一步影响测试
- AI 只读 steps 的权限测试
