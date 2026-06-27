# forge

`forge` is the front door.

If you want Prompt Control to do the whole job, this is the package. It loads the SCML contract, builds the JSON scaffold, runs the LLM loop, validates retries, and returns the final JSON.

## Flow

- contract path in
- scaffold built
- LLM called
- validator retries
- final JSON out

It saves you from wiring contracts, prompts, validation, and retries yourself.

## Minimal Shape

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/PromptFunctions/promptcontrol/dev-contracts/forge"
)

type client struct{}

func (client) Complete(ctx context.Context, systemPrompt, userPrompt string, schema map[string]any) (string, error) {
	_ = ctx
	_ = systemPrompt
	_ = userPrompt
	_ = schema
	return `{}`, nil
}

func main() {
	cfg := forge.Config{
		MaxRetries: 3,
		Timeout:    90 * time.Second,
	}

	out, err := forge.RunFile("path/to/contract.md", client{}, cfg, nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(out))
}
```

## Use This Package When

- you want the full Prompt Control workflow
- you do not want to wire `contracts` and `gating` yourself
- you want one package to handle load, render, validate, and retry

If you need direct SCML access, read [`../contracts`](../contracts/README.md).
