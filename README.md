# Agent Orchestrator

A lightweight, deterministic, and safety-first orchestrator for coding agents.

Agent Orchestrator is an experimental control plane for running bounded coding-agent tasks. It focuses on strict task contracts, isolated writer worktrees, deterministic verification, immutable evidence, and explicit human approval before any high-impact action.

## Status

This repository is an early public snapshot. It contains the single-writer vertical slice, file-backed run state, Codex adapter boundaries, verification hooks, evidence bundles, and safety policies needed to explore agent orchestration patterns.

It is not a production deployment system. Push, merge, build, release, and deploy operations remain explicit maintainer actions outside the orchestrator.

## What it does

- Defines a canonical human task contract with allowed paths, acceptance criteria, budgets, and delegation rules.
- Prepares an isolated Git worktree for a single writer agent.
- Runs a provider-neutral writer adapter and requires structured completion output.
- Persists recoverable run state and immutable evidence.
- Redacts known secret patterns from evidence artifacts.
- Requires verification and human approval before a run can complete.

## Core boundaries

- One writer operates in an isolated Git worktree.
- Reviewers consume immutable evidence and receive no repository tools by default.
- Deterministic verification is independent from model review.
- Push, merge, build, and deploy are never automatic product actions.
- Model output, repository content, tool output, and adapter events are treated as untrusted input.

## Source of truth

- Architecture: [docs/design/agent-orchestrator.md](docs/design/agent-orchestrator.md)
- Decision record: [docs/adr/0001-core-architecture.md](docs/adr/0001-core-architecture.md)
- MVP execution plan: [docs/exec-plans/mvp.md](docs/exec-plans/mvp.md)
- Work state: [tasks.md](tasks.md)
- Security policy: [SECURITY.md](SECURITY.md)

## Requirements

The Go module is `github.com/xinchenglearning/agent-orchestrator` and targets Go 1.26.

You need:

- Go 1.26 or newer.
- Git.
- Optional: Codex CLI for real writer-adapter tests.

## Local verification

Run the standard checks:

```bash
go test ./...
go vet ./...
```

Real agent tests are behind the `real_agents` build tag and should only be run in an isolated test repository with disposable credentials:

```bash
go test -tags real_agents ./tests/e2e
```

## Configuration

The Codex adapter passes through only a small allowlist of environment variables, including:

```text
CODEX_API_KEY
OPENAI_API_KEY
HTTPS_PROXY
HTTP_PROXY
NO_PROXY
SSL_CERT_FILE
```

Do not commit `.env` files, API keys, local transcripts, model logs, or generated evidence bundles.

## Maturity

This project is suitable for studying agent harness design, task contracts, evidence bundles, and safety boundaries. It is not intended to be used as-is for production code automation.

## License

MIT. See [LICENSE](LICENSE).
