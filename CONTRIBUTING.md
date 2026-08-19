# Contributing

Keep changes small, reversible, and directly tied to a task in `tasks.md`.

1. Update the design source first when architecture changes.
2. Add a failing behavior test before implementation.
3. Run focused tests, then the full local verification suite.
4. Update the related task status in the same logical commit.
5. Do not bypass hooks, signing, policy gates, or safety checks.

Build, publication, push, and deployment remain explicit maintainer actions.
