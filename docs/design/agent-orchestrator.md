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
