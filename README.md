# promptcontrol

## Prompt Control Protocol

Prompt Control is a **contract enforcement engine** for probabilistic LLM outputs.
It defines a deterministic validation protocol for structured JSON contracts in
high-throughput agentic systems.

## Objective

Guarantee that JSON outputs conform to a declared contract shape with:

- deterministic key-level validation
- low-latency execution under load
- predictable behavior for enforcement loops
- operational safety through explicit missing-field reporting

## Protocol Model

Prompt Control executes a stateless enforcement cycle:

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

## Industrial Integration Pattern

Prompt Control is intended to be paired with structured outputs:

1. Request structured JSON from LLM/provider.
2. Validate using Prompt Control.
3. Re-prompt with missing-key list if incomplete.
4. Continue until policy-complete or policy-exhausted.

This yields fast, reliable, contract-compliant behavior at scale.

## Artifacts

- `prompt_control.go`: standalone Go protocol fixture
- `prompt_control.py`: Python protocol-equivalent fixture
