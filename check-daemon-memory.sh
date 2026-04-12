#!/usr/bin/env bash
# Verify that a secret value does not appear in the daemon's memory.
#
# Usage:
#   ./check-daemon-memory.sh <secret-value>
#   ./check-daemon-memory.sh $(sauce get github password)

set -euo pipefail

if [[ "${1:-}" == "" ]]; then
    echo "Usage: $0 <secret-value>" >&2
    echo "  e.g. $0 \$(sauce get github password)" >&2
    exit 1
fi

if [[ "$(uname -s)" != "Linux" ]]; then
    echo "This script requires Linux (/proc/<pid>/mem)." >&2
    exit 1
fi

NEEDLE="$1"
PID=$(pgrep -f "sauce.*_serve" 2>/dev/null | head -1 || true)

if [[ -z "$PID" ]]; then
    echo "Daemon not running (no 'sauce _serve' process found)." >&2
    echo "Start it with: sauce daemon start" >&2
    exit 1
fi

echo "Daemon PID: $PID"
echo "Scanning memory for secret value..."

python3 - "$PID" "$NEEDLE" <<'PYEOF'
import re, sys

pid    = int(sys.argv[1])
needle = sys.argv[2].encode()

try:
    with open(f"/proc/{pid}/maps") as f:
        maps = f.readlines()
except PermissionError:
    print(f"Cannot read /proc/{pid}/maps — are you the process owner?", file=sys.stderr)
    sys.exit(2)

found = False
with open(f"/proc/{pid}/mem", "rb") as mem:
    for line in maps:
        parts = line.split()
        if len(parts) < 2 or "r" not in parts[1]:
            continue  # skip non-readable regions
        m = re.match(r"([0-9a-f]+)-([0-9a-f]+)", line)
        if not m:
            continue
        start = int(m.group(1), 16)
        end   = int(m.group(2), 16)
        if end - start > 500 * 1024 * 1024:
            continue  # skip huge mappings (e.g. mmap'd files)
        try:
            mem.seek(start)
            chunk = mem.read(end - start)
            if needle in chunk:
                region = parts[-1] if len(parts) > 4 else "(anonymous)"
                print(f"  FOUND in: {line.strip()}  [{region}]")
                found = True
        except OSError:
            pass  # page disappeared or not accessible

if found:
    print("\nWARNING: secret value is present in daemon memory.")
    sys.exit(1)
else:
    print("OK: secret value not found in daemon memory.")
PYEOF
