//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/PromptFunctions/promptcontrol/DevContracts/scml"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run print_contract.go <contract-path>")
		os.Exit(1)
	}
	contractPath := os.Args[1]

	contract, err := scml.ParseFile(contractPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse contract: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(contract.RenderView(), "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal contract: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== STRUCTURED_CONTRACT_JSON ===")
	fmt.Println(string(out))
	fmt.Println("=== STRUCTURED_CONTRACT_TEMPLATE ===")
	fmt.Println(contract.GoTemplate())
}
