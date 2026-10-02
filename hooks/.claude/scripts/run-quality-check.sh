#!/usr/bin/env bash
# Stop hook: runs `npm run quality` and blocks the turn from ending when it
# fails, feeding the check's own output back to Claude as the reason.
set -uo pipefail

input="$(cat)"

# Claude Code sets stop_hook_active=true on the *second* Stop event caused by
# this same hook blocking the first one. Skip the check then, so a task that
# can never pass quality doesn't loop forever.
stop_hook_active="$(printf '%s' "$input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)"
if [ "$stop_hook_active" = "true" ]; then
  exit 0
fi

if output="$(npm run quality 2>&1)"; then
  exit 0
fi

reason="$(printf '%s' "$output" | tail -n 60)"
jq -n --arg reason "$reason" '{
  decision: "block",
  reason: ("`npm run quality` failed — fix the issues below before finishing:\n\n" + $reason)
}'
