# gating

`gating` is the final check.

It does not generate anything. It decides whether a JSON result is acceptable against a rendered contract reference and a validation spec.

## At a glance

- input:
  - candidate JSON
  - reference render
  - validation spec
- output:
  - `completed`
  - or deterministic missing items

Most users do not need this package directly. Use [`../forge`](../forge/README.md) unless you already own your own LLM loop.

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
