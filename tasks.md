# Tasks

| ID | Status | Task | Acceptance |
|---|---|---|---|
| M0-01 | complete | Establish architecture sources | Design, ADR, execution plan, and work-state responsibilities are distinct |
| M0-02 | complete | Create safe repository scaffold | Script generates the declared layout and refuses a non-empty target |
| M0-03 | complete | Add scaffold contract test | Test verifies paths, module, branch, CI permissions, and overwrite refusal |
| M0-04 | complete | Select an open-source license | MIT license added for the public snapshot |
| M0-05 | complete | Run local Go verification | Go 1.26.6 and the M0 scaffold contract pass locally |
| M1-01 | complete | Define the human task contract | Repository root, base ref, objective, allowed paths, acceptance, delegation, budget, and canonical digest validation pass |
| M1-02 | complete | Prove the `single` task-completion loop | A real agent completes one trusted-local repository task; strict mode fails closed without native sandbox support |
| M1-03 | pending | Add independent `review` | A read-only reviewer receives only immutable evidence and cannot access the writer workspace |
| M1-04 | pending | Run the feasibility benchmark | Four pinned tasks report completion, cost, evidence, and policy violations without claiming production value |
| M1-05 | pending | Validate human-accepted outcomes | Ten internal tasks record blind acceptance, rejection, rework, correction time, cost, and safety invariants |
