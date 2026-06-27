# Prompt Control

Prompt Control is a small contract-driven workflow for structured LLM output.

Most applications should use `forge`. It loads an SCML contract, builds the JSON scaffold, runs the LLM loop, validates the result, and returns the final JSON.

## Packages

- [`dev-contracts/forge`](./dev-contracts/forge/README.md)
  Default entrypoint for applications.
- [`dev-contracts/contracts`](./dev-contracts/contracts/README.md)
  SCML parsing, rendering, schema, validation metadata, and policy views.
- [`dev-contracts/gating`](./dev-contracts/gating/README.md)
  Deterministic JSON validation used by `forge`.

## Typical Flow

1. Write a contract in SCML.
2. Call `forge.RunFile(...)` with the contract path and your LLM client.
3. `forge` loads the contract, validates retries, and returns final JSON.

## Start Here

- If you want to use Prompt Control in an app, read [`dev-contracts/forge/README.md`](./dev-contracts/forge/README.md).
- If you want to author or parse SCML directly, read [`dev-contracts/contracts/README.md`](./dev-contracts/contracts/README.md).
- If you need low-level validation only, read [`dev-contracts/gating/README.md`](./dev-contracts/gating/README.md).
