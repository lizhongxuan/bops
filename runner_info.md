# Runner YAML 字段说明（当前实现）

本文档基于当前仓库 `runner` 代码实现整理，目标是回答两个问题：

1. 现在的 YAML 字段到底支持什么、怎么写。
2. 为了更好支撑生产场景（尤其失败处理），还需要补哪些字段。

## 1. 最小可运行 YAML

```yaml
version: v0.1
name: demo
inventory:
  hosts:
    local:
      address: "http://127.0.0.1:7072"
steps:
  - name: hello
    targets: [local]
    action: cmd.run
    args:
      cmd: echo hello
```

## 2. 顶层字段（Workflow）

| 字段 | 必填 | 类型 | 用法 |
|---|---|---|---|
| `version` | 是 | string | 工作流版本；为空会校验失败。 |
| `name` | 是 | string | 工作流名称；为空会校验失败。 |
| `description` | 否 | string | 描述信息。 |
| `env_packages` | 否 | string[] | 当前执行链路未直接消费，更多用于描述/扩展。 |
| `validation_env` | 否 | string | 当前执行链路未直接消费，更多用于描述/扩展。 |
| `inventory` | 否* | object | 执行目标定义。无 host 时会在执行阶段失败。 |
| `vars` | 否 | map | 全局变量，参与 `when` 判断和后续步骤变量上下文。**注意：不会自动注入 shell 环境变量。** |
| `plan` | 否 | object | 计划元数据，支持 `mode/strategy`（见下）。 |
| `steps` | 是 | array | 步骤列表，不能为空。 |
| `handlers` | 否 | array | 处理器列表，可被 `steps[].notify` 触发。 |
| `tests` | 否 | array | 模型中存在，但当前 apply 链路不执行。 |

`plan` 支持：

| 字段 | 可选值 | 说明 |
|---|---|---|
| `plan.mode` | `manual-approve` / `auto` | 不在该集合会校验失败。 |
| `plan.strategy` | `sequential` | 当前只允许顺序策略。 |

## 3. inventory 字段

```yaml
inventory:
  vars:
    global_key: global_val
  groups:
    pg:
      hosts: [pg1, pg2]
      vars:
        group_key: group_val
  hosts:
    pg1:
      address: "http://127.0.0.1:7072"
      vars:
        host_key: host_val
```

说明：

- `inventory.hosts.<name>.address` 为空时默认使用 host 名称。
- `steps[].targets` 可写 host 名，也可写 group 名。
- 同名变量合并优先级：`inventory.vars < group.vars < host.vars`。

## 4. steps 字段

每个 step 支持字段如下：

| 字段 | 必填 | 类型 | 用法 |
|---|---|---|---|
| `name` | 是 | string | 步骤名；重复会校验失败。 |
| `targets` | 否 | string[] | 目标 host/group；为空表示全部 hosts。 |
| `action` | 是 | string | 模块动作名，如 `cmd.run`。 |
| `args` | 否 | map | 动作参数（大多数 action 必需）。 |
| `must_vars` | 否 | string[] | 执行前校验变量存在，不满足则失败。 |
| `when` | 否 | string | 条件执行表达式。 |
| `loop` | 否 | array | 循环执行；每次注入变量 `item`。 |
| `retries` | 否 | int | 重试次数（总尝试次数 = `retries + 1`）。 |
| `timeout` | 否 | string | 单次尝试超时，格式如 `30s`/`10m`/`6h`。 |
| `continue_on_error` | 否 | bool | 失败后继续后续 step。 |
| `expect_vars` | 否 | string[] | 本 step 必须导出的变量；缺失则失败。 |
| `notify` | 否 | string[] | 触发 handlers。handler 名不存在会校验失败。 |

### 4.1 `when` 表达式语法

支持：

- 布尔字面量：`true/false/yes/no`
- 变量：`${VAR}` 或直接写 `VAR`
- 比较：`== != > < >= <=`
- 逻辑：`&& ||`

示例：

```yaml
when: ${BACKUP_OK} == "true" && ${RETRY_COUNT} < 3
```

### 4.2 失败行为（当前）

- 默认任一 step 执行失败，workflow 立即失败并停止。
- 只有 `continue_on_error: true` 时，step 失败后继续。
- 当前**不支持** `on_error` / `on_timeout` / `finally` 这种编排字段。

### 4.3 变量注入注意事项

- `vars` 不会自动把 `${VAR}` 替换进 `cmd.run/shell.run` 的命令文本。
- `cmd.run/shell.run/script.*` 能读取的是环境变量（`args.env` 或 `env.set`/`BOPS_EXPORT` 注入的 `env`）。
- 如果你要在 shell 里用 `${VAR}`，请先通过 `env.set` 或导出变量写入环境。

## 5. handlers 字段

结构与 step 类似，但用于被 `notify` 调用：

| 字段 | 必填 | 说明 |
|---|---|---|
| `name` | 是 | handler 名称 |
| `action` | 是 | 动作 |
| `args` | 否 | 参数 |
| `when` | 否 | 条件 |
| `retries` | 否 | 重试 |
| `timeout` | 否 | 超时 |

## 6. 内置 action 与参数

> action 来自默认注册表；写不存在的 action 会失败。

### 6.1 `cmd.run`

必填参数：

- `args.cmd`：命令字符串

可选参数：

- `args.dir`：工作目录
- `args.env`：环境变量 map
- `args.export_vars`：是否解析 `BOPS_EXPORT:` 行
- `args.max_output_bytes`：输出截断上限
- `args.output_path`：stdout/stderr 落盘目录

### 6.2 `shell.run`

必填参数：

- `args.script`：shell 脚本内容

可选参数与 `cmd.run` 一致：`dir/env/export_vars/max_output_bytes/output_path`

