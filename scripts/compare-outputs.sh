#!/usr/bin/env bash
# compare-outputs.sh
# Cross-implementation validation: compares Go and TypeScript status --format json outputs.
# Exits 0 on match (within tolerance), 1 on mismatch.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Allow overriding the Go binary path
GO_BIN="${GO_BIN:-}"
TS_BIN="${TS_BIN:-}"

# Build Go binary if not provided
if [[ -z "$GO_BIN" ]]; then
  GO_BIN="$(mktemp /tmp/codeburn-go-XXXXXX)"
  echo "Building Go binary..."
  CGO_ENABLED=0 go build -o "$GO_BIN" "$ROOT/cmd/codeburn/" 2>&1
  trap "rm -f $GO_BIN" EXIT
fi

# Set TS command
if [[ -z "$TS_BIN" ]]; then
  TS_CMD="npx tsx $ROOT/src/cli.ts"
else
  TS_CMD="$TS_BIN"
fi

echo "Comparing status --format json outputs..."

GO_OUT=$("$GO_BIN" status --format json 2>/dev/null)
TS_OUT=$(eval "$TS_CMD status --format json" 2>/dev/null)

if [[ -z "$GO_OUT" ]]; then
  echo "ERROR: Go produced no output"
  exit 1
fi
if [[ -z "$TS_OUT" ]]; then
  echo "ERROR: TypeScript produced no output"
  exit 1
fi

# Parse values using Python (available everywhere)
compare_json() {
  python3 - "$1" "$2" <<'PYEOF'
import json, sys

go_raw, ts_raw = sys.argv[1], sys.argv[2]
go = json.loads(go_raw)
ts = json.loads(ts_raw)

errors = []

# Currency code must match
if go.get("currency") != ts.get("currency"):
  errors.append(f"currency: go={go.get('currency')} ts={ts.get('currency')}")

# Cost tolerance: within 1% or $0.01
def within(a, b, field):
  diff = abs(a - b)
  rel = abs(a - b) / max(abs(a), abs(b), 0.0001)
  if diff > 0.01 and rel > 0.01:
    errors.append(f"{field}: go={a} ts={b} (diff={diff:.4f}, rel={rel:.2%})")

within(go["today"]["cost"], ts["today"]["cost"], "today.cost")
within(go["month"]["cost"], ts["month"]["cost"], "month.cost")

# Calls tolerance: within 5%
def within_calls(a, b, field):
  diff = abs(a - b)
  rel = abs(a - b) / max(a, b, 1)
  if rel > 0.05:
    errors.append(f"{field}: go={a} ts={b} (diff={diff}, rel={rel:.2%})")

within_calls(go["today"]["calls"], ts["today"]["calls"], "today.calls")
within_calls(go["month"]["calls"], ts["month"]["calls"], "month.calls")

if errors:
  print("MISMATCH:")
  for e in errors:
    print(f"  {e}")
  sys.exit(1)
else:
  print(f"  today.cost : go={go['today']['cost']} ts={ts['today']['cost']} OK")
  print(f"  month.cost : go={go['month']['cost']} ts={ts['month']['cost']} OK")
  print(f"  today.calls: go={go['today']['calls']} ts={ts['today']['calls']} OK")
  print(f"  month.calls: go={go['month']['calls']} ts={ts['month']['calls']} OK")
PYEOF
}

compare_json "$GO_OUT" "$TS_OUT"

echo ""
echo "Comparing export CSV structure..."

GO_CSV="$(mktemp /tmp/go-export-XXXXXX.csv)"
TS_CSV="$(mktemp /tmp/ts-export-XXXXXX.csv)"
trap "rm -f $GO_CSV $TS_CSV" EXIT

"$GO_BIN" export -o "$GO_CSV" 2>/dev/null
eval "$TS_CMD export -o $TS_CSV" 2>/dev/null

# Check both contain the same section headers
go_sections=$(grep "^#" "$GO_CSV" | sort)
ts_sections=$(grep "^#" "$TS_CSV" | sort)

if [[ "$go_sections" != "$ts_sections" ]]; then
  echo "CSV section headers differ:"
  echo "GO:"
  echo "$go_sections"
  echo "TS:"
  echo "$ts_sections"
  exit 1
fi

echo "  CSV section headers match OK"
echo ""
echo "All checks passed."
