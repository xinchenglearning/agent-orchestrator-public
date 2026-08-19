# Human Task Completion MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build one local CLI that completes a bounded repository task through a real coding agent, objective verification, immutable evidence, and explicit human acceptance.

**Architecture:** Keep a deterministic Go control plane around provider-neutral task, run,
session, event, and evidence contracts. Implement a clearly labeled `trusted-local` single-agent
loop first, then require a proven native sandbox capability for strict mode and add the
independent read-only `review` strategy without changing kernel contracts.

**Tech Stack:** Go 1.26 standard library, Git CLI, JSON/JSONL files, supervised local processes, SHA-256 evidence digests.

---

## Product Acceptance

The MVP is complete only when a human can:

1. Submit a repository task with a canonical repository root, pinned base ref, objective,
   deliverables, allowed paths, constraints, acceptance commands, delegation level, and budget.
2. Approve the digest of the task and acceptance commands before execution.
3. Let one real agent modify an isolated worktree in explicitly labeled `trusted-local` mode.
4. Use strict mode only when the adapter proves native sandbox support; otherwise fail closed.
5. Receive objective verification results and immutable evidence, review, and decision digests.
6. Approve, request one bounded rework, reject the result, or cancel execution.
7. Observe that only an explicit approval of the decision packet transitions the run to
   `COMPLETED`.

The first supported task domain is repository work: bug fixes, small features, tests, and
documentation. Research, operations, desktop UI, service mode, remote scheduling, dynamic
plugins, and arbitrary workflow graphs remain outside this plan.

## Lightweight Budget

- One `orch` binary.
- No daemon, database, queue, container, browser runtime, or desktop runtime.
- Standard library only until a direct dependency has a measured replacement cost.
- One writer worktree and one verifier worktree per run.
- At most one rework attempt in the first vertical slice.
- No automatic push, merge, build, deploy, or production action.
- Verification commands are argv arrays; model-controlled text is never interpolated into a
  shell command.
- `trusted-local` mode treats the repository as trusted and makes no filesystem-containment
  claim.
- Strict mode requires a probed native sandbox capability and refuses execution when absent.

## Delivery Sequence

| Milestone | Human-visible outcome | Exit gate |
|---|---|---|
| M1A | `single` strategy produces a verified evidence packet | Real agent and real Git task reach `AWAITING_APPROVAL` |
| M1B | Human can approve, reject, cancel, or request bounded rework | Only approval reaches `COMPLETED` |
| M1C | `review` adds an independent read-only reviewer | Reviewer receives only the evidence packet |
| M1D | Feasibility benchmark compares solo and review modes | Four pinned tasks report acceptance, correction time, cost, and violations |
| M1E | Human dogfood validates usable outcomes | Ten internal tasks record blind acceptance, rejection, rework, and correction time |

## Planned File Map

```text
cmd/orch/main.go
internal/domain/
  task.go
  task_test.go
  run.go
  run_test.go
internal/host/
  service.go
  service_test.go
internal/orchestrator/
  engine.go
  engine_test.go
internal/strategy/
  single.go
  review.go
internal/adapters/
  adapter.go
  result.go
  result_test.go
  codex/adapter.go
  codex/adapter_real_test.go
  claude/adapter.go
  claude/adapter_real_test.go
internal/runtime/process/
  runner.go
  runner_test.go
internal/policy/
  policy.go
  policy_test.go
internal/workspace/
  git.go
  git_integration_test.go
internal/verification/
  verifier.go
  verifier_integration_test.go
internal/evidence/
  bundle.go
  bundle_test.go
  decision.go
  decision_test.go
internal/redaction/
  redactor.go
  redactor_test.go
internal/store/
  file.go
  file_test.go
  lock.go
  lock_test.go
tests/e2e/
  single_real_test.go
  review_real_test.go
  testdata/writer-fixture/
  testdata/hidden-tests/
tests/contract/
  adapter_suite.go
benchmarks/
  tasks/
  report.go
  dogfood.go
tasks.md
```

### Task 1: Toolchain And Baseline Gate

**Files:**
- Modify: `tasks.md`

- [ ] **Step 1: Verify the required local tools**

Run:

```bash
go version
git --version
codex --version
claude --version
```

Expected:

- Go reports `go1.26.x`.
- Git, Codex, and Claude commands exit with status `0`.
- If Go is absent, stop and request explicit installation approval.
- If either agent is absent, mark its adapter task blocked; do not replace it with a mock E2E.

- [ ] **Step 2: Run the existing scaffold contract**

