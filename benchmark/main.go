//go:build !compliance && !large
// +build !compliance,!large

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/PromptFunctions/promptcontrol/DevContracts/scml"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./benchmark <contract-path>")
		os.Exit(1)
	}

	contract, err := scml.ParseFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse contract: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(describeContract(contract))
}

func describeContract(contract *scml.Contract) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "title: %q\n", contract.Title)
	builder.WriteString("constants:\n")
	for _, constant := range contract.OrderedConstants {
		fmt.Fprintf(&builder, "  - %s=%q\n", constant.Key, constant.Value)
	}

	builder.WriteString("sections:\n")
	for _, section := range contract.OrderedSections {
		fmt.Fprintf(&builder, "  - name: %q data-type: %q\n", section.Name, section.DataType)
		if len(section.Routes) > 0 {
			builder.WriteString("    routes:\n")
			writeRouteDescriptor(&builder, section.Routes, 3)
		}
	}

	policy := contract.PolicyView()
	builder.WriteString("policy:\n")
	fmt.Fprintf(&builder, "  read: %s\n", formatStringList(policy.ReadAllowlist))
	fmt.Fprintf(&builder, "  write: %s\n", formatPermissionWrites(policy.PermissionTable))

	return builder.String()
}

func writeRouteDescriptor(builder *strings.Builder, routes []scml.RouteNode, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, route := range routes {
		fmt.Fprintf(builder, "%s- path: %q data-type: %q\n", indent, route.Path, route.DataType)
		if len(route.Children) > 0 {
			writeRouteDescriptor(builder, route.Children, depth+1)
		}
	}
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}

	var builder strings.Builder
	builder.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			builder.WriteString(", ")
		}
		fmt.Fprintf(&builder, "%q", value)
	}
	builder.WriteByte(']')
	return builder.String()
}

func formatPermissionWrites(entries []scml.PermissionEntry) string {
	if len(entries) == 0 {
		return "[]"
	}

	values := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Write {
			values = append(values, entry.Path)
		}
	}
	return formatStringList(values)
}
