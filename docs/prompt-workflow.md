# Workflow YAML 生成 Prompt 模板

目标: 你只需要描述运维步骤, 把下面模板中的占位内容替换掉, 发送给其他 AI, 对方就能输出可直接用的 YAML。

## 使用方式
1) 复制下面的 Prompt 模板。
2) 把 `<>` 里的内容替换成你的需求。
3) 发送给 AI, 并要求 **只输出 YAML**。

## YAML 格式说明
```yaml
version: v0.1
name: workflow-name
description: short description
env_packages:
  - prod-env

inventory:
  groups:
    web:
      hosts: [host1, host2]
      vars:
        key: value
  hosts:
    host1:
      address: 10.0.0.10
      vars:
        key: value
  vars:
    global_key: value

vars:
  key: value

plan:
  mode: manual-approve
  strategy: sequential

steps:
  - name: step name
    targets: [host1, web]
    action: cmd.run
    with:
      cmd: "echo hello"
    when: "true"
    loop: [a, b, c]
    retries: 1
    timeout: 5s
    notify: [restart nginx]

handlers:
  - name: restart nginx
    action: service.restart
    with:
      name: nginx

tests:
  - name: lint
    action: cmd.run
    with:
      cmd: "echo lint"
```

## 字段功能说明
- version: 工作流版本, 用于兼容控制。
- name: 工作流名称, 必填。
- description: 简短说明。
- inventory: 目标主机与分组定义。
  - groups: 主机组, 每组包含 hosts 与组级 vars。
  - hosts: 主机列表, 每台可定义 address 与 vars。
  - vars: 全局默认变量, 会被组/主机变量覆盖。
- vars: 工作流级变量, 可用于模板渲染或模块参数。
- env_packages: 关联环境变量包列表, 执行时会注入到运行环境变量中。
- plan: 执行策略。
  - mode: manual-approve 或 auto。
  - strategy: 固定 sequential。
- steps: 顺序执行的步骤列表。
  - name: 步骤名称, 必填且唯一。
  - targets: 目标主机或分组名列表, 空则默认所有主机。
  - action: 执行动作, 仅允许 `cmd.run`, `pkg.install`, `template.render`, `service.ensure`, `service.restart`, `env.set`, `script.shell`, `script.python`。
  - with: 动作参数, 由对应模块定义。
  - when: 简单条件, 仅支持 true/false/yes/no。
  - loop: 循环项列表, 每次循环会注入 `item` 变量。
  - retries: 失败重试次数。
  - timeout: 单次执行超时, 例如 `5s`, `1m`。
  - notify: 触发 handlers 的名称列表。
- handlers: 事件触发步骤, 由 notify 触发执行。
- tests: 测试步骤列表, 通常用于 lint 或 dry run。

## Prompt 模板
```text
你是运维工作流编排助手。请只输出一份 YAML（不要解释），严格遵循以下 schema：
- 顶层字段：version, name, description, inventory, vars(可选), plan, steps, handlers(可选), tests(可选)
- 可选顶层字段：env_packages（环境变量包列表）
- plan: mode 只能是 manual-approve 或 auto；strategy 固定 sequential
- steps: 每个步骤包含 name, targets, action, with, when(可选), loop(可选), retries(可选), timeout(可选), notify(可选)
- targets 必须来自 inventory 中的 host 名或 group 名
- 只能使用以下 action: cmd.run, pkg.install, template.render, service.ensure, service.restart, env.set, script.shell, script.python
- 线性执行：按 steps 顺序执行，每一步对目标主机并发完成后再进入下一步
- 不要使用 depends_on/DAG
- cmd.run 不支持 ${var} 变量替换（除非用 env），模板渲染仅支持 template.render
- env.set 用于写入环境变量，后续步骤可通过环境变量读取（例如命令里的 $TOKEN）
- script.shell/script.python 支持 script 或 script_ref（脚本库名称）二选一

环境信息：
- OS/架构：<例如 macOS M1 / Ubuntu 22.04>
- 主机：<例如 local(127.0.0.1) 或 web1(10.0.0.10), web2(10.0.0.11)>

需求描述：
- <步骤 1 需求>
- <步骤 2 需求>
- <步骤 3 需求>

输出要求：
- <例如：每步使用系统内置命令>
- <例如：多行命令用 YAML 的 | 形式>
```

## 示例（macOS M1 内存检查）
```text
你是运维工作流编排助手。请只输出一份 YAML（不要解释），严格遵循以下 schema：
- 顶层字段：version, name, description, inventory, plan, steps
- plan: mode 只能是 manual-approve 或 auto；strategy 固定 sequential
- steps: 每个步骤包含 name, targets, action, with
- targets 必须来自 inventory 中的 host 名
- 只能使用以下 action: cmd.run
- 线性执行：按 steps 顺序执行，每一步对目标主机并发完成后再进入下一步
- 不要使用 depends_on/DAG

环境信息：
- OS/架构：macOS M1
- 主机：local(127.0.0.1)

需求描述：
1) 查看本地内存情况
2) 打印出内存占用最多的程序名
3) 打印其运行时间

输出要求：
- 每步使用 macOS 自带命令完成
- 多行命令用 YAML 的 | 形式
```