Run:

```bash
./scripts/test-bootstrap.sh
```

Expected: `PASS: M0 bootstrap contract`.

- [ ] **Step 3: Record the gate in the task board**

Change `M0-05` to `complete` only after `go version` succeeds. Keep unavailable real adapters
explicitly blocked.

- [ ] **Step 4: Commit the baseline state when it changed**

```bash
git add tasks.md
git commit -m "chore: record MVP toolchain readiness"
```

Do not create an empty commit when no readiness status changed.

### Task 2: Human-Centered Task Contract

**Files:**
- Create: `internal/domain/task.go`
- Create: `internal/domain/task_test.go`

- [ ] **Step 1: Write failing validation tests**

```go
func TestValidateTaskRequiresHumanOutcome(t *testing.T) {
    task := domain.Task{ID: "task-1"}
    err := domain.ValidateTask(task)
    if err == nil {
        t.Fatal("expected invalid task")
    }
}

func TestValidateTaskAcceptsReversibleRepositoryWork(t *testing.T) {
    task := domain.Task{
        ID:           "task-1",
        Repository: domain.RepositorySpec{
            Root:    "/tmp/fixture-repo",
            BaseRef: "0123456789abcdef0123456789abcdef01234567",
        },
        Objective:    "Fix the failing parser test",
        Rationale:    "Restore valid configuration loading",
        Deliverables: []string{"one Git commit", "passing tests"},
        AllowedPaths: []string{"parser/", "parser_test.go"},
        Constraints:  []string{"do not modify public APIs"},
        Acceptance: []domain.CommandSpec{{
            Argv:           []string{"go", "test", "./..."},
            TimeoutSeconds: 120,
        }},
        Delegation: domain.DelegationExecuteReversible,
        Budget: domain.Budget{MaxAttempts: 1, MaxSeconds: 900},
    }
    if err := domain.ValidateTask(task); err != nil {
        t.Fatal(err)
    }
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/domain -run TestValidateTask -v
```

Expected: FAIL because `Task` and `ValidateTask` do not exist.

- [ ] **Step 3: Implement the minimal contract**

```go
type DelegationLevel string

const (
    DelegationSuggest           DelegationLevel = "suggest"
    DelegationPrepare           DelegationLevel = "prepare"
    DelegationExecuteReversible DelegationLevel = "execute_reversible"
    DelegationApprovalRequired  DelegationLevel = "approval_required"
)

type CommandSpec struct {
    Argv           []string `json:"argv"`
    TimeoutSeconds int      `json:"timeoutSeconds"`
}

type Budget struct {
    MaxAttempts int `json:"maxAttempts"`
    MaxSeconds  int `json:"maxSeconds"`
}

type RepositorySpec struct {
    Root    string `json:"root"`
    BaseRef string `json:"baseRef"`
}

type Task struct {
    ID           string          `json:"id"`
    Repository   RepositorySpec  `json:"repository"`
    Objective    string          `json:"objective"`
    Rationale    string          `json:"rationale"`
    Deliverables []string        `json:"deliverables"`
    NonGoals     []string        `json:"nonGoals"`
    AllowedPaths []string        `json:"allowedPaths"`
    Constraints  []string        `json:"constraints"`
    Acceptance   []CommandSpec   `json:"acceptance"`
    Delegation   DelegationLevel `json:"delegation"`
    Budget       Budget          `json:"budget"`
}
```

`ValidateTask` must reject IDs outside `[a-zA-Z0-9._-]+`, non-canonical repository roots,
non-commit base refs, empty objectives, rationale, deliverables, allowed paths, acceptance argv,
unknown delegation levels, attempts outside `1..2`, and non-positive time budgets. M1 supports
only `execute_reversible`; the other declared delegation levels return
`ErrUnsupportedDelegation` until their behavior has dedicated tests.

Compute the task digest from canonical JSON. The digest covers repository root, base ref,
allowed paths, acceptance argv, constraints, delegation, and budget so an agent cannot alter
them after human approval.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
go test ./internal/domain -run TestValidateTask -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/task.go internal/domain/task_test.go
git commit -m "feat(domain): add human task contract"
```

### Task 3: Deterministic Run State And Human Decision

**Files:**
- Create: `internal/domain/run.go`
- Create: `internal/domain/run_test.go`

- [ ] **Step 1: Write failing transition tests**

Cover these exact rules:

```text
CREATED -> PREPARING -> IMPLEMENTING -> VERIFYING -> AWAITING_APPROVAL
AWAITING_APPROVAL + approve -> COMPLETED
AWAITING_APPROVAL + request_rework -> REWORKING
AWAITING_APPROVAL + reject -> REJECTED
REWORKING -> IMPLEMENTING
any non-terminal state + cancel -> CANCELED
unknown external write -> RECOVERY_REQUIRED
```

Add a test proving `VERIFYING` cannot transition directly to `COMPLETED`.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/domain -run TestTransition -v
```

