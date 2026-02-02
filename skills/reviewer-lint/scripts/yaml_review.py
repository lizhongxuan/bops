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


def main():
    payload = load_payload()
    yaml_text = str(payload.get("yaml", ""))
    issues = []
    if not yaml_text.strip():
        issues.append("yaml is empty")
    lines = yaml_text.splitlines()
    has_steps = any(line.strip().startswith("steps:") for line in lines)
    has_step_name = any(line.strip().startswith("- name:") for line in lines)
    has_action = any(line.strip().startswith("action:") for line in lines)
    has_with = any(line.strip().startswith("with:") for line in lines)

    if not has_steps:
        issues.append("missing steps section")
    if not has_step_name:
        issues.append("missing step name")
    if has_step_name and not has_action:
        issues.append("missing action field")
    if has_step_name and not has_with:
        issues.append("missing with field")

    result = {"ok": len(issues) == 0, "issues": issues}
    sys.stdout.write(json.dumps(result))


if __name__ == "__main__":
    main()
