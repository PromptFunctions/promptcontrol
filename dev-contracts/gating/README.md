# gating

`gating` is the JSON enforcement half of Prompt Control. It validates a rendered JSON candidate against a rendered contract reference and optional file-list rules.

## What It Does

- Returns `completed` when the candidate matches the rendered contract requirements.
- Returns `missing_fields` with a deterministic, sorted list of missing paths or missing files otherwise.
- Enforces:
  - structural field presence
  - exact scaffold identity for fields like `Title`, `Name`, `Path`, `DataSource`, `DataType`, `DataPolicy`
  - non-empty item slots
  - file-list disk checks through `ValidationSpec`

## Core API

```go
result := gating.ValidateContract(candidateJSON, referenceJSON, validationSpec)
```

Constants:

- `JSONContractCompletedStatus` = `completed`
- `JSONContractMissingStatus` = `missing_fields`

## Typical Workflow

1. Parse the contract shape from `dev-contracts`.
2. Use `contract.RenderView()` as the structured-output reference.
3. Use `contract.RenderValidation()` to derive file-list validation metadata.
4. Validate the returned JSON with `ValidateContract(...)`.
5. Re-prompt using `result.Missing` until the contract is complete.

## Third-Party Usage

Import path:

```go
import gating "github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
```

Install:

```bash
go get github.com/PromptFunctions/promptcontrol/dev-contracts/gating
```

Minimal example:

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    gating "github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
    "github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)

func main() {
    contract, err := scml.ParseFile("dev-contracts/contracts/IRSEV_CONTRACT_SCML.md")
    if err != nil {
        log.Fatal(err)
    }

    reference := contract.RenderView()
    validation := toValidationSpec(contract.RenderValidation())

    candidate := mustRenderMap(reference)
    result := gating.ValidateContract(candidate, reference, validation)
    fmt.Println(result.Status, result.Missing)
}

func mustRenderMap(rendered scml.RenderContract) map[string]any {
    data, err := json.Marshal(rendered)
    if err != nil {
        log.Fatal(err)
    }
    var out map[string]any
    if err := json.Unmarshal(data, &out); err != nil {
        log.Fatal(err)
    }
    return out
}

func toValidationSpec(renderValidation scml.RenderValidation) gating.ValidationSpec {
    spec := gating.ValidationSpec{
        FileLists: make([]gating.FileListSpec, 0, len(renderValidation.FileLists)),
    }
    for _, fileList := range renderValidation.FileLists {
        spec.FileLists = append(spec.FileLists, gating.FileListSpec{
            ItemsPath: fileList.ItemsPath,
            BaseDir:   fileList.BaseDir,
            Mode:      fileList.Mode,
        })
    }
    return spec
}
```

## Relationship To dev-contracts

`dev-contracts` defines and renders the contract structure.
`gating` validates that returned JSON matches that rendered contract deterministically.

Read [../README.md](../README.md) for the combined workflow overview.
