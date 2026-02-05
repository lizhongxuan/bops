#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BOPS_URL:-http://127.0.0.1:7070}"
PROMPT="${1:-在 web1/web2 上安装 nginx，渲染配置并启动服务}"
TIMEOUT_SECS="${BOPS_TIMEOUT_SECS:-120}"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required" >&2
  exit 1
fi

log_dir="${TMPDIR:-/tmp}"
log_file_1="${log_dir}/bops-adk-flow-1-$(date +%s).log"
log_file_2="${log_dir}/bops-adk-flow-2-$(date +%s).log"

payload_pause=$(PROMPT="$PROMPT" python3 - <<'PY'
import json
import os
prompt = os.environ.get("PROMPT", "")
print(json.dumps({
  "mode": "generate",
  "prompt": prompt,
  "pause_after_step": True
}))
PY
)

payload_resume=$(PROMPT="$PROMPT" python3 - <<'PY'
import json
import os
prompt = os.environ.get("PROMPT", "")
print(json.dumps({
  "mode": "generate",
  "prompt": prompt,
  "pause_after_step": False
}))
PY
)

run_stream_py='import json,sys,time,urllib.request
url=sys.argv[1]
payload=sys.argv[2].encode("utf-8")
log_file=sys.argv[3]
mode=sys.argv[4]
max_seconds=int(sys.argv[5])

def write_log(line):
    with open(log_file, "a", encoding="utf-8") as f:
        f.write(line)
        if not line.endswith("\n"):
            f.write("\n")

def extract_checkpoint(obj):
    if not isinstance(obj, dict):
        return ""
    data = obj.get("data") or {}
    if isinstance(data, dict):
        value = data.get("checkpoint_id")
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""

seen_plan=False
step_patch_count=0
checkpoint_id=""
seen_result=False
start=time.time()
current_event="message"

req=urllib.request.Request(url, data=payload, headers={"Content-Type": "application/json"})
try:
    resp=urllib.request.urlopen(req, timeout=max_seconds)
except Exception as e:
    print(json.dumps({"error": str(e)}))
    sys.exit(2)

for raw in resp:
    if time.time() - start > max_seconds:
        break
    try:
        line=raw.decode("utf-8", errors="ignore").rstrip("\n")
    except Exception:
        continue
    if not line:
        continue
    write_log(line)
    if line.startswith("event:"):
        current_event=line.replace("event:", "").strip()
        continue
    if not line.startswith("data:"):
        continue
    data=line.replace("data:", "", 1).strip()
    if not data:
        continue
    try:
        obj=json.loads(data)
    except Exception:
        continue
    if current_event == "message":
        if isinstance(obj, dict) and obj.get("type") == "plan_ready":
            seen_plan=True
    elif current_event == "card":
        if isinstance(obj, dict) and obj.get("event_type") == "step_patch_created":
            step_patch_count += 1
    elif current_event == "status":
        if isinstance(obj, dict):
            node = obj.get("node")
            event_type = obj.get("event_type")
            status = obj.get("status")
            if (event_type == "checkpoint" or node == "checkpoint") and status == "paused":
                checkpoint_id = extract_checkpoint(obj)
    elif current_event == "result":
        seen_result=True

    if mode == "pause" and checkpoint_id:
        break
    if mode == "finish" and seen_result:
        break

resp.close()
print(json.dumps({
  "seen_plan": seen_plan,
  "step_patch_count": step_patch_count,
  "checkpoint_id": checkpoint_id,
  "seen_result": seen_result
}))
'

printf '>> POST %s/api/ai/workflow/stream (pause after step)\n' "$BASE_URL"
summary_1=$(python3 - <<PY "$BASE_URL/api/ai/workflow/stream" "$payload_pause" "$log_file_1" "pause" "$TIMEOUT_SECS"
$run_stream_py
PY
)

python3 - <<PY "$summary_1"
import json,sys
result=json.loads(sys.argv[1])
if result.get("error"):
    raise SystemExit("request failed: %s" % result["error"])
if not result.get("seen_plan"):
    raise SystemExit("missing plan_ready")
if result.get("step_patch_count", 0) < 1:
    raise SystemExit("missing step_patch_created")
if not result.get("checkpoint_id"):
    raise SystemExit("missing checkpoint_id")
print("+ ok: plan_ready")
print("+ ok: step_patch_created")
print("+ ok: checkpoint paused")
PY

checkpoint_id=$(python3 - <<PY "$summary_1"
import json,sys
result=json.loads(sys.argv[1])
print(result.get("checkpoint_id", ""))
PY
)

printf '>> resume checkpoint: %s\n' "$checkpoint_id"

payload_resume=$(python3 - <<PY "$payload_resume" "$checkpoint_id"
import json,sys
payload=json.loads(sys.argv[1])
payload["resume_checkpoint_id"] = sys.argv[2]
print(json.dumps(payload))
PY
)

summary_2=$(python3 - <<PY "$BASE_URL/api/ai/workflow/stream" "$payload_resume" "$log_file_2" "finish" "$TIMEOUT_SECS"
$run_stream_py
PY
)

python3 - <<PY "$summary_2"
import json,sys
result=json.loads(sys.argv[1])
if result.get("error"):
    raise SystemExit("request failed: %s" % result["error"])
if result.get("step_patch_count", 0) < 1:
    raise SystemExit("resume stream missing step_patch_created")
if not result.get("seen_result"):
    raise SystemExit("resume stream missing result event")
print("+ ok: resume step_patch_created")
print("+ ok: result event")
PY

printf '\nOK: ADK plan-execute + step patch + pause/resume verified.\n'
printf '>> logs:\n- %s\n- %s\n' "$log_file_1" "$log_file_2"
