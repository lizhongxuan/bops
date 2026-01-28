# 使用说明

## CLI

- plan
  - `bops plan -f examples/simple.yaml`
- apply
  - `bops apply -f examples/simple.yaml --verbose`
- test
  - `bops test -f examples/simple.yaml`
- status
  - `bops status`

## 配置

默认读取 `bops.json` 或环境变量 `BOPS_CONFIG` 指定的配置文件。

示例:
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

### Skill / Agent 配置

在 `bops.json` 中声明 Skills 与 Agents:

```json
{
  "claude_skills": ["demo-ping"],
  "agents": [
    {
      "name": "ops-agent",
      "model": "gpt-4o-mini",
      "skills": ["demo-ping"]
    }
  ],
  "tool_conflict_policy": "error"
}
```

支持环境变量覆盖:
- `BOPS_CLAUDE_SKILLS` (逗号分隔)
- `BOPS_AGENTS` (JSON)
- `BOPS_TOOL_CONFLICT_POLICY` (`error` / `overwrite` / `keep` / `prefix`)

## AI 工作流助手 (Web)

入口: `http://localhost:5173/`，首页提供“生成 → 校验 → 修复 → 保存”的完整链路。

核心流程:
- 输入需求，点击“生成方案”获得 YAML + 步骤预览。
- 可执行“校验 / 沙箱验证”查看错误与输出日志。
- 风险等级为 `high` 或校验失败时，需要人工确认并填写原因后才能保存。
- 保存后自动跳转到工作流编排页。

验证终端:
- 在首页执行“沙箱验证”后，点击“终端详情”进入 `验证终端` 页面查看 stdout/stderr。
- 终端入口路径: `/validation-console`。

审计日志:
- 每次沙箱执行会记录到 `data/validation_audit.log` (JSONL)，便于追踪执行环境与结果。

### 示例 prompt

```
在 web1/web2 上安装 nginx，渲染配置并启动服务
```

### 生成 YAML 样例

```yaml
version: v0.1
name: deploy-nginx
description: install nginx on web hosts

inventory:
  groups:
    web:
      hosts:
        - web1
        - web2

plan:
  mode: manual-approve

steps:
  - name: install nginx
    targets: [web]
    action: pkg.install
    with:
      name: nginx

  - name: render config
    targets: [web]
    action: template.render
    with:
      src: nginx.conf.j2
      dest: /etc/nginx/nginx.conf

  - name: start nginx
    targets: [web]
    action: service.ensure
    with:
      name: nginx
      state: started
```

### 错误场景与修复示例

错误示例:
```yaml
version: v0.1
name: bad-example
steps:
  - name: start nginx
    targets: [web]
    with:
      name: nginx
      state: started
```

校验返回:
```
steps[0] action is required
```

修复后的步骤:
```yaml
- name: start nginx
  targets: [web]
  action: service.ensure
  with:
    name: nginx
    state: started
```

## 内置模块 (actions)

### template.render
- 用途: 渲染模板文件并写入目标路径。
- with 参数:
  - `src`: 模板文件路径 (必填)
  - `dest`: 输出文件路径 (必填)
  - `vars`: 额外模板变量 (可选, map)
  - `mode`: 文件权限 (可选, 例如 `0644`)

示例:
```yaml
- name: render config
  targets: [web]
  action: template.render
  with:
    src: nginx.conf.j2
    dest: /etc/nginx/nginx.conf
```

### pkg.install
- 用途: 安装系统包, 自动选择包管理器 (apt/dnf/yum/apk/pacman)。
- with 参数:
  - `name`: 单个包名
  - `names`: 多个包名列表

示例:
```yaml
- name: install package
  targets: [web]
  action: pkg.install
  with:
    name: nginx
```

### service.ensure
- 用途: 确保服务状态为 started/stopped (systemctl/service/rc-service)。
- with 参数:
  - `name`: 服务名 (必填)
  - `state`: `started` 或 `stopped` (默认 `started`)

示例:
```yaml
- name: ensure service
  targets: [web]
  action: service.ensure
  with:
    name: nginx
    state: started
```

补充: 其他 action 还包括 `cmd.run`、`service.restart`、`script.shell`、`script.python`。

### env.set
- 用途: 写入运行期环境变量，供后续步骤使用。
- with 参数:
  - `env`: key/value 变量表 (必填)

示例:
```yaml
- name: set env
  targets: [local]
  action: env.set
  with:
    env:
      TOKEN: "abc123"
```

### script.shell
- 用途: 运行 shell 脚本 (内联或脚本库引用)。
- with 参数:
  - `script`: 直接脚本文本 (与 `script_ref` 二选一)
  - `script_ref`: 脚本库名称 (与 `script` 二选一)
  - `args`: 传递给脚本的参数列表 (可选)
  - `env`: 环境变量 (可选)
  - `dir`: 工作目录 (可选)

示例:
```yaml
- name: run shell script
  targets: [local]
  action: script.shell
  with:
    script: |
      echo "hello shell"
```

### script.python
- 用途: 运行 python 脚本 (内联或脚本库引用)。
- with 参数:
  - `script`: 直接脚本文本 (与 `script_ref` 二选一)
  - `script_ref`: 脚本库名称 (与 `script` 二选一)
  - `args`: 传递给脚本的参数列表 (可选)
  - `env`: 环境变量 (可选)
  - `dir`: 工作目录 (可选)

示例:
```yaml
- name: run python script
  targets: [local]
  action: script.python
  with:
    script: |
      import platform
      print(platform.platform())
```

## Demo

```bash
go build -o bin/bops ./cmd/bops
./bin/bops plan -f examples/demo.yaml
./bin/bops apply -f examples/demo.yaml --verbose
cat /tmp/bops-demo.txt
```
