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