Expected: FAIL because the run state machine does not exist.

- [ ] **Step 3: Implement state and event types**

```go
type RunState string

const (
    RunCreated          RunState = "CREATED"
    RunPreparing        RunState = "PREPARING"
    RunImplementing     RunState = "IMPLEMENTING"
    RunVerifying        RunState = "VERIFYING"
    RunAwaitingApproval RunState = "AWAITING_APPROVAL"
    RunReworking        RunState = "REWORKING"
    RunCompleted        RunState = "COMPLETED"
    RunRejected         RunState = "REJECTED"
    RunFailed           RunState = "FAILED"
    RunCanceled         RunState = "CANCELED"
    RunRecoveryRequired RunState = "RECOVERY_REQUIRED"
)

type HumanDecision string

const (
    DecisionApprove       HumanDecision = "approve"
    DecisionRequestRework HumanDecision = "request_rework"
    DecisionReject        HumanDecision = "reject"
    DecisionCancel        HumanDecision = "cancel"
)

type Approval struct {
    Actor                string        `json:"actor"`
    Decision             HumanDecision `json:"decision"`
    TaskDigest           string        `json:"taskDigest"`
    DecisionPacketDigest string        `json:"decisionPacketDigest"`
    At                   time.Time     `json:"at"`
}

type EventType string

const (
    EventPrepared        EventType = "prepared"
    EventImplementation EventType = "implementation_finished"
    EventVerification   EventType = "verification_finished"
    EventApproved       EventType = "approved"
    EventReworkRequested EventType = "rework_requested"
    EventRejected       EventType = "rejected"
    EventCanceled       EventType = "canceled"
    EventWriteAmbiguous EventType = "write_ambiguous"
)
```

Implement `Transition(current RunState, event EventType) (RunState, error)` as a table-driven
pure function. Do not let adapters mutate run state. An approval event is valid only when its
task and decision-packet digests match the persisted run; a stale or mismatched approval must
leave the state unchanged.

- [ ] **Step 4: Verify GREEN**

```bash
go test ./internal/domain -run TestTransition -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/run.go internal/domain/run_test.go
git commit -m "feat(domain): add human-gated run state machine"
```

### Task 4: Durable State And Effect Records

**Files:**
- Create: `internal/store/file.go`
- Create: `internal/store/file_test.go`
- Create: `internal/store/lock.go`
- Create: `internal/store/lock_test.go`

- [ ] **Step 1: Write failing persistence tests**

Tests must prove:

- `state.json` is replaced atomically.
- `events.jsonl` preserves append order.
- An effect intent without settlement is reported as ambiguous.
- Events from an older run generation are rejected.
- Two mutating CLI processes cannot acquire the same run lock.
- A lock left by a crashed process is never removed automatically.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/store -v
```

Expected: FAIL because the file store does not exist.

- [ ] **Step 3: Implement the file layout**

Resolve the repository storage root with `git rev-parse --git-common-dir` so invocation from a
linked worktree does not create a second state tree.

```text
<git-common-dir>/agent-orchestrator/runs/<run-id>/
  state.json
  events.jsonl
  evidence/
  sessions.json
  lock/
```

Use `os.CreateTemp`, `File.Sync`, `File.Close`, and `os.Rename` for state replacement. Append one
JSON event plus newline per write. Persist effect records in this order:

```text
effect_intent -> external operation -> effect_settlement
```

- [ ] **Step 4: Implement the cross-process run lock**

Acquire a lock with atomic `os.Mkdir(<run-dir>/lock, 0700)` before every mutating operation.
Write owner PID, command, generation, and acquisition time inside the directory. Hold the lock
through state validation and settlement, then remove it with `defer`.

`orch run`, `orch decide`, `orch cancel`, and recovery commands require the lock. `orch status`
reads the atomically replaced snapshot without mutating it. A crash-left lock causes
`RECOVERY_REQUIRED`; M1 requires an explicit human recovery command and never performs automatic
stale-lock deletion.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/store -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/file.go internal/store/file_test.go \
  internal/store/lock.go internal/store/lock_test.go
git commit -m "feat(store): persist recoverable run state"
```