### 6.3 `script.shell` / `script.python`

必填参数（二选一）：

- `args.script`（内联脚本）或
- `args.script_ref`（脚本仓库引用）

可选参数：

- `args.args`：脚本参数（string 或 list）
- `args.dir` / `args.env` / `args.export_vars` / `args.max_output_bytes` / `args.output_path`

### 6.4 `env.set`

必填参数：

- `args.env`：map，至少一个 key

作用：

- 将 env 写入运行时上下文，后续 `cmd.run/shell.run/script.*` 会读到。

### 6.5 `template.render`

必填参数：

- `args.src`：模板文件路径
- `args.dest`：输出路径

可选参数：

- `args.vars`：模板变量
- `args.mode`：文件权限（如 `0644`）

### 6.6 `wait.event`

- 当前未实现，执行会报错。

## 7. 当前不支持（常见误写）

以下字段不会按你预期生效（多数会被忽略）：

- 顶层：`failure_policy` `env_required` `context` `defaults` `finally`
- step：`id` `type` `host` `run` `on_error` `on_timeout` `set_context`
- retry 子对象：`retry.max_attempts` `retry.interval_sec`
- 秒数字段：`timeout_sec` `poll_interval_sec`

补充：当前 YAML 解析不是严格模式，未知字段通常不会直接报错，而是被静默忽略。

建议统一使用 runner 原生字段：`action + args + retries + timeout + continue_on_error + notify`。

## 8. 建议补充字段（推荐）

下面是针对“失败可观测性 + 长任务控制”最实用的一批扩展字段：

### 8.1 顶层扩展

| 字段 | 建议类型 | 建议语义 |
|---|---|---|
| `env_required` | string[] | 启动前校验必须环境变量，不满足直接失败。 |
| `defaults` | object | step 默认参数，如 `timeout`、`retries`。 |
| `finally` | step[] | 无论成功/失败都执行，适合结果回传。 |
| `strict_schema` | bool | 开启后未知字段直接报错，避免静默忽略。 |

### 8.2 step 扩展

| 字段 | 建议类型 | 建议语义 |
|---|---|---|
| `id` | string | 稳定步骤标识，便于审计/回调。 |
| `on_error` | object/step[] | step 失败时执行补偿或上报。 |
| `on_timeout` | object/step[] | 区分超时与普通失败。 |
| `error_code` | string | 业务错误码（如 `E18xx`），便于外部系统识别。 |
| `retry_delay` | duration | 控制重试间隔，避免热重试。 |
| `retry_backoff` | string/object | 指数退避策略。 |
| `depends_on` | string[] | 显式依赖关系（未来支持 DAG 时使用）。 |

### 8.3 回调/结果字段

| 字段 | 建议类型 | 建议语义 |
|---|---|---|
| `report` | object | 统一回调配置（url、auth、payload 模板、retry）。 |
| `capture` | object | 失败时自动收集日志片段/产物路径。 |

## 9. 一个更稳妥的失败处理写法（现有能力下）

当前不支持 `finally` 时，建议在最后增加显式“结果上报 step”，并把高风险 step 开 `continue_on_error`，再用 `when` 控制分支。例如：

```yaml
steps:
  - name: prepare env
    action: env.set
    args:
      env:
        STANZA: demo
        RESULT_CALLBACK_URL: http://127.0.0.1:8080/callback
        TASK_TOKEN: demo-token

  - name: run incr backup
    action: cmd.run
    continue_on_error: true
    args:
      export_vars: true
      cmd: |
        if pgbackrest --stanza="${STANZA}" --type=incr backup; then
          echo "BOPS_EXPORT:BACKUP_STATUS=success"
        else
          echo "BOPS_EXPORT:BACKUP_STATUS=failed"
          exit 1
        fi

  - name: report result
    action: cmd.run
    args:
      cmd: |
        curl -X POST "${RESULT_CALLBACK_URL}" \
          -H "Authorization: Bearer ${TASK_TOKEN}" \
          -H "Content-Type: application/json" \
          -d "{\"status\":\"${BACKUP_STATUS}\"}"
```

> 该方案的限制是：如果进程被强杀/runner 崩溃，上报 step 仍可能执行不到，因此推荐后续补 `finally` 原生字段。

## 10. Runner 运行状态接口（非 YAML 字段）

除了 YAML 编排字段，Runner 还支持按 `run_id` 的运行状态持久化与查询，方便长任务追踪与重启后排障。

### 10.1 `RunStateStore`（可插拔）

Runner 核心只依赖接口，不直接绑定数据库。业务程序可注入自己的存储实现：

- `CreateRun(ctx, run)`
- `UpdateRun(ctx, run)`
- `GetRun(ctx, runID)`
- `ListRuns(ctx, filter)`
- `MarkInterruptedRunning(ctx, reason)`

默认实现：

- `state.NewInMemoryRunStore()`：仅进程内可见，重启后丢失。
- `state.NewFileStore(path)`：文件持久化，适合单机示例。

### 10.2 生命周期状态机

合法迁移：

`queued -> running -> success/failed/canceled/interrupted`

终态不能回退到 `running`，非法状态迁移会被拒绝。

### 10.3 回调字段契约

状态回调统一 payload 字段：

- `run_id`
- `workflow_name`
- `status`
- `step`
- `host`
- `timestamp`
- `error`
- `version`

回调是异步 best-effort：回调失败不会改变运行结果状态，但会记录 `last_notify_error`。

### 10.4 示例查询接口

- `runner/examples/web-ui`：`GET /run-status?run_id=<id>`
- `runner/examples/agent-server`：`GET /run-status?run_id=<id>`

未知 `run_id` 返回 404（not found）。
