#!/usr/bin/env bash
# Hermetic test for dedup-db-matrix.sh. `docker` is stubbed on PATH so the digest
# pass runs without a registry or network: the stub maps :latest and :18 to one
# digest and :12 to another, and honours STUB_DOCKER_FAIL to simulate an
# unresolvable digest (verifying fail-open).
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"

stubdir="$(mktemp -d)"
trap 'rm -rf "$stubdir"' EXIT

cat >"$stubdir/docker" <<'STUB'
#!/usr/bin/env bash
# Stub of `docker buildx imagetools inspect <image> --format ...`.
[ "${STUB_DOCKER_FAIL:-0}" = 1 ] && exit 1
img=""; want=0
for a in "$@"; do
  [ "$want" = 1 ] && { img="$a"; break; }
  [ "$a" = inspect ] && want=1
done
case "$img" in
  *:latest | *:18) echo "sha256:same" ;;
  *:12) echo "sha256:twelve" ;;
  *) echo "sha256:other" ;;
esac
STUB
chmod +x "$stubdir/docker"
export PATH="$stubdir:$PATH"

dedup() { "$here/dedup-db-matrix.sh"; }

# 1. Exact (engine,tag) duplicates collapse (pass 1, independent of docker).
out=$(dedup <<'JSON'
[{"engine":"postgres","tag":"12","image":"r/postgres:12"},
 {"engine":"postgres","tag":"12","image":"r/postgres:12"}]
JSON
)
echo "$out" | jq -e 'length == 1' >/dev/null

# 2. Same-digest tags collapse; distinct digest kept; first-listed tag wins.
out=$(dedup <<'JSON'
[{"engine":"postgres","tag":"latest","image":"r/postgres:latest"},
 {"engine":"postgres","tag":"18","image":"r/postgres:18"},
 {"engine":"postgres","tag":"12","image":"r/postgres:12"}]
JSON
)
echo "$out" | jq -e '[.[].tag] == ["latest","12"]' >/dev/null

# 3. Same digest across *different* engines must NOT collapse (keyed by engine).
out=$(dedup <<'JSON'
[{"engine":"postgres","tag":"latest","image":"r/postgres:latest"},
 {"engine":"mysql","tag":"latest","image":"r/mysql:latest"}]
JSON
)
echo "$out" | jq -e 'length == 2' >/dev/null

# 4. Fail-open: unresolvable digest keeps distinct (engine,tag) entries.
out=$(STUB_DOCKER_FAIL=1 dedup <<'JSON'
[{"engine":"postgres","tag":"latest","image":"r/postgres:latest"},
 {"engine":"postgres","tag":"18","image":"r/postgres:18"}]
JSON
)
echo "$out" | jq -e 'length == 2' >/dev/null

echo "dedup-db-matrix_test: PASS"
