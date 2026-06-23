# Prompt Control

Prompt Control is the repo that combines two pieces of the same contract workflow:

- `JSONContractValidator`: deterministic JSON contract enforcement
- `DevContracts`: SCML contract definitions, parsing, and template generation

## How The Pieces Fit

1. `DevContracts` parses an SCML contract into a structured Go contract model.
2. Your application uses that model as the reference shape for structured LLM outputs.
3. `JSONContractValidator` checks the returned JSON against the reference shape.
4. `DevContracts` can also render the final contract text from the validated result.

Used together, they give you a contract-based workflow that is human-readable, machine-checkable, and deterministic.

## Quick Import Paths

```go
import (
    promptcontrol "github.com/PromptFunctions/promptcontrol/JSONContractValidator"
    "github.com/PromptFunctions/promptcontrol/DevContracts/scml"
)
```

## When To Read Which README

- Read [DevContracts/README.md](DevContracts/README.md) if you are authoring or parsing SCML contracts.
- Read [JSONContractValidator/README.md](JSONContractValidator/README.md) if you are validating structured JSON outputs.

## Minimal Example

```go
package main

import (
    "fmt"
    "log"
    "os"

    promptcontrol "github.com/PromptFunctions/promptcontrol/JSONContractValidator"
    "github.com/PromptFunctions/promptcontrol/DevContracts/scml"
)

func main() {
    contractPath := os.Args[1]
    contract, err := scml.ParseFile(contractPath)
    if err != nil {
        log.Fatal(err)
    }

    _ = contract // parse the contract source of truth first

    type ReferenceShape struct {
        Issue struct {
            Description []string `json:"description"`
        } `json:"issue"`
    }

    payload := map[string]any{}
    status, missing := promptcontrol.JSONContract(payload, ReferenceShape{})
    fmt.Println(status, missing)
}
```

## Repo Layout

- `JSONContractValidator/` - JSON enforcement engine
- `DevContracts/` - SCML contract parser, templates, and contract sources