### Task 5: Process Runner And Policy Boundary

**Files:**
- Create: `internal/runtime/process/runner.go`
- Create: `internal/runtime/process/runner_test.go`
- Create: `internal/policy/policy.go`
- Create: `internal/policy/policy_test.go`

- [ ] **Step 1: Write failing process and policy tests**

Tests must prove:

- Commands execute with `exec.CommandContext(name, args...)`, never `sh -c`.
- Only explicitly allowed environment keys are inherited.
- Cancellation terminates the child process group.
- `push`, `merge`, `deploy`, and external production actions are denied.
- Write capability is granted to at most one role.
- Strict mode rejects a writer adapter when native sandbox support is absent.
- Trusted-local mode is labeled in every event and evidence packet and makes no containment
  claim.
- Verification rejects argv whose digest differs from the human-approved task digest.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/runtime/process ./internal/policy -v
```

Expected: FAIL because the runner and policy do not exist.

- [ ] **Step 3: Implement the minimal APIs**

```go
type Spec struct {
    Path    string
    Args    []string
    Dir     string
    Env     map[string]string
    Timeout time.Duration
}

type Policy struct {
    AllowedEnv      map[string]struct{}
    AllowWriteRole  string
    DeniedActions   map[string]struct{}
}

type ExecutionMode string

const (
    ExecutionTrustedLocal ExecutionMode = "trusted-local"
    ExecutionStrict       ExecutionMode = "strict"
)
```

Build `cmd.Env` from the allowlist rather than inheriting `os.Environ`. Return separate stdout,
stderr, exit code, start time, and end time fields. Each adapter declares the minimum
authentication environment keys it needs; policy intersects that request with the global
allowlist.

Strict mode delegates filesystem containment to a probed native agent sandbox and fails closed
when `NativeSandbox` is false. M1 does not implement a custom sandbox. Trusted-local mode may be
used only for developer-owned repositories and must remain visible in status, events, evidence,
and benchmark reports.

- [ ] **Step 4: Verify GREEN**

```bash
go test ./internal/runtime/process ./internal/policy -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runtime/process internal/policy
git commit -m "feat(runtime): enforce process and policy boundaries"
```

### Task 6: Git Worktrees, Verification, And Evidence

**Files:**
- Create: `internal/workspace/git.go`
- Create: `internal/workspace/git_integration_test.go`
- Create: `internal/verification/verifier.go`
- Create: `internal/verification/verifier_integration_test.go`
- Create: `internal/evidence/bundle.go`
- Create: `internal/evidence/bundle_test.go`
- Create: `internal/evidence/decision.go`
- Create: `internal/evidence/decision_test.go`
- Create: `internal/redaction/redactor.go`
- Create: `internal/redaction/redactor_test.go`

- [ ] **Step 1: Write failing real-Git integration tests**

Create a temporary repository with an initial commit. Prove that:

- Writer and verifier use separate worktrees.
- A dirty worktree is never deleted automatically.
- Verification executes only the task's argv commands.
- Result diffs touching paths outside `Task.AllowedPaths` are rejected before verification.
- The evidence bundle contains base commit, result commit, artifact digests, command results,
  execution mode, and task digest.
- Review results reference the immutable evidence digest rather than a mutable path.
- Decision packets reference task, evidence, review, and verification digests.
- Changing any referenced artifact changes its containing digest.
- API keys, bearer tokens, credential-like environment values, and configured literals are
  redacted before any stdout, stderr, model output, or event is persisted.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/workspace ./internal/verification ./internal/evidence -v
```

Expected: FAIL because these services do not exist.

- [ ] **Step 3: Implement explicit Git argv operations**

Resolve the canonical repository root and common Git directory before creating any worktree.
Use the process runner for:

```text
git rev-parse --show-toplevel
git rev-parse --git-common-dir
git worktree add --detach <path> <base-sha>
git status --porcelain
git diff --binary <base-sha>..<result-sha>
git rev-parse HEAD
```

Never use shell command concatenation. Refuse cleanup when `git status --porcelain` is non-empty.
Parse changed paths from Git's structured name-status output and reject any path outside the
human-approved allowlist.

- [ ] **Step 4: Implement redaction before persistence**

The redactor accepts configured exact secrets and detects common token, authorization header,
and private-key forms. Redact streams before they reach the store, evidence writer, or event
log. Tests must prove the original secret bytes do not appear anywhere under the run directory.

