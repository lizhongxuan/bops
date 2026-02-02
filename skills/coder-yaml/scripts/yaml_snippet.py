#!/usr/bin/env python3
import json
import sys


def load_payload():
    raw = sys.stdin.read().strip()
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except Exception:
        return {}


def yaml_escape(value):
    if isinstance(value, bool):
        return "true" if value else "false"
    if value is None:
        return "null"
    if isinstance(value, (int, float)):
        return str(value)
    text = str(value)
    if text == "" or ":" in text or "#" in text or "\n" in text:
        return '"' + text.replace('"', '\\"') + '"'
    return text


def emit_yaml(value, indent=0):
    prefix = " " * indent
    if isinstance(value, dict):
        lines = []
        for key, item in value.items():
            if isinstance(item, (dict, list)):
                lines.append(f"{prefix}{key}:")
                lines.extend(emit_yaml(item, indent + 2))
            else:
                lines.append(f"{prefix}{key}: {yaml_escape(item)}")
        return lines
    if isinstance(value, list):
        lines = []
        for item in value:
            if isinstance(item, (dict, list)):
                lines.append(f"{prefix}-")
                lines.extend(emit_yaml(item, indent + 2))
            else:
                lines.append(f"{prefix}- {yaml_escape(item)}")
        return lines
    return [f"{prefix}{yaml_escape(value)}"]


def main():
    payload = load_payload()
    name = str(payload.get("name", "")).strip()
    action = str(payload.get("action", "")).strip()
    with_args = payload.get("with", {})
    issues = []
    if not name:
        issues.append("name is required")
    if not action:
        issues.append("action is required")
    if with_args is None:
        with_args = {}
    snippet = ["- name: " + yaml_escape(name), "  action: " + yaml_escape(action), "  with:"]
    snippet.extend(emit_yaml(with_args, indent=4))
    result = {"yaml": "\n".join(snippet), "issues": issues}
    sys.stdout.write(json.dumps(result))


if __name__ == "__main__":
    main()
