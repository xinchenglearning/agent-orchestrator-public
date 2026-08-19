#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BOOTSTRAP="$SCRIPT_DIR/bootstrap-m0.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -x "$BOOTSTRAP" ]] || fail "bootstrap-m0.sh is missing or not executable"

TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/agent-orchestrator-bootstrap.XXXXXX")"
trap 'rm -rf "$TMP_ROOT"' EXIT

TARGET="$TMP_ROOT/agent-orchestrator"
MODULE="github.com/xinchenglearning/agent-orchestrator"

"$BOOTSTRAP" "$TARGET" "$MODULE"

required_paths=(
  ".github/workflows/ci.yml"
  "cmd/orch/.gitkeep"
  "docs/adr/0001-core-architecture.md"
  "docs/design/agent-orchestrator.md"
  "docs/exec-plans/mvp.md"
  "internal/adapters/claude/.gitkeep"
  "internal/adapters/codex/.gitkeep"
  "internal/domain/.gitkeep"
  "internal/evidence/.gitkeep"
  "internal/orchestrator/.gitkeep"
  "internal/policy/.gitkeep"
  "internal/redaction/.gitkeep"
  "internal/runtime/process/.gitkeep"
  "internal/runtime/sandbox/.gitkeep"
  "internal/store/.gitkeep"
  "internal/verification/.gitkeep"
  "internal/workspace/.gitkeep"
  "scripts/bootstrap-m0.sh"
  "scripts/test-bootstrap.sh"
  "tests/contract/.gitkeep"
  "tests/e2e/.gitkeep"
  "tests/integration/.gitkeep"
  ".gitignore"
  "CONTRIBUTING.md"
  "README.md"
  "SECURITY.md"
  "go.mod"
  "tasks.md"
)

for path in "${required_paths[@]}"; do
  [[ -e "$TARGET/$path" ]] || fail "missing $path"
done

[[ "$(git -C "$TARGET" branch --show-current)" == "main" ]] || fail "default branch is not main"
grep -Fxq "module $MODULE" "$TARGET/go.mod" || fail "module path is incorrect"
grep -Fxq "go 1.26.0" "$TARGET/go.mod" || fail "Go version is incorrect"
grep -Fq "permissions:" "$TARGET/.github/workflows/ci.yml" || fail "CI permissions are missing"
grep -Fq "client-neutral Host API" "$TARGET/docs/design/agent-orchestrator.md" ||
  fail "design is not client-neutral"
grep -Fq "Collaboration strategies assign roles per run" \
  "$TARGET/docs/design/agent-orchestrator.md" ||
  fail "collaboration roles are provider-bound"
grep -Fq "## Feasibility and lightweight gates" \
  "$TARGET/docs/design/agent-orchestrator.md" ||
  fail "feasibility gates are missing"
grep -Fq "Every extension starts as a real vertical slice" \
  "$TARGET/docs/design/agent-orchestrator.md" ||
  fail "vertical-slice gate is missing"
grep -Fq "## Complexity budget" "$TARGET/docs/design/agent-orchestrator.md" ||
  fail "complexity budget is missing"
grep -Fq "managed child process" "$TARGET/docs/adr/0001-core-architecture.md" ||
  fail "desktop process model is missing"

printf 'preserve\n' >"$TARGET/preserve.txt"
if "$BOOTSTRAP" "$TARGET" "$MODULE" >/dev/null 2>&1; then
  fail "bootstrap overwrote a non-empty directory"
fi
grep -Fxq "preserve" "$TARGET/preserve.txt" || fail "existing file was modified"

printf 'PASS: M0 bootstrap contract\n'
