# promptcontrol

## Prompt Control (Stateless Contract Enforcement)

`prompt_control` is a structural fixture that demonstrates deterministic contract
enforcement for probabilistic LLM outputs.

### Why this exists

LLMs are probabilistic generators. Even with structured outputs, large or complex
contracts may occasionally return incomplete or partially filled objects.

Prompt Control adds a lightweight validation loop:

1. Flatten the reference contract shape into canonical nested key paths.
2. Build a Bloom filter from those expected key paths.
3. Compare the returned JSON object key paths against the contract.
4. Return:
   - `completed` when all keys are present
   - `missing_fields` + missing key list otherwise

This design is useful at scale because Bloom filters are fast and memory efficient.
Combined with structured outputs from an LLM provider, they provide a performant
contract-enforcement layer for heavy agentic workflows and large JSON objects.

### Included fixtures

- `prompt_control.go`
  - Standalone, stateless Go implementation.
  - Includes Bloom filter + exact map check for deterministic correctness.
- `prompt_control.py`
  - Python equivalent with the same architecture and semantics.
  - Useful for demos, notebooks, and portability across stacks.

### Core guarantees

- Stateless algorithm engine.
- Deterministic missing-key output (sorted).
- Fast membership checks via Bloom filter.
- Exact authoritative key map to avoid false-positive drift.

### Typical workflow

1. Ask LLM for structured output (JSON contract).
2. Run Prompt Control validation.
3. If missing fields remain, re-prompt LLM with missing-key list.
4. Repeat until contract is complete or retry policy decides best-effort fallback.
