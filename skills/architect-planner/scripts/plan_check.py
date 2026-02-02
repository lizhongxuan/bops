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


def normalize_steps(payload):
    if isinstance(payload, list):
        return payload
    if isinstance(payload, dict):
        if isinstance(payload.get("steps"), list):
            return payload.get("steps")
    return []


def main():
    payload = load_payload()
    steps = []
    if isinstance(payload, dict) and isinstance(payload.get("steps"), list):
        steps = payload.get("steps")
    elif isinstance(payload, dict) and isinstance(payload.get("plan_json"), str):
        try:
            parsed = json.loads(payload.get("plan_json"))
        except Exception:
            parsed = None
        if parsed is not None:
            steps = normalize_steps(parsed)
    elif isinstance(payload, dict) and isinstance(payload.get("steps"), list):
        steps = payload.get("steps")

    issues = []
    if not steps:
        issues.append("plan steps is empty")
    for idx, step in enumerate(steps):
        if not isinstance(step, dict):
            issues.append(f"steps[{idx}] must be object")
            continue
        name = str(step.get("step_name", "")).strip()
        if not name:
            issues.append(f"steps[{idx}] missing step_name")

    result = {
        "ok": len(issues) == 0,
        "issues": issues,
        "step_count": len(steps)
    }
    sys.stdout.write(json.dumps(result))


if __name__ == "__main__":
    main()
