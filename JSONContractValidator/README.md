# JSONContractValidator

`JSONContractValidator` is the JSON enforcement half of Prompt Control. It checks whether a returned JSON object matches the contract shape expected by your application.

## What It Does

- Returns `completed` when all required keys are present.
- Returns `missing_fields` with a deterministic, sorted list of missing keys otherwise.
- Uses a Bloom filter as a fast pre-check and an exact key map as the authoritative check.

## Core API

```go
status, missing := promptcontrol.JSONContract(candidateJSON, contractShape)
```

Constants:

- `JSONContractCompletedStatus` = `completed`
- `JSONContractMissingStatus` = `missing_fields`

## Typical Workflow

1. Parse the contract shape from `DevContracts`.
2. Use that shape as the structured-output reference in your LLM call.
3. Validate the returned JSON with `JSONContractValidator`.
4. Re-prompt using the missing-key list until the contract is complete.

## Third-Party Usage

Import path:

```go
import promptcontrol "github.com/PromptFunctions/promptcontrol/JSONContractValidator"
```

Install:

```bash
go get github.com/PromptFunctions/promptcontrol/JSONContractValidator
```

Minimal example:

```go
package main

import (
    "fmt"

    promptcontrol "github.com/PromptFunctions/promptcontrol/JSONContractValidator"
)

func main() {
    type ContractShape struct {
        Issue struct {
            Description []string `json:"description"`
        } `json:"issue"`
    }

    candidate := map[string]any{
        "issue": map[string]any{
            "description": []any{"example"},
        },
    }

    status, missing := promptcontrol.JSONContract(candidate, ContractShape{})
    fmt.Println(status, missing)
}
```

## Relationship To DevContracts

`DevContracts` defines the contract structure.
`JSONContractValidator` enforces that the returned JSON matches it exactly.

Read [../README.md](../README.md) for the combined workflow overview.