- [ ] **Step 5: Implement the three-stage evidence chain**

```go
type Artifact struct {
    Name   string `json:"name"`
    SHA256 string `json:"sha256"`
    Size   int64  `json:"size"`
}

type EvidenceBundle struct {
    RunID          string          `json:"runId"`
    TaskDigest     string          `json:"taskDigest"`
    ExecutionMode  string          `json:"executionMode"`
    BaseCommit     string          `json:"baseCommit"`
    ResultCommit   string          `json:"resultCommit"`
    Artifacts      []Artifact      `json:"artifacts"`
    Verification   []CommandResult `json:"verification"`
}

type ReviewResult struct {
    EvidenceDigest string    `json:"evidenceDigest"`
    Findings       []Finding `json:"findings"`
}

type Finding struct {
    Severity       string `json:"severity"`
    File           string `json:"file"`
    Line           int    `json:"line"`
    Evidence       string `json:"evidence"`
    Recommendation string `json:"recommendation"`
}

type DecisionPacket struct {
    TaskDigest         string `json:"taskDigest"`
    EvidenceDigest     string `json:"evidenceDigest"`
    ReviewDigest       string `json:"reviewDigest,omitempty"`
    VerificationDigest string `json:"verificationDigest"`
}

type CommandResult struct {
    Argv       []string  `json:"argv"`
    ExitCode   int       `json:"exitCode"`
    StdoutPath string    `json:"stdoutPath"`
    StderrPath string    `json:"stderrPath"`
    StartedAt  time.Time `json:"startedAt"`
    FinishedAt time.Time `json:"finishedAt"`
}
```

Canonicalize structs before hashing and store each digest beside, not inside, the hashed value.
The reviewer receives `EvidenceBundle` and its digest. `ReviewResult` references that digest.
Human approval references the final `DecisionPacket` digest, preventing circular, self-hashed,
or path-only trust.

- [ ] **Step 6: Verify GREEN**

```bash
go test ./internal/workspace ./internal/verification ./internal/evidence \
  ./internal/redaction -v
```

Expected: PASS using real Git commands.

- [ ] **Step 7: Commit**

```bash
git add internal/workspace internal/verification internal/evidence internal/redaction
git commit -m "feat: isolate work and produce immutable evidence"
```

### Task 7: Provider-Neutral Adapter Contract

**Files:**
- Create: `internal/adapters/adapter.go`
- Create: `internal/adapters/result.go`
- Create: `internal/adapters/result_test.go`
- Create: `tests/contract/adapter_suite.go`

- [ ] **Step 1: Write failing structured-result tests**

Cover raw JSON, fenced JSON, invalid schema, one format-repair retry, and repeated invalid output
ending in `protocol_error`.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/adapters ./tests/contract -v
```

Expected: FAIL because adapter contracts and parsing do not exist.

- [ ] **Step 3: Implement the adapter interface**

```go
type Capabilities struct {
    StructuredOutput bool
    Cancellation     bool
    SessionResume    bool
    ReadOnly         bool
    WorkspaceWrite   bool
    NativeSandbox    bool
    Streaming        bool
}

type Adapter interface {
    Probe(context.Context) (Capabilities, error)
    Start(context.Context, Job) (Session, error)
}

type Session interface {
    Events() <-chan Event
    Wait(context.Context) (Result, error)
    Cancel(context.Context) error
}

type Event struct {
    Type       string
    Message    string
    Data       json.RawMessage
    At         time.Time
    Generation uint64
}

type Result struct {
    SessionID       string
    ResultCommit    string
    StructuredValue json.RawMessage
    Usage           Usage
}

type Usage struct {
    InputTokens  int64
    OutputTokens int64
    WallTime     time.Duration
}

type Job struct {
    Task         domain.Task
    TaskDigest   string
    Role         string
    ExecutionMode policy.ExecutionMode
    WorkspaceDir string
    EvidenceDir  string
    Budget       domain.Budget
    Generation   uint64
}
```

`Session.Wait` is the only source of the normalized terminal result. Terminal events may report
progress but cannot substitute for `Result`. `Job` contains no provider-specific prompt flags.
Session resume remains an explicit post-MVP capability; adapters report it false until a real
contract test proves it.

- [ ] **Step 4: Implement extraction and validation**

Extract the first valid JSON object from raw text or a fenced block, decode with
`json.Decoder.DisallowUnknownFields`, validate required fields, and allow one adapter-level
format-repair attempt. Never execute data from the result.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/adapters ./tests/contract -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/adapters/adapter.go internal/adapters/result.go \
  internal/adapters/result_test.go tests/contract/adapter_suite.go
git commit -m "feat(adapters): define provider-neutral contract"
```

