# Loop 输出 JSON 协议

- 输出必须是单个 JSON 对象，不要 Markdown，不要额外说明。
- action 只能是: tool_call | final | need_more_info。
- 每轮只能输出一个 action。

## 字段说明
- action: 必填，动作类型。
- tool: 当 action=tool_call 时必填，工具名称。
- args: 当 action=tool_call 时必填，对象类型。
- yaml: 当 action=final 时必填，完整 workflow YAML 字符串。
- questions: 当 action=need_more_info 时必填，字符串数组。
- message: 可选，用于补充说明（尽量少用）。

## 示例

### 1) 调用工具
{
  "action": "tool_call",
  "tool": "read_file",
  "args": {
    "path": "config.json"
  }
}

### 2) 需要更多信息
{
  "action": "need_more_info",
  "questions": [
    "目标主机有哪些?",
    "需要执行的命令是什么?"
  ]
}

### 3) 完成输出
{
  "action": "final",
  "yaml": "version: v0.1\nname: demo\n..."
}
