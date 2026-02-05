#!/usr/bin/env bash
set -euo pipefail

TOKEN="${TOKEN:-runner-token}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_YAML="${TMP_YAML:-/tmp/pg-restore-smoke.yaml}"

cleanup() {
  if [[ -n "${PIDS:-}" ]]; then
    for pid in ${PIDS}; do
      kill "$pid" >/dev/null 2>&1 || true
    done
  fi
  rm -f "${TMP_YAML}"
}
trap cleanup EXIT

echo "Starting agent servers..."
go run "${ROOT_DIR}/runner/examples/agent-server" --addr :7072 --token "${TOKEN}" >/tmp/agent-7072.log 2>&1 &
PIDS="$!"
go run "${ROOT_DIR}/runner/examples/agent-server" --addr :7073 --token "${TOKEN}" >/tmp/agent-7073.log 2>&1 &
PIDS="${PIDS} $!"
go run "${ROOT_DIR}/runner/examples/agent-server" --addr :7074 --token "${TOKEN}" >/tmp/agent-7074.log 2>&1 &
PIDS="${PIDS} $!"

sleep 1

cat >"${TMP_YAML}" <<'EOF'
version: v0.1
name: pg-restore-smoke
inventory:
  hosts:
    host1:
      address: "http://127.0.0.1:7072"
    host2:
      address: "http://127.0.0.1:7073"
    host3:
      address: "http://127.0.0.1:7074"
steps:
  - name: check backup exists
    targets: [host1]
    action: cmd.run
    with:
      export_vars: true
      cmd: |
        echo "BOPS_EXPORT:BACKUP_OK=true"

  - name: use backup flag
    targets: [host2]
    action: cmd.run
    with:
      export_vars: true
      cmd: |
        if [ "${BACKUP_OK}" != "true" ]; then
          echo "missing BACKUP_OK"
          exit 2
        fi
        echo "BOPS_EXPORT:RESTORE_OK=${BACKUP_OK}"

  - name: verify restore flag
    targets: [host3]
    action: cmd.run
    with:
      cmd: |
        if [ "${RESTORE_OK}" != "true" ]; then
          echo "missing RESTORE_OK"
          exit 3
        fi
EOF

echo "Running smoke workflow..."
go run "${ROOT_DIR}/runner/examples/agent-dispatch" --token "${TOKEN}" "${TMP_YAML}"

echo "OK: runner smoke workflow passed (vars exported via BOPS_EXPORT prefix)."
