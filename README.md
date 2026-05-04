# Prompt Control Protocol

## The bridge between the probabilistic and deterministic worlds

Prompt Control is a **contract enforcement engine** for probabilistic LLM outputs.
It leverages Bloom filters to define a deterministic validation protocol for structured JSON contracts.

## Issue
As LLMs are probabilistic systems, there is no user-side enforcement for reliable response formats.
Even when using "structured outputs", LLM responses still might forget keys or leave them empty. 

## Objective
PrompControl offers a contract enforcement strategy that guarantees JSON outputs conform to a declared contract shape with:

- deterministic key-level validation
- low-latency execution under load
- predictable behavior for enforcement loops
- operational safety through explicit missing-field reporting
 
## Result
Move from best effort to **contract-based workflows** and scale your LLM workflows without worrying about conformity.
PromptControl allows you to enforce deterministic communication:

- guarantees structured, repeatable outputs across LLM pipelines
- enables high-compliance chatbots workflows (forms, step-driven operations, strict schemas)
- supports reliable agent-to-agent coordination and data exchange
- any environment where consistency and strict formats are required

## Contract Based Workflows

Prompt Control is intended to be paired with structured outputs when strict contract compliance is required in your application or between agents

1. Request structured JSON from LLM/provider.
2. Validate using Prompt Control.
3. Re-prompt with missing-key list if incomplete.
4. Continue until policy-complete or policy-exhausted.

This yields fast, reliable, contract-compliant behavior at scale.

## Integration With `dev-contracts` (SCL DSL)

`PromptControl` and `dev-contracts` solve different layers of the same workflow:

- `dev-contracts`: defines contracts in SCL Markdown and parses them into structured data (and optional template generation).
- `PromptControl`: enforces that LLM structured-output JSON actually matches the expected contract key structure.

Combined high-fidelity workflow:

1. SCL contract input.
2. Parse with `dev-contracts/scl` into structured contract data.
3. Use that contract shape as the reference for LLM structured outputs.
4. Validate each returned JSON with `PromptControl` Bloom-filter + exact-map enforcement.
5. If incomplete, re-prompt using missing-key feedback until complete.
6. Optionally render final contract text from the returned JSON using the generated template.

Usage modes:

- `dev-contracts` alone: contract-based workflow without caller-side key enforcement.
- `PromptControl` alone: enforcement for any contract shape you already have.
- Together (recommended): highly compliant, highly structured contract workflows with deterministic caller-side enforcement.

## Protocol Model

Prompt Control executes a stateless enforcement workflow:

1. Canonicalize the reference contract into nested key paths.
2. Build a key-membership index:
   - Bloom filter (fast probabilistic gate)
   - exact key map (authoritative verifier)
3. Canonicalize returned LLM JSON into nested key paths.
4. Enforce contract membership:
   - `completed` when all required keys exist
   - `missing_fields` with deterministic sorted missing keys otherwise

## Security / Reliability Posture

- **Stateless core**: no hidden runtime state; safe for concurrent integration.
- **Deterministic output**: stable missing-key ordering for reproducible behavior.
- **Authoritative verification**: exact key map prevents Bloom false-positive drift.
- **Protocol-first semantics**: explicit result states support strict orchestration.

## Performance Characteristics

- Bloom filter membership checks provide low-overhead pre-validation.
- Memory footprint remains efficient even for large contract key sets.
- Suitable for large JSON contracts and high-frequency validation loops.

## Artifacts

- `prompt_control.go`: Go protocol fixture
- `prompt_control.py`: Python protocol fixture