### Task 8: Real Writer Adapter

**Files:**
- Create: `internal/adapters/codex/adapter.go`
- Create: `internal/adapters/codex/adapter_real_test.go`

- [ ] **Step 1: Write the contract test against the installed CLI**

Start the file with `//go:build real_agents` so ordinary `go test ./...` never invokes an
external model. The test must:

- Skip only when `codex` is not installed and report the missing prerequisite.
- Probe real capabilities.
- Run against a disposable Git worktree.
- Produce a real commit for a deterministic fixture task.
- Cancel a long-running request.
- Return normalized events plus a schema-valid terminal `Result`.
- Prove strict execution is rejected when `NativeSandbox` is false.
- When `NativeSandbox` is true, attempt a canary write outside the assigned workspace and prove
  the native sandbox denies it before advertising strict-mode support.

- [ ] **Step 2: Verify RED**

```bash
go test -tags=real_agents ./internal/adapters/codex -run Contract -v -count=1
```

Expected: FAIL because the real adapter does not exist.

- [ ] **Step 3: Implement the smallest supported invocation**

Use the process runner and the installed CLI's documented non-interactive, structured-output
mode. Map provider events and terminal results to the common model. Record the exact native
sandbox capability and minimal authentication environment keys. If structured output,
cancellation, terminal result, or strict sandbox behavior cannot be proven, stop the
corresponding mode and record the failed feasibility gate instead of parsing terminal screen
text.

- [ ] **Step 4: Verify GREEN**

```bash
go test -tags=real_agents ./internal/adapters/codex -run Contract -v -count=1
```

Expected: PASS against the real CLI.

- [ ] **Step 5: Commit**

```bash
git add internal/adapters/codex
git commit -m "feat(codex): add real writer adapter"
```

### Task 9: Single Strategy, Host Service, And CLI

**Files:**
- Create: `internal/strategy/single.go`
- Create: `internal/orchestrator/engine.go`
- Create: `internal/orchestrator/engine_test.go`
- Create: `internal/host/service.go`
- Create: `internal/host/service_test.go`
- Create: `cmd/orch/main.go`

- [ ] **Step 1: Write a failing in-process completion test**

Use a deterministic test adapter and real temporary Git repository. Assert:

- The task passes validation.
- The supplied task digest matches the canonical task before any process starts.
- The selected execution mode passes policy and capability checks.
- Exactly one writer starts.
- Verification runs after the result commit.
- The run stops at `AWAITING_APPROVAL`.
- `approve` moves to `COMPLETED`.
- `request_rework` starts at most one additional attempt.
- Concurrent `run`, `decide`, or `cancel` mutations fail while the run lock is held.

- [ ] **Step 2: Verify RED**

```bash
go test ./internal/orchestrator ./internal/host -v
```

Expected: FAIL because the engine and host service do not exist.

- [ ] **Step 3: Implement the deterministic engine**

The engine owns run locking, task-digest checks, transitions, policy checks, effect records,
generation fencing, budgets, and stop conditions. The strategy only declares jobs and
dependencies; it cannot mutate state.

- [ ] **Step 4: Implement CLI commands**

```text
orch prepare --task <task.json>
orch run --task <task.json> --approved-task-digest <sha256> \
  --strategy single --writer codex --mode trusted-local
orch status <run-id> --json
orch decide <run-id> approve --decision-packet-digest <sha256>
orch decide <run-id> request-rework
orch decide <run-id> reject --decision-packet-digest <sha256>
orch cancel <run-id>
orch recover <run-id> acknowledge-stale-lock
```

`prepare` validates the task, resolves the repository and base commit, and prints the canonical
task digest without starting an agent. `run` refuses a mismatched digest, prints the run ID, and
streams normalized events. `status --json` returns only persisted state and digest references.
`recover` requires explicit human confirmation and never automatically removes a lock.

- [ ] **Step 5: Verify GREEN**

```bash
go test ./internal/orchestrator ./internal/host ./cmd/orch -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/strategy internal/orchestrator internal/host cmd/orch
git commit -m "feat: complete the single-agent task loop"
```

### Task 10: Real Single-Strategy End-To-End

**Files:**
- Create: `tests/e2e/single_real_test.go`
- Create: `tests/e2e/testdata/writer-fixture/README.md`
- Create: `tests/e2e/testdata/hidden-tests/parser_acceptance_test.go`
- Modify: `tasks.md`

