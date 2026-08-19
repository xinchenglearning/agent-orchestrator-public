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
