#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BOPS_URL:-http://127.0.0.1:7070}"
PROMPT="${1:-在 web1/web2 上安装 nginx 并启动服务}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for JSON encoding" >&2
  exit 1
fi

payload=$(PROMPT="$PROMPT" python3 - <<'PY'
import json
import os
prompt = os.environ.get("PROMPT", "")
print(json.dumps({
  "mode": "generate",
  "agent_mode": "multi",
  "agent_name": "architect",
  "agents": ["coder", "reviewer"],
  "prompt": prompt,
  "execute": False
}))
PY
)

log_file="${TMPDIR:-/tmp}/bops-multi-agent-$(date +%s).log"

printf '>> POST %s/api/ai/workflow/stream\n' "$BASE_URL"
printf '>> prompt: %s\n' "$PROMPT"

curl -N "$BASE_URL/api/ai/workflow/stream" \
  -H "Content-Type: application/json" \
  -d "$payload" \
  | tee "$log_file"

printf '\n>> log saved: %s\n' "$log_file"

checks=(
  '"card_type":"plan_step"'
  '"card_type":"subloop"'
  '"card_type":"yaml_patch"'
  '"subagent_summaries"'
)

for pattern in "${checks[@]}"; do
  if ! grep -q "$pattern" "$log_file"; then
    echo "!! missing: $pattern" >&2
    exit 1
  fi
  echo "+ ok: $pattern"
done

echo "OK: multi-agent stream contains step/subloop/patch cards and summaries."