- [ ] **Step 1: Create a deterministic fixture repository**

The writer fixture starts with a failing parser behavior and contains no hidden acceptance
tests. Keep hidden tests outside the writer worktree. After the writer commits, the harness
copies hidden tests only into the verifier worktree. The task contract asks the agent to fix the
behavior without changing the public API.

- [ ] **Step 2: Write the real E2E test**

Start the file with `//go:build real_agents`. The test invokes the built CLI, runs the real
writer adapter, verifies Git isolation, injects and executes the hidden test only in the
verifier worktree, inspects the evidence and decision digests, submits a synthetic approval
fixture, and confirms the final state is `COMPLETED`.

The synthetic approval proves state-machine wiring only. It does not count as evidence that a
human accepted the result.

- [ ] **Step 3: Run the real vertical slice**

Run only after explicit build approval:

```bash
go test -tags=real_agents ./tests/e2e -run TestSingleRealTask -v -count=1
```

Expected: PASS with one result commit, complete evidence, and no remote Git operations.

- [ ] **Step 4: Record lightweight baselines**

Capture binary size, cold startup, peak RSS, child-process count, run-directory bytes, and
control-plane time. Store the measurements in the E2E artifact; do not create thresholds until
the first baseline exists.

- [ ] **Step 5: Update task state and commit**

Mark the single-strategy vertical slice complete only after the real E2E passes.

```bash
git add tests/e2e tasks.md
git commit -m "test: prove real human task completion"
```

### Task 11: Independent Review Strategy

**Files:**
- Create: `internal/strategy/review.go`
- Create: `internal/adapters/claude/adapter.go`
- Create: `internal/adapters/claude/adapter_real_test.go`
- Create: `tests/e2e/review_real_test.go`

- [ ] **Step 1: Write the reviewer isolation test**

Assert that the reviewer job contains the task contract, immutable `EvidenceBundle`, its digest,
diff artifact, and verifier results but no repository path and no write capability. The returned
`ReviewResult` must reference the exact evidence digest.

- [ ] **Step 2: Verify RED**

```bash
go test -tags=real_agents ./internal/strategy ./internal/adapters/claude ./tests/e2e \
  -run 'Review|Claude' -v
```

Expected: FAIL because the review strategy and real reviewer adapter do not exist.

- [ ] **Step 3: Implement the real reviewer adapter**

Start both real test files with `//go:build real_agents`. Use the same adapter contract and one
documented structured-output invocation. Require findings with severity, file, line, evidence,
and recommendation. Reject unsupported fields and perform one format-repair retry.

- [ ] **Step 4: Implement review scheduling**

Run deterministic verification and read-only review after the writer commit. Combine them only
after both settle. A blocking verifier failure or blocking review finding prevents
`AWAITING_APPROVAL` and enters bounded rework.

- [ ] **Step 5: Run the real review E2E**

Run only after explicit build approval:

```bash
go test -tags=real_agents ./tests/e2e -run TestReviewRealTask -v -count=1
```

Expected: PASS with reviewer isolation, an evidence-linked review digest, one decision-packet
digest, and synthetic approval wiring.

- [ ] **Step 6: Commit**

```bash
git add internal/strategy/review.go internal/adapters/claude tests/e2e/review_real_test.go
git commit -m "feat: add independent review strategy"
```

### Task 12: Feasibility Benchmark

**Files:**
- Create: `benchmarks/tasks/bug-fix.json`
- Create: `benchmarks/tasks/small-feature.json`
- Create: `benchmarks/tasks/test-addition.json`
- Create: `benchmarks/tasks/docs-change.json`
- Create: `benchmarks/report.go`
- Modify: `tasks.md`

- [ ] **Step 1: Pin four real repository tasks**

Each task records base commit, human objective, hidden acceptance commands, allowed paths,
delegation level, timeout, and expected human deliverable. Agents never receive the known patch.

- [ ] **Step 2: Run baseline and orchestrated modes**

Run each task in:

```text
single with writer A
review with the same writer A and reviewer B
```

Record completion, first-pass acceptance, human correction minutes, attempts, wall time, cost,
evidence completeness, and policy violations.

- [ ] **Step 3: Produce the decision report**

The report must separate deterministic architecture invariants from stochastic task success.
Safety violations, incomplete evidence, duplicate writes, or unapproved external actions are
automatic feasibility failures. Four tasks are too few to claim human outcome improvement.

- [ ] **Step 4: Apply the feasibility gate**

