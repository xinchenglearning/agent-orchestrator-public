#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

usage() {
  printf 'Usage: %s TARGET_DIR GO_MODULE\n' "$(basename "$0")" >&2
  exit 64
}

fail() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 1
}

[[ $# -eq 2 ]] || usage

for command_name in git install mktemp; do
  command -v "$command_name" >/dev/null 2>&1 || fail "required command not found: $command_name"
done

TARGET_INPUT="$1"
MODULE_PATH="$2"
[[ -n "$TARGET_INPUT" ]] || fail "target directory is empty"
[[ "$MODULE_PATH" =~ ^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)+$ ]] || fail "invalid Go module path"

PARENT_DIR="$(cd "$(dirname "$TARGET_INPUT")" && pwd)"
TARGET_DIR="$PARENT_DIR/$(basename "$TARGET_INPUT")"
PROJECT_NAME="$(basename "$TARGET_DIR")"
[[ "$TARGET_DIR" != "/" ]] || fail "refusing to initialize filesystem root"

if [[ -d "$TARGET_DIR" ]] && [[ -n "$(find "$TARGET_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  fail "target directory is not empty: $TARGET_DIR"
fi
[[ ! -e "$TARGET_DIR" || -d "$TARGET_DIR" ]] || fail "target exists and is not a directory"

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[[ -f "$SOURCE_DIR/test-bootstrap.sh" ]] || fail "test-bootstrap.sh must be next to this script"

STAGING_DIR="$(mktemp -d "$PARENT_DIR/.${PROJECT_NAME}.bootstrap.XXXXXX")"
cleanup() {
  rm -rf "$STAGING_DIR"
}
trap cleanup EXIT

mkdir -p \
  "$STAGING_DIR/.github/workflows" \
  "$STAGING_DIR/cmd/orch" \
  "$STAGING_DIR/docs/adr" \
  "$STAGING_DIR/docs/design" \
  "$STAGING_DIR/docs/exec-plans" \
  "$STAGING_DIR/internal/adapters/claude" \
  "$STAGING_DIR/internal/adapters/codex" \
  "$STAGING_DIR/internal/domain" \
  "$STAGING_DIR/internal/evidence" \
  "$STAGING_DIR/internal/orchestrator" \
  "$STAGING_DIR/internal/policy" \
  "$STAGING_DIR/internal/redaction" \
  "$STAGING_DIR/internal/runtime/process" \
  "$STAGING_DIR/internal/runtime/sandbox" \
  "$STAGING_DIR/internal/store" \
  "$STAGING_DIR/internal/verification" \
  "$STAGING_DIR/internal/workspace" \
  "$STAGING_DIR/scripts" \
  "$STAGING_DIR/tests/contract" \
  "$STAGING_DIR/tests/e2e" \
  "$STAGING_DIR/tests/integration"

for directory in \
  cmd/orch \
  internal/adapters/claude \
  internal/adapters/codex \
  internal/domain \
  internal/evidence \
  internal/orchestrator \
  internal/policy \
  internal/redaction \
  internal/runtime/process \
  internal/runtime/sandbox \
  internal/store \
  internal/verification \
  internal/workspace \
  tests/contract \
  tests/e2e \
  tests/integration; do
  : >"$STAGING_DIR/$directory/.gitkeep"
done

printf 'module %s\n\ngo 1.26.0\n' "$MODULE_PATH" >"$STAGING_DIR/go.mod"

cat >"$STAGING_DIR/.gitignore" <<'EOF'
.DS_Store
/.agent-orchestrator/
/bin/
/coverage.out
/dist/
*.test
EOF

cat >"$STAGING_DIR/README.md" <<EOF
# Agent Orchestrator

A lightweight, deterministic, and safety-first orchestrator for coding agents.

## M0 status

The repository contains the architecture records, work-state source, security baseline,
CI contract, and package boundaries required for the M1 vertical slice.

## Core boundaries

- One writer operates in an isolated Git worktree.
- Reviewers consume immutable evidence and receive no repository tools by default.
- Deterministic verification is independent from model review.
- Push, merge, build, and deploy are never automatic product actions.

## Source of truth

- Architecture: [docs/design/agent-orchestrator.md](docs/design/agent-orchestrator.md)
- Decision record: [docs/adr/0001-core-architecture.md](docs/adr/0001-core-architecture.md)
- MVP execution plan: [docs/exec-plans/mvp.md](docs/exec-plans/mvp.md)
- Work state: [tasks.md](tasks.md)
- Security policy: [SECURITY.md](SECURITY.md)

## Development

The Go module is \`$MODULE_PATH\` and targets Go 1.26.

This repository is not licensed for redistribution until the license task in
\`tasks.md\` is resolved.
EOF

cat >"$STAGING_DIR/docs/design/agent-orchestrator.md" <<'EOF'
# Agent Orchestrator Design

## Purpose

Agent Orchestrator coordinates heterogeneous coding agents through one deterministic,
recoverable, and client-neutral control plane. It is a small local control system, not another
agent runtime, model gateway, or collaboration platform.

The design optimizes for two properties in this order:

1. Feasibility: every supported path must work against a real agent and repository.
2. Lightweight operation: the default product remains one Go binary with no mandatory service.

Generality comes from stable boundaries and capability contracts, not from pre-installing every
transport, strategy, storage engine, or client.

## Architecture

```text
CLI / TUI -------- in-process --------\
                                       \
Desktop -------- stdio / local IPC ----> Host API
                                         |
                                  Orchestration kernel
                                  /        |         \
                             Strategy    Policy    State/evidence
                                  \        |         /
                                   Agent runtime gateway
                                      /           \
                              Local process     Remote adapter
```

Dependency direction is always client -> Host API -> kernel -> runtime gateway -> agent.
Clients render state and submit commands but never own workflow state, policy, or recovery.

## Feasibility and lightweight gates

Every extension starts as a real vertical slice and must pass all four gates before it becomes
part of the supported architecture.

| Gate | Required proof | Reject or defer when |
|---|---|---|
| Real execution | One real task completes against an actual agent and temporary repository | Only mocks, prompt examples, or interface sketches exist |
| Contract | Probe, launch, event, cancellation, result, and failure behavior are exercised | The integration depends on terminal text scraping or undocumented assumptions |
| Complexity | Binary, direct dependency, child-process, storage, and configuration deltas are recorded | A simpler standard-library or existing-boundary solution was not evaluated |
| Operations | Debug, recovery, disable, replacement, and failure-degradation paths are documented | The feature requires an unowned service, silent background process, or manual data repair |

A proposal without measured demand stays outside the kernel. Experimental integrations may
live behind build-time or repository boundaries, but they do not expand default runtime
requirements.

## Complexity budget

The default distribution has fixed budgets:

- One Go executable.
- Zero mandatory daemons.
- Zero mandatory external databases or queues.
- Zero mandatory containers, browser runtimes, or desktop runtimes.
- One assigned workspace writer per run.
- Two built-in MVP strategies: `single` and `review`.
- Static, capability-declaring adapters compiled into the binary.
- Local files for atomic state, append-only events, and immutable evidence.

Every new direct runtime dependency requires a written reason and replacement path. Any change
that introduces a permanent process, external storage service, dynamic plugin runtime, remote
control plane, or additional default transport requires a new ADR.

Binary size and startup time receive baselines when the first executable exists. Until then,
changes record dependency and process-count deltas rather than inventing unsupported numeric
targets.

## Client modes

- CLI and TUI call the kernel in-process.
- Desktop clients launch the same binary as a managed child process and use versioned stdio or
  authenticated local IPC.
- CI and SDK clients use the same command and event contracts without interactive presentation.
- Client disconnection does not imply cancellation; cancellation is an explicit command.
- Service mode remains deferred until background execution, multi-client attachment, or remote
  scheduling is demonstrated by a real workflow that cannot use the managed-process model.

The client-neutral Host API contains only commands, ordered events, session lifecycle, and
approval requests. It does not expose provider-specific prompts or terminal rendering.

## Minimal domain contracts

- `Task`: requested outcome, repository, constraints, validation, and risk hints.
- `AgentDescriptor`: identity, transport, capabilities, limits, and probe result.
- `Strategy`: required roles, allowed concurrency, budgets, and stop conditions.
- `Run`: deterministic state, attempts, generation fence, and terminal decision.
- `RunEvent`: ordered facts suitable for CLI, desktop, persistence, and recovery.
- `Evidence`: immutable commit, diff, test, log, and structured-result references.
- `Approval`: human decision with the evidence digest being approved.

Provider data is translated at the runtime gateway and cannot leak into the domain contracts.

## Agent runtime gateway

Adapters normalize heterogeneous runtimes into probe, launch, stream, cancel, resume, and
terminal-result operations. Supported transports are added one at a time after passing the
feasibility gates. A supervised local process is the first transport; structured RPC or remote
APIs are later adapters, not parallel MVP requirements.

An agent descriptor declares structured output, streaming, cancellation, session resume, user
interaction, repository read or write access, and native sandbox support. Unsupported
capabilities are explicit and cause strategy selection to fail closed.

Model-generated structured results require JSON extraction, schema validation, one bounded
format-repair attempt, and a protocol error on repeated failure.

## Collaboration strategies

Collaboration strategies assign roles per run; adapters never hard-code an agent as planner,
implementer, reviewer, or verifier.

The MVP implements only:

- `single`: one bounded writer followed by deterministic verification.
- `review`: one bounded writer, one independent read-only reviewer, and deterministic
  verification.

Parallel candidate generation, debate, dynamic graphs, planner swarms, and guarded multi-stage
workflows remain deferred. They require a real task showing that `single` or `review` cannot
meet quality, latency, or safety needs within budget.

## Execution and recovery

1. A client submits a task and subscribes to ordered run events.
2. Policy determines the minimum risk level and allowed strategy.
3. The scheduler matches strategy requirements to probed agent capabilities.
4. Workspace creates isolated writer and verifier worktrees.
5. Runtime launches bounded jobs with an environment-variable allowlist.
6. Commits, diffs, results, and logs become immutable evidence.
7. Review and deterministic verification run as required by the strategy.
8. The kernel emits an approval request referencing an evidence digest.
9. A human approves, requests rework, or cancels.

External effects use intent -> effect -> settlement records. Unknown write outcomes enter
`RECOVERY_REQUIRED` and are never replayed automatically. A new run generation rejects stale
events from processes started before recovery.

## Module boundaries

- `domain`: provider-neutral contracts and state.
- `host`: client commands, event subscriptions, sessions, and approvals.
- `orchestrator`: deterministic scheduler, state machine, budgets, and stop conditions.
- `strategy`: `single` and `review` collaboration rules.
- `adapters`: provider and transport translation.
- `runtime`: process supervision, cancellation, IPC, and sandbox enforcement.
- `workspace`: Git worktree lifecycle and commit validation.
- `verification`: user-authorized deterministic commands.
- `evidence`: immutable review and approval inputs.
- `store`: atomic snapshots and append-only events.
- `policy`: risk, permissions, budgets, and denial rules.
- `redaction`: secret and sensitive-data filtering.

No module may bypass `policy`, write another module's storage directly, or depend on a client
package.

## Architectural fitness functions

| Property | Rule | Check |
|---|---|---|
| Default topology | No daemon or external data service is required | real CLI smoke test |
| Dependency direction | Kernel packages never import client or provider packages | package-boundary test |
| Provider neutrality | Domain and strategy contracts contain no provider names | contract test |
| Strategy determinism | Same state and event input produces the same transition | state-machine tests |
| Single writer | At most one role has write access to a run workspace | workspace integration test |
| Adapter viability | Probe, cancel, failure, and structured result work with a real CLI | adapter contract suite |
| Recovery safety | Ambiguous writes cannot be replayed automatically | crash-recovery integration test |
| Client parity | CLI and desktop fixtures observe the same ordered events | Host API contract suite |
| Scaffold consistency | Generated design preserves required architecture gates | bootstrap contract test |

## Deferred capabilities

- Permanent daemon or remote orchestration service.
- External database, queue, or distributed lock.
- Dynamic in-process plugin loading.
- Multi-writer shared worktrees.
- Agent-to-agent direct communication.
- Arbitrary strategy DSL or user-supplied workflow code.
- Cloud control plane and cross-machine scheduling.
- Automatic push, merge, build, deploy, or production action.

Each deferred capability needs a measured use case, a simpler-alternative analysis, a real
vertical slice, and an ADR before implementation.
EOF

cat >"$STAGING_DIR/docs/adr/0001-core-architecture.md" <<'EOF'
# ADR 0001: Core and Client Architecture

- Status: Accepted
- Responsibility: repository maintainer with contract, dependency, and integration checks

## Decision question

How can one lightweight core coordinate heterogeneous agents while supporting terminal,
desktop, CI, and future clients without binding orchestration behavior to a provider or UI?

## Context and forces

- Lightweight distribution: the default product must remain one Go binary without a required
  daemon, database server, or desktop runtime.
- Provider neutrality: agent products expose different process, protocol, session, and sandbox
  capabilities and must remain replaceable.
- Client parity: terminal and desktop clients need the same run state, events, approvals, and
  recovery behavior.
- Safety and recovery: privileged writes, cancellation, retries, and human gates cannot depend
  on model cooperation or a connected UI.
- Evolvability: collaboration patterns must change without rewriting provider adapters.

## Decision

Use a modular Go monolith centered on a headless deterministic orchestration kernel.

Expose a versioned Host API for commands, ordered events, session lifecycle, and approvals.
CLI and TUI clients call the kernel in-process. A desktop client launches the same binary as a
managed child process and uses versioned stdio or authenticated local IPC. A permanent daemon
is not part of the default runtime.

Place provider-specific behavior behind a capability-declaring agent runtime gateway.
Collaboration strategies assign planner, implementer, reviewer, verifier, or other roles for
each run. Provider adapters normalize transport and lifecycle behavior but do not own roles,
workflow state, policy, or evidence.

Persist atomic state snapshots plus append-only events, isolate writes with Git worktrees, and
require kernel policy checks for every privileged transition.

## Alternatives considered

- CLI-only orchestration: smallest initial surface, but makes desktop support a later fork and
  couples presentation to lifecycle behavior.
- Separate desktop orchestrator: enables tailored UX, but duplicates state, policy, recovery,
  and adapter logic.
- Mandatory local daemon: supports background work and multiple clients, but adds installation,
  authentication, upgrade, and orphan-process costs before those benefits are required.
- Agent-to-agent mesh: appears flexible, but makes authority, budgets, evidence, and termination
  nondeterministic.
- Terminal-screen scraping: broad compatibility, but lacks stable events, typed results, and
  reliable session recovery.

## Consequences

### Positive

- Terminal, desktop, CI, and SDK clients share one behavior and persistence model.
- New agents integrate through capabilities and lifecycle contracts rather than workflow forks.
- Collaboration strategies can evolve independently from providers and user interfaces.
- The default runtime remains a single binary with no mandatory background service.
- Policy, evidence, and recovery remain enforceable when a client disconnects.

### Negative

- The Host API and event schema become public compatibility surfaces that require versioning.
- Desktop child-process supervision and local IPC need platform-specific integration tests.
- Provider protocol changes still require maintained adapters and contract fixtures.
- Capability negotiation and strategy validation add explicit design work before execution.

## Runtime dependency responsibility

The kernel owns state and can be inspected through local files and event logs without a
desktop client. Clients can restart and reconnect to persisted runs. Agent adapters can be
disabled or replaced independently. If local IPC is unavailable, desktop integration may
degrade to supervised stdio; terminal operation remains available.

## Architectural fitness functions

| Property | Rule | Check |
|---|---|---|
| Dependency direction | Clients depend on Host API; kernel never imports client packages | package-boundary test on every change |
| Provider neutrality | Domain and strategy contracts contain no provider-specific roles | contract test on every change |
| Client parity | CLI and desktop fixtures observe the same ordered run events | integration suite before release |
| Isolation | At most one assigned writer can modify a run workspace | workspace integration test on every change |
| Compatibility | Host API and event fixtures remain backward compatible within a major version | contract suite before merge |
| Lightweight default | CLI operation requires no daemon or desktop process | end-to-end smoke test before release |

## Reversibility

This is a two-way decision while the Host API is internal. A service mode can be added without
changing the kernel when background execution, multi-client attachment, or remote scheduling
becomes a measured requirement. Reconsider the modular monolith only if independent scaling,
release cadence, or fault isolation provides more value than the operational cost of a
distributed control plane.
EOF

cat >"$STAGING_DIR/docs/exec-plans/mvp.md" <<'EOF'
# MVP Execution Plan

## Critical path

1. M0 establishes repository contracts and safe defaults.
2. M1 proves a real implementation-review-verification vertical slice.
3. M2 adds durable state and crash recovery.
4. M3 hardens process supervision and provider adapters.
5. M4 adds Git isolation and immutable evidence.
6. M5 enforces strict policy, sandboxing, and redaction.
7. M6 validates the human control workflow with real tasks.

## Stage gates

- M1 stops if either provider cannot produce structured output, cancel, or resume reliably.
- M2 stops if an ambiguous external write can be replayed automatically.
- M5 stops if strict mode can write outside its assigned workspace or inherit unrelated secrets.
- M6 requires commit, review, verification, and human decision evidence for every completed run.
EOF

cat >"$STAGING_DIR/tasks.md" <<'EOF'
# Tasks

| ID | Status | Task | Acceptance |
|---|---|---|---|
| M0-01 | complete | Establish architecture sources | Design, ADR, execution plan, and work-state responsibilities are distinct |
| M0-02 | complete | Create safe repository scaffold | Script generates the declared layout and refuses a non-empty target |
| M0-03 | complete | Add scaffold contract test | Test verifies paths, module, branch, CI permissions, and overwrite refusal |
| M0-04 | pending | Select an open-source license | Maintainer records the chosen license and rationale before public distribution |
| M0-05 | blocked | Run local Go verification | Blocked until Go 1.26 is installed with explicit approval |
| M1-01 | pending | Prove the real dual-agent vertical slice | Real implementation, blind review, and deterministic verification produce one evidence packet |
EOF

cat >"$STAGING_DIR/SECURITY.md" <<'EOF'
# Security Policy

## Reporting

Report vulnerabilities through the repository's private security-advisory channel. Do not
publish credentials, exploit details, or affected user data in a public issue.

## Trust model

Repository content, model output, tool output, and adapter events are untrusted inputs.
Worktrees are conflict isolation, not security sandboxes. Strict mode must fail closed when
the required process or filesystem boundary is unavailable.

## Product safety invariants

- No automatic push, merge, build, deploy, or production action.
- No shell interpolation for model-controlled values.
- No unrelated credential inheritance.
- No forced deletion of dirty worktrees.
- No completion without review and verification evidence.
EOF

cat >"$STAGING_DIR/CONTRIBUTING.md" <<'EOF'
# Contributing

Keep changes small, reversible, and directly tied to a task in `tasks.md`.

1. Update the design source first when architecture changes.
2. Add a failing behavior test before implementation.
3. Run focused tests, then the full local verification suite.
4. Update the related task status in the same logical commit.
5. Do not bypass hooks, signing, policy gates, or safety checks.

Build, publication, push, and deployment remain explicit maintainer actions.
EOF

cat >"$STAGING_DIR/.github/workflows/ci.yml" <<'EOF'
name: ci

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: 1.26.x
          cache: false
      - name: Test
        run: go test ./...
      - name: Vet
        run: go vet ./...
EOF

if [[ "$MODULE_PATH" == github.com/*/* ]]; then
  OWNER="${MODULE_PATH#github.com/}"
  OWNER="${OWNER%%/*}"
  printf '* @%s\n' "$OWNER" >"$STAGING_DIR/.github/CODEOWNERS"
fi

install -m 0755 "$SOURCE_DIR/bootstrap-m0.sh" "$STAGING_DIR/scripts/bootstrap-m0.sh"
install -m 0755 "$SOURCE_DIR/test-bootstrap.sh" "$STAGING_DIR/scripts/test-bootstrap.sh"

if ! git -C "$STAGING_DIR" init -b main >/dev/null 2>&1; then
  git -C "$STAGING_DIR" init >/dev/null
  git -C "$STAGING_DIR" symbolic-ref HEAD refs/heads/main
fi

if [[ -d "$TARGET_DIR" ]]; then
  rmdir "$TARGET_DIR"
fi
mv "$STAGING_DIR" "$TARGET_DIR"
trap - EXIT

printf 'Initialized %s\n' "$TARGET_DIR"
printf 'Module: %s\n' "$MODULE_PATH"
printf 'Next: review the generated files before committing.\n'
