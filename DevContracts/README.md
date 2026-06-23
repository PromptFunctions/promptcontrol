# DevContracts

`DevContracts` is the SCML side of Prompt Control. It defines the contract language, parses contract sources, and generates renderable contract data.

## What It Does

- Parses comment-wrapped SCML contract files into a structured Go contract model.
- Preserves source order for constants, sections, and nested routes.
- Produces both a render view and a Go-template-compatible template view.
- Exposes a tiny conventions file that defines the allowed SCML tags and attributes.

## Core API

```go
contractPath := os.Args[1]
contract, err := scml.ParseFile(contractPath)
if err != nil {
    // handle parse/validation error
}

fmt.Println(contract.Sections["ISSUE"])
fmt.Println(contract.Constants["SCOPE_CORE"])
render := contract.RenderView()
tplText := contract.GoTemplate()
tplView := contract.TemplateView()
```

Returned contract shape:

```go
type Contract struct {
    Sections  map[string][]string
    Constants map[string]string

    OrderedConstants []ConstantEntry
    OrderedSections  []SectionEntry
}
```

## SCML In This Repo

SCML here uses XML-like tags inside HTML comments and is validated strictly.

- root element: `<contract>`
- constants element: `<constants><pre>...</pre></constants>`
- sections: `<section name="...">`
- allowed attribute: `name`
- content items: `- ...`
- constant references in content: `${KEY}`

Nested sections are expressed by nesting `<section>` elements.

## Contract Sources

- `contracts/IRSEV_CONTRACT_SCML.md` - canonical SCML contract source
- `contracts/IRSEV_CONTRACT.md` - legacy markdown contract source retained for reference

## Templates

`GoTemplate()` returns the generic template text for rendering the parsed contract.
`TemplateView()` returns the data structure that executes that template in source order.

## Third-Party Usage

Import path:

```go
import "github.com/PromptFunctions/promptcontrol/DevContracts/scml"
```

Install:

```bash
go get github.com/PromptFunctions/promptcontrol/DevContracts/scml
```

Minimal example:

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "strings"
    "text/template"

    "github.com/PromptFunctions/promptcontrol/DevContracts/scml"
)

func main() {
    contractPath := os.Args[1]
    contract, err := scml.ParseFile(contractPath)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(contract.Sections["ISSUE"])

    out, err := json.MarshalIndent(contract.RenderView(), "", "  ")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(out))

    tpl, err := template.New("contract").Parse(contract.GoTemplate())
    if err != nil {
        log.Fatal(err)
    }

    var b strings.Builder
    if err := tpl.Execute(&b, contract.TemplateView()); err != nil {
        log.Fatal(err)
    }
    fmt.Println(b.String())
}
```

## Relationship To Prompt Control

`DevContracts` defines the contract shape.
`JSONContractValidator` enforces that a returned JSON object matches that shape.

Read [../README.md](../README.md) for the combined workflow overview.
