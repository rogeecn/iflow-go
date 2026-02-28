#!/usr/bin/env bash
set -euo pipefail

HOST="${IFLOW_HOST:-127.0.0.1}"
PORT="${IFLOW_PORT:-28000}"
TOKEN="${IFLOW_CLIENT_TOKEN:-e794c8fc-8069-4fb2-b2a0-4d03eba4a383}"
MODEL="${IFLOW_MODEL:-glm-5}"
PROMPT="${IFLOW_PROMPT:-你好，请回复：测试通过。}"

URL="http://${HOST}:${PORT}/v1/chat/completions"

run_case() {
  local stream_flag="$1"
  local case_name="$2"
  local payload
  local body_file
  local status
  local failed=0

  payload=$(cat <<JSON
{
  "model": "${MODEL}",
  "messages": [
    {"role": "user", "content": "${PROMPT}"}
  ],
  "stream": ${stream_flag}
}
JSON
)

  body_file=$(mktemp)
  status=$(curl -sS -N -o "${body_file}" -w "%{http_code}" \
    -X POST "${URL}" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${TOKEN}" \
    --data "${payload}" || true)

  echo "[iflow-go test:${case_name}]"
  echo "URL: ${URL}"
  echo "HTTP: ${status}"
  echo "--- response body ---"
  cat "${body_file}"
  echo

  if [[ "${status}" =~ ^[0-9]{3}$ ]] && (( status >= 400 )); then
    failed=1
  fi

  if [[ "${stream_flag}" == "true" && "${failed}" -eq 0 ]]; then
    if ! rg -q "data:" "${body_file}"; then
      echo "stream case validation failed: no SSE data line found" >&2
      failed=1
    fi
  fi

  rm -f "${body_file}"
  return "${failed}"
}

failed=0
run_case "false" "non-stream" || failed=1
run_case "true" "stream" || failed=1
exit "${failed}"
