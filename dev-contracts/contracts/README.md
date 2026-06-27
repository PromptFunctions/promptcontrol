# contracts

`contracts` is the SCML engine room in Prompt Control.

It takes human-written contract files and turns them into structured views the rest of the system can use. Think of it as the compiler for Prompt Control contracts.

## At a glance

- input: SCML contract
- output:
  - parsed contract model
  - rendered JSON shape
  - structured-output schema
  - validation metadata
  - derived policy view

Most applications should use [`../forge`](../forge/README.md) instead of calling this package directly.

## Minimal Example

```go
package main

import (
	"fmt"

	contracts "github.com/PromptFunctions/promptcontrol/dev-contracts/contracts"
)

func main() {
	contract, err := contracts.ParseFile("dev-contracts/contracts/IRSEV_CONTRACT_SCML.md")
	if err != nil {
		panic(err)
	}

	fmt.Println(contract.SectionsView()["issue"])
	fmt.Println(contract.ConstantsView()["SCOPE_CORE"])
	fmt.Println(contract.RenderView().Title)
}
```

## Use This Package When

- you are authoring or debugging SCML
- you want direct access to the parsed contract model
- you need render, schema, validation, or policy views without the full forge loop

For the normal app workflow, use [`../forge`](../forge/README.md).
