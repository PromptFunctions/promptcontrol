# PromptControl

PromptControl is a contract-based workflow library for structured LLM output.

At a high level, it turns Markdown into contracts and repeatable, software-enforced workflows. The idea is simple: define the job as a contract, ask the model to fill that contract, then validate the result before the system accepts it.

The goal is not to trust the model. The goal is to make agent work more predictable, more repeatable, and easier to keep inside a defined scope.

Under that model, Markdown stops being just documentation. With SCML, a `.md` file becomes a programmable semantic resource: still readable by humans and LLMs, but also executable as a contract boundary for agent software. Imports and dependencies make those resources composable, which gives agents a lightweight way to coordinate semantic workoad without giving up deterministic checks. In that sense, PromptControl is aimed at a more semantic-native style of application design, where LLM reasoning and software enforcement live in the same workflow.

Most applications should use `forge`. It loads an SCML contract, builds the JSON scaffold, runs the LLM loop, validates the result, and returns the final JSON.

## Example Contract

Here is a small mock SCML contract. The XML-like structure lives inside Markdown comments, so the file stays readable as normal Markdown while still being parseable by the engine.

```md
# Change Plan

<!-- <scml> -->
<!-- <constants> -->
<pre>
TARGET = "src/service.go"
</pre>
<!-- </constants> -->

## ISSUE
<!-- <section name="issue"> -->
  - Describe the problem briefly.
  - State the expected behavior.
<!-- </section> -->

## SOLUTION
<!-- <section name="solution"> -->
  - Explain the intended fix.
<!-- </section> -->

## EXECUTION
<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - ${TARGET}
<!-- </section> -->
<!-- <section name="steps"> -->
  - Update the handler logic.
  - Add a focused regression test.
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
```

## Packages

- [`dev-contracts/forge`](./dev-contracts/forge/README.md)
  Default entrypoint for applications.
- [`dev-contracts/contracts`](./dev-contracts/contracts/README.md)
  SCML parsing, rendering, schema, validation metadata, and policy views.
- [`dev-contracts/gating`](./dev-contracts/gating/README.md)
  Deterministic JSON validation used by `forge`.

## Typical Flow

1. Write a contract in SCML.
2. Let `contracts` turn that file into a structured reference.
3. Let `gating` define what a valid result must look like.
4. Call `forge.RunFile(...)` to run the full loop and return final JSON.

## Start Here

- If you want to use Prompt Control in an app, read [`dev-contracts/forge/README.md`](./dev-contracts/forge/README.md).
- If you want to author or parse SCML directly, read [`dev-contracts/contracts/README.md`](./dev-contracts/contracts/README.md).
- If you need low-level validation only, read [`dev-contracts/gating/README.md`](./dev-contracts/gating/README.md).
