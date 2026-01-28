import json
import os
import sys


def read_args():
    raw = sys.stdin.read().strip()
    if not raw:
        return {}
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {"_raw": raw}


def main():
    args = read_args()
    host = args.get("host") or os.getenv("BOPS_ARG_HOST") or "unknown"
    count = args.get("count", 1)
    result = {
        "ok": True,
        "host": host,
        "count": count,
        "message": f"Simulated ping to {host} with count={count}."
    }
    sys.stdout.write(json.dumps(result))


if __name__ == "__main__":
    main()
