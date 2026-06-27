# gating

`gating` is the low-level validator used by `forge`.

It checks a candidate JSON object against a rendered contract reference and returns a deterministic result:

- `completed`
- `missing_fields`

Most users do not need this package directly. Use [`../forge`](../forge/README.md) unless you want custom orchestration.

## Core API

```go
result := gating.ValidateContract(candidate, reference, spec)
```

- `candidate`: JSON decoded into `map[string]any`
- `reference`: usually `contract.RenderView()`
- `spec`: usually derived from `contract.RenderValidation()`

## Use This Package When

- you already have your own LLM loop
- you only want deterministic validation
- you do not need the full forge workflow

For SCML parsing, read [`../contracts`](../contracts/README.md).
