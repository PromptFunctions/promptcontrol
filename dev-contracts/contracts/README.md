# contracts

`contracts` is the SCML package inside Prompt Control.

It parses comment-wrapped SCML files, resolves imports and constants, and gives you the views used by the rest of the system:

- `RenderView()` for the rendered JSON shape
- `RenderSchema()` for structured-output schema generation
- `RenderValidation()` for file-list validation metadata
- `PolicyView()` for derived read/write policy data

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
- you need render/schema/validation views without the full forge loop

For the normal app workflow, use [`../forge`](../forge/README.md).
