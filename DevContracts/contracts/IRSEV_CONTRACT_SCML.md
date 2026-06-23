# IRSEV Framework

A minimal, structured prompt framework for guiding code changes with precision.
Should be iterative-friendly in mind and promote efficient back-and-forth.

<!-- <contract> -->
<!-- <constants> -->
<pre>
SCOPE_CORE = "changes limited to explicitly listed files and functions"
GUARDRAIL = "no full file rewrites or refactors"
</pre>
<!-- </constants> -->

<!-- <section name="issue"> -->
## ISSUE
  - Describe the objective, change request, or observed problem.
  - Use concrete examples when applicable (before → after).
  - No speculation, only defined intent or observed behavior.
  - minimal reproduction steps or expected usage scenario
  - affected components / modules
  - ${SCOPE_CORE}
  - ${GUARDRAIL}
<!-- </section> -->

<!-- <section name="root-cause"> -->
## ROOT CAUSE
  - Identify the specific mechanism causing the issue.
  - Point to the exact function / logic responsible.
  - Explain *why* the current behavior happens.
  - file paths and line references
  - conditions under which the issue manifests
<!-- </section> -->

<!-- <section name="solution"> -->
## SOLUTION
  - Define the intended behavior clearly.
  - State the rule or invariant to enforce.
  - Do not describe implementation yet.
  - non-goals / explicitly unchanged behavior
  - expected input → output mapping examples
<!-- </section> -->

<!-- <section name="execution"> -->
## EXECUTION
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
> #### FILE SCOPE
  - packages/cli/*
  - packages/server/server.go
  - packages/telemetry/stats.go
<!-- </section> -->
<!-- <section name="steps"> -->
> #### STEPS
  - Provide precise code-level actions.
  - Target exact lines / blocks to modify.
  - What to keep
  - What to remove
  - What to replace with
  - Avoid broad rewrites.
  - step-by-step execution sequence
  - exact symbols / identifiers to match
<!-- </section> -->
<!-- <section name="failure-modes"> -->
> #### FAILURE MODES
  - Define explicit failure modes for each step.
  - Identify what can break during execution (logic, parsing, state, side-effects).
  - Specify how failures are detected (errors, logs, invalid states).
  - Define expected system behavior on failure (retry, abort, fallback).
  - Ensure no silent failures (must surface errors deterministically).
  - Validate post-conditions after each step.
  - List invariants that must remain true during execution.
<!-- </section> -->
<!-- </section> -->

<!-- <section name="validation"> -->
## VALIDATION
  - List concrete checks:
  - visual outputs
  - edge cases
  - regressions to avoid
  - Include exact expected patterns.
  - expected logs / return values
  - idempotency checks
  - failure scenarios and expected behavior
  - ${SCOPE_CORE}
  - ${GUARDRAIL}
<!-- </section> -->
<!-- </contract> -->
