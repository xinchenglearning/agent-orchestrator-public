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
