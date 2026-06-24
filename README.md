# Prompt Control

Prompt Control is the repo that combines two pieces of the same contract workflow:

- `gating`: deterministic JSON contract enforcement
- `dev-contracts`: SCML contract definitions, parsing, and template generation

## How The Pieces Fit

1. `dev-contracts` parses an SCML contract into a structured Go contract model.
2. Your application uses that model as the rendered reference shape for structured LLM outputs.
3. `gating` checks the returned JSON against the rendered reference shape.
4. `dev-contracts` can also render the final contract text from the validated result.

Used together, they give you a contract-based workflow that is human-readable, machine-checkable, and deterministic.

## Quick Import Paths

```go
import (
    gating "github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
    "github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)
```

## When To Read Which README

- Read [dev-contracts/README.md](dev-contracts/README.md) if you are authoring or parsing SCML contracts.
- Read [dev-contracts/gating/README.md](dev-contracts/gating/README.md) if you are validating structured JSON outputs.

## Minimal Example

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "os"

    gating "github.com/PromptFunctions/promptcontrol/dev-contracts/gating"
    "github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)

func main() {
    contractPath := os.Args[1]
    contract, err := scml.ParseFile(contractPath)
    if err != nil {
        log.Fatal(err)
    }

    reference := contract.RenderView()
    validation := toValidationSpec(contract.RenderValidation())
    payload := mustRenderMap(reference)

    result := gating.ValidateContract(payload, reference, validation)
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

## Repo Layout

- `dev-contracts/gating/` - JSON enforcement engine
- `dev-contracts/` - SCML contract parser, templates, and contract sources