Continue to dogfood only when both strategies preserve all deterministic invariants and at
least one completes a pinned task. Keep `single` as the default during dogfood; four tasks cannot
justify changing the product default.

- [ ] **Step 5: Update task state and commit**

```bash
git add benchmarks tasks.md
git commit -m "test: benchmark task feasibility"
```

### Task 13: Ten-Task Human Dogfood Gate

**Files:**
- Create: `benchmarks/dogfood.go`
- Create: `benchmarks/dogfood/README.md`
- Modify: `tasks.md`

- [ ] **Step 1: Select ten bounded internal repository tasks**

Use tasks that a maintainer would otherwise perform: bug fixes, small features, tests, and
documentation. Record the canonical task digest before execution. Never include production
credentials or require automatic external actions.

- [ ] **Step 2: Collect blind human decisions**

Before revealing the execution mode, require the maintainer to record:

```text
accept | request_rework | reject
mandatory correction minutes
reason codes
unexpected behavior
remaining risk
```

Store the decision beside the decision-packet digest. A synthetic approval fixture does not
count.

- [ ] **Step 3: Compare `single` and `review`**

Use the same writer and task constraints in both modes. Report accepted-task rate, first-pass
acceptance, correction minutes, attempts, wall time, model cost, evidence completeness, and
policy violations.

- [ ] **Step 4: Apply the product gate**

Safety violations, incomplete evidence, duplicate writes, or unapproved external actions are
automatic failures. Make `review` a default option only when it improves accepted outcomes or
reduces human correction enough to justify its measured cost. Otherwise keep it opt-in.

- [ ] **Step 5: Update task state and commit**

```bash
git add benchmarks/dogfood.go benchmarks/dogfood/README.md tasks.md
git commit -m "test: validate human-accepted task outcomes"
```

## Verification Lanes

### Pre-Commit

```bash
go test ./internal/domain ./internal/store ./internal/policy
go vet ./...
```

Expected runtime target: establish p95 after 20 CI runs, then keep the fast lane under five
minutes by splitting lanes rather than removing checks.

### Pre-Merge

```bash
go test ./internal/... ./tests/contract/... ./tests/integration/...
go vet ./...
```

Required: deterministic tests, real Git integration, adapter protocol fixtures, run-lock
contention, race-safe state transitions, redaction, and no flaky required checks. Files tagged
`real_agents` are excluded from this lane.

### Pre-Release

```bash
go test -tags=real_agents ./internal/adapters/codex ./internal/adapters/claude \
  ./tests/e2e/... -count=1
```

Required: real installed agents, disposable repositories, human-decision fixtures, and captured
evidence artifacts. External-agent E2E stays outside the fast pre-commit lane.

## Quality Policy

- Critical state transitions and policy branches require complete behavior coverage.
- Changed-code line coverage may be reported, but it cannot replace state, failure, and
  permission tests.
- A required test with more than one percent rerun disagreement is quarantined or downgraded
  within 24 hours, assigned an owner, and given a seven-day expiry.
- Test infrastructure failures are reported separately from product failures.
- Synthetic repositories are used before merge; real repository tasks are pinned and executed
  only in isolated pre-release or benchmark runs.
- Hidden acceptance tests remain outside writer worktrees and are injected only into verifier
  worktrees after the result commit.
- Real-agent tests always use the `real_agents` build tag and never run from ordinary
  `go test ./...`.

## Stop Conditions

Stop implementation and revisit the architecture when:

- A real adapter requires terminal-screen scraping.
- A real task cannot be expressed without provider-specific domain fields.
- A requested strict run lacks a proven native sandbox capability.
- An unknown external write can be replayed automatically.
- A run lock can be bypassed or automatically reclaimed without human recovery.
- Human approval can be bypassed on the path to `COMPLETED`.
- Unredacted secrets can reach events, logs, evidence, review input, or decision packets.
- The default path requires a daemon, external database, queue, or dynamic plugin runtime.
- The second agent adds cost but does not improve accepted outcomes or human correction time.

## Explicitly Deferred

- Desktop UI implementation; only the Host API contract is prepared.
- Service mode, cloud control plane, and cross-machine scheduling.
- A custom cross-platform sandbox; M1 reuses proven native agent sandboxes and otherwise fails
  strict mode closed.
- Agent-to-agent direct communication.
- Parallel candidates, debate, planner swarms, and arbitrary workflow graphs.
- Automatic push, merge, build, deploy, or production actions.
- Non-repository task domains until the repository-task completion loop is proven.
