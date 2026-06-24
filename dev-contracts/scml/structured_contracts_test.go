package scml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"
)

func TestSCMLLanguageConventions(t *testing.T) {
	expectedTypes := []string{"scml", "constants", "pre", "section"}
	for _, typ := range expectedTypes {
		if _, ok := SCMLLanguageConventions.Types[typ]; !ok {
			t.Fatalf("missing allowed type %q", typ)
		}
	}
	if len(SCMLLanguageConventions.Types) != len(expectedTypes) {
		t.Fatalf("unexpected number of allowed types: got %d want %d", len(SCMLLanguageConventions.Types), len(expectedTypes))
	}
	if _, ok := SCMLLanguageConventions.Attributes["name"]; !ok {
		t.Fatalf("missing allowed attribute name")
	}
	if _, ok := SCMLLanguageConventions.Attributes["depends-on"]; !ok {
		t.Fatalf("missing allowed attribute depends-on")
	}
	if _, ok := SCMLLanguageConventions.Attributes["data-type"]; !ok {
		t.Fatalf("missing allowed attribute data-type")
	}
	if _, ok := SCMLLanguageConventions.Attributes["data-policy"]; !ok {
		t.Fatalf("missing allowed attribute data-policy")
	}
	if _, ok := SCMLLanguageConventions.Attributes["data-source"]; !ok {
		t.Fatalf("missing allowed attribute data-source")
	}
	if len(SCMLLanguageConventions.Attributes) != 5 {
		t.Fatalf("unexpected number of allowed attributes: got %d want 5", len(SCMLLanguageConventions.Attributes))
	}
}

func TestParseFile_SCMLContract(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCMLPATH", dir)

	contractsDir := filepath.Join(dir, "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "report.txt"), []byte("report\n"), 0o600); err != nil {
		t.Fatalf("WriteFile report.txt failed: %v", err)
	}

	writeNamedContract(t, contractsDir, "SOLUTION.md", `
# Solution Module

<!-- <scml> -->
<!-- <constants> -->
<pre>
SOLUTION_FILE = "report.txt"
</pre>
<!-- </constants> -->

<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
## SOLUTION
  - ${SOLUTION_FILE}
<!-- </section> -->
<!-- </scml> -->
`)

	contractPath := writeNamedContract(t, contractsDir, "IRSEV_CONTRACT_SCML.md", `
# IRSEV Framework

A minimal, structured prompt framework for guiding code changes with precision.
Should be iterative-friendly in mind and promote efficient back-and-forth.

<!-- <scml> -->
<!-- <import path="contracts/SOLUTION.md" name="SOLUTION"/> -->
<!-- <constants> -->
<pre>
SCOPE_CORE = "changes limited to explicitly listed files and functions"
GUARDRAIL = "no full file rewrites or refactors"
</pre>
<!-- </constants> -->

## ISSUE
<!-- <section name="issue"> -->
  - Describe the objective, change request, or observed problem.
  - Use concrete examples when applicable (before → after).
  - ${SCOPE_CORE}
<!-- </section> -->

## ROOT CAUSE
<!-- <section name="root-cause"> -->
  - Identify the specific mechanism causing the issue.
  - Point to the exact function / logic responsible.
<!-- </section> -->

## SOLUTION
<!-- <section name="solution" data-source="SOLUTION.solution"> -->
<!-- </section> -->

## EXECUTION
<!-- <section name="execution"> -->
<!-- <section name="steps"> -->
  - Provide precise code-level actions.
<!-- </section> -->
<!-- <section name="failure-modes"> -->
  - Define explicit failure modes for each step.
<!-- </section> -->
<!-- </section> -->

## VALIDATION
<!-- <section name="validation"> -->
  - List concrete checks.
  - ${GUARDRAIL}
<!-- </section> -->
<!-- </scml> -->
`)

	contract, err := ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}

	wantSections := []string{"issue", "root-cause", "solution", "execution", "validation"}
	if len(contract.OrderedSections) != len(wantSections) {
		t.Fatalf("unexpected ordered section count: got %d want %d", len(contract.OrderedSections), len(wantSections))
	}
	for i, want := range wantSections {
		if got := contract.OrderedSections[i].Name; got != want {
			t.Fatalf("unexpected section order at index %d: got %q want %q", i, got, want)
		}
	}

	if got := contract.Sections["issue"]; len(got) < 2 {
		t.Fatalf("ISSUE must keep its top-level list items, got %v", got)
	} else {
		if got[0] != "Describe the objective, change request, or observed problem." {
			t.Fatalf("unexpected issue[0]: %q", got[0])
		}
		if got[1] != "Use concrete examples when applicable (before → after)." {
			t.Fatalf("unexpected issue[1]: %q", got[1])
		}
	}

	if got := contract.Sections["execution"]; len(got) != 0 {
		t.Fatalf("EXECUTION should have no direct section items in the current fixture, got %v", got)
	}
	if got := contract.OrderedSections[2]; got.DataType != "file-list" || got.DataPolicy != "write" || len(got.Items) != 1 {
		t.Fatalf("SOLUTION section should be imported from module, got %+v", got)
	}
	if got := contract.Sections["solution"]; len(got) != 1 || got[0] != "report.txt" {
		t.Fatalf("unexpected imported SOLUTION items: %v", got)
	}
	if len(contract.Imports) != 1 {
		t.Fatalf("unexpected import count: got %d want 1", len(contract.Imports))
	}
	if contract.Imports[0].Name != "SOLUTION" || contract.Imports[0].Path != "contracts/SOLUTION.md" {
		t.Fatalf("unexpected import metadata: %+v", contract.Imports[0])
	}
	if got := contract.PolicyView().WriteAllowlist; len(got) != 1 || got[0] != "report.txt" {
		t.Fatalf("unexpected write allowlist: %v", got)
	}

	if got := contract.Constants["SCOPE_CORE"]; got != "changes limited to explicitly listed files and functions" {
		t.Fatalf("unexpected SCOPE_CORE value: %q", got)
	}
	if got := contract.Constants["GUARDRAIL"]; got != "no full file rewrites or refactors" {
		t.Fatalf("unexpected GUARDRAIL value: %q", got)
	}

	if got := contract.OrderedConstants; len(got) != 2 {
		t.Fatalf("unexpected ordered constant count: got %d want 2", len(got))
	} else {
		if got[0].Key != "SCOPE_CORE" || got[1].Key != "GUARDRAIL" {
			t.Fatalf("unexpected constant order: %+v", got)
		}
	}

	for _, section := range contract.OrderedSections {
		for _, item := range section.Items {
			if strings.Contains(item, "${") {
				t.Fatalf("section %q still contains unresolved constant token: %q", section.Name, item)
			}
		}
		for _, route := range section.Routes {
			if strings.Contains(strings.Join(route.Items, "\n"), "${") {
				t.Fatalf("route %q still contains unresolved constant token", route.Path)
			}
		}
	}

	rendered, err := json.Marshal(contract.RenderView())
	if err != nil {
		t.Fatalf("marshal render view failed: %v", err)
	}
	if !strings.Contains(string(rendered), `"Title":"IRSEV Framework"`) {
		t.Fatalf("render view missing title: %s", string(rendered))
	}
	if strings.Contains(string(rendered), "${") {
		t.Fatalf("render view contains unresolved constant token: %s", string(rendered))
	}

	templateText := contract.GoTemplate()
	if strings.TrimSpace(templateText) == "" {
		t.Fatalf("GoTemplate returned empty template")
	}
	if templateText != contract.GoTemplate() {
		t.Fatalf("GoTemplate must be deterministic across calls")
	}
	if !strings.Contains(templateText, "<!-- <scml> -->") {
		t.Fatalf("template missing scml root")
	}
	if !strings.Contains(templateText, "{{ renderSection . }}") {
		t.Fatalf("template missing renderSection helper call")
	}

	tmpl, err := template.New("contract").Funcs(TemplateFuncMap()).Parse(templateText)
	if err != nil {
		t.Fatalf("template parse failed: %v", err)
	}

	view := contract.TemplateView()
	if len(view.Constants) != len(contract.OrderedConstants) {
		t.Fatalf("template constants size mismatch: got %d want %d", len(view.Constants), len(contract.OrderedConstants))
	}
	if len(view.Sections) != len(contract.OrderedSections) {
		t.Fatalf("template sections size mismatch: got %d want %d", len(view.Sections), len(contract.OrderedSections))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}

	renderedTemplate := buf.String()
	if strings.Contains(renderedTemplate, "${") {
		t.Fatalf("template execute output contains unresolved constant token: %s", renderedTemplate)
	}

	for _, token := range []string{
		"<!-- <section name=\"issue\"> -->",
		"<!-- <section name=\"root-cause\"> -->",
		"<!-- <section name=\"solution\" data-source=\"SOLUTION.solution\" data-type=\"file-list\" data-policy=\"write\"> -->",
		"<!-- <section name=\"execution\"> -->",
		"<!-- <section name=\"validation\"> -->",
	} {
		if !strings.Contains(renderedTemplate, token) {
			t.Fatalf("rendered template missing token %q", token)
		}
	}
}

func TestContractRenderValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCMLPATH", dir)

	contractsDir := filepath.Join(dir, "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "report.txt"), []byte("report\n"), 0o600); err != nil {
		t.Fatalf("WriteFile report.txt failed: %v", err)
	}

	contractPath := writeNamedContract(t, contractsDir, "render_validation.md", `
# Render Validation

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - report.txt
<!-- </section> -->
<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - report.txt
<!-- </section> -->
<!-- <section name="block"> -->
<!-- <section name="nested" data-type="file-list" data-policy="write"> -->
  - report.txt
<!-- </section> -->
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`)

	contract, err := ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}

	validation := contract.RenderValidation()
	if len(validation.FileLists) != 3 {
		t.Fatalf("unexpected file-list validation count: got %d want 3", len(validation.FileLists))
	}
	if validation.FileLists[0].ItemsPath != "Sections[0].Items" {
		t.Fatalf("unexpected first items path: %q", validation.FileLists[0].ItemsPath)
	}
	if validation.FileLists[1].ItemsPath != "Sections[1].Routes[0].Items" {
		t.Fatalf("unexpected nested items path: %q", validation.FileLists[1].ItemsPath)
	}
	if validation.FileLists[2].ItemsPath != "Sections[1].Routes[2].Items" {
		t.Fatalf("unexpected flattened child items path: %q", validation.FileLists[2].ItemsPath)
	}
	for _, spec := range validation.FileLists {
		if spec.BaseDir != contractsDir {
			t.Fatalf("unexpected base dir for %q: got %q want %q", spec.ItemsPath, spec.BaseDir, contractsDir)
		}
		if spec.Mode != RenderValidationModeOpenCardinality {
			t.Fatalf("unexpected validation mode for %q: got %q want %q", spec.ItemsPath, spec.Mode, RenderValidationModeOpenCardinality)
		}
	}

	schema, err := json.Marshal(contract.RenderSchema())
	if err != nil {
		t.Fatalf("marshal render schema failed: %v", err)
	}
	for _, want := range []string{
		`"required":["Title","Constants","Sections"]`,
		`"required":["Name","DependsOn","DataSource","DataType","DataPolicy","Items","Routes"]`,
		`"required":["Term","Path","DependsOn","DataSource","DataType","DataPolicy","Items"]`,
		`"items":{"type":"string"}`,
	} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("render schema missing %q: %s", want, string(schema))
		}
	}
	if strings.Contains(string(schema), `"Children"`) {
		t.Fatalf("render schema should not expose recursive children: %s", string(schema))
	}
	if strings.Contains(string(schema), `"$defs"`) {
		t.Fatalf("render schema should be instance-shaped, not generic defs-based: %s", string(schema))
	}
	if strings.Contains(string(schema), `"minItems":3`) || strings.Contains(string(schema), `"maxItems":3`) {
		t.Fatalf("render schema should avoid fixed array cardinality in Anthropic subset: %s", string(schema))
	}
	if strings.Contains(string(schema), `"oneOf"`) {
		t.Fatalf("render schema should avoid oneOf in Anthropic subset: %s", string(schema))
	}
	if strings.Contains(string(schema), `"const"`) {
		t.Fatalf("render schema should avoid const in Anthropic subset: %s", string(schema))
	}
}

func TestParseFile_CommentWrappedNestedRoutes(t *testing.T) {
	content := `
# IRSEV Framework

<!-- <scml> -->
<!-- <constants> -->
<pre>
SCOPE_CORE = "scope"
</pre>
<!-- </constants> -->

<!-- <section name="EXECUTION"> -->
## EXECUTION
  - top level item
<!-- <section name="steps"> -->
  - step a
  - step b
<!-- </section> -->
<!-- <section name="failure_modes"> -->
  - failure a
<!-- </section> -->
<!-- <section name="block"> -->
<!-- <section name="nested_block"> -->
  - nested one
<!-- </section> -->
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`

	path := writeTempContract(t, content)
	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile nested SCML routes failed: %v", err)
	}

	if len(contract.OrderedSections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(contract.OrderedSections))
	}
	sec := contract.OrderedSections[0]
	if sec.Name != "EXECUTION" {
		t.Fatalf("unexpected section name: %q", sec.Name)
	}
	if len(sec.Items) != 1 || sec.Items[0] != "top level item" {
		t.Fatalf("unexpected top-level section items: %+v", sec.Items)
	}
	if len(sec.Routes) != 3 {
		t.Fatalf("expected 3 top-level routes, got %d", len(sec.Routes))
	}
	if sec.Routes[0].Term != "steps" || sec.Routes[1].Term != "failure_modes" || sec.Routes[2].Term != "block" {
		t.Fatalf("unexpected top route order: %+v", sec.Routes)
	}
	if len(sec.Routes[2].Children) != 1 || sec.Routes[2].Children[0].Term != "nested_block" {
		t.Fatalf("unexpected nested route tree under block: %+v", sec.Routes[2].Children)
	}

	rendered := contract.RenderView()
	if len(rendered.Sections) != 1 {
		t.Fatalf("expected 1 rendered section, got %d", len(rendered.Sections))
	}
	if len(rendered.Sections[0].Routes) != 4 {
		t.Fatalf("expected 4 flattened rendered routes, got %d", len(rendered.Sections[0].Routes))
	}
	expectedRenderedPaths := []string{"execution.steps", "execution.failure_modes", "execution.block", "execution.block.nested_block"}
	for i, want := range expectedRenderedPaths {
		if got := rendered.Sections[0].Routes[i].Path; got != want {
			t.Fatalf("unexpected flattened rendered route at index %d: got %q want %q", i, got, want)
		}
	}
	validation := contract.RenderValidation()
	if len(validation.FileLists) != 0 {
		t.Fatalf("expected no file-list validation entries for nested route fixture, got %v", validation.FileLists)
	}

	routesBySection, ok := contract.SectionRoutes["execution"]
	if !ok {
		t.Fatalf("expected section routes for execution")
	}
	expectedPaths := []string{"execution.steps", "execution.failure_modes", "execution.block", "execution.block.nested_block"}
	for _, path := range expectedPaths {
		if _, ok := routesBySection[path]; !ok {
			t.Fatalf("missing route path %q", path)
		}
	}

	if got := contract.Sections["EXECUTION"]; len(got) != 1 || got[0] != "top level item" {
		t.Fatalf("unexpected section items map value: %v", got)
	}

	tpl := contract.GoTemplate()
	tplExec, err := template.New("contract").Funcs(TemplateFuncMap()).Parse(tpl)
	if err != nil {
		t.Fatalf("template parse failed: %v", err)
	}
	var out bytes.Buffer
	if err := tplExec.Execute(&out, contract.TemplateView()); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}
	renderedTemplate := out.String()
	if !strings.Contains(renderedTemplate, "<!-- <section name=\"steps\"> -->") {
		t.Fatalf("rendered template missing steps section: %s", renderedTemplate)
	}
	if !strings.Contains(renderedTemplate, "<!-- <section name=\"nested_block\"> -->") {
		t.Fatalf("rendered template missing nested route block: %s", renderedTemplate)
	}
}

func TestParseFile_HyphenatedSectionNames(t *testing.T) {
	content := `
# Hyphenated Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->

<!-- <section name="root-cause"> -->
## ROOT-CAUSE
  - issue summary
<!-- <section name="failure-modes"> -->
  - nested summary
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`

	path := writeTempContract(t, content)
	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile hyphenated SCML failed: %v", err)
	}

	if len(contract.OrderedSections) != 1 {
		t.Fatalf("expected 1 ordered section, got %d", len(contract.OrderedSections))
	}
	if got := contract.OrderedSections[0].Name; got != "root-cause" {
		t.Fatalf("unexpected section name: %q", got)
	}
	if got := contract.OrderedSections[0].Routes; len(got) != 1 || got[0].Term != "failure-modes" {
		t.Fatalf("unexpected nested route tree: %+v", got)
	}

	routesBySection, ok := contract.SectionRoutes["root-cause"]
	if !ok {
		t.Fatalf("expected route map entry for root-cause")
	}
	if _, ok := routesBySection["root-cause.failure-modes"]; !ok {
		t.Fatalf("missing canonical path for hyphenated nested route")
	}
}

func TestParseFile_DependsOnRoundTrip(t *testing.T) {
	content := `
# Research DAG

<!-- <scml> -->
<!-- <constants> -->
<pre>
</pre>
<!-- </constants> -->

<!-- <section name="foundations"> -->
  - fundamentals
<!-- </section> -->

<!-- <section name="landscape" depends-on="foundations"> -->
  - compare approaches
<!-- </section> -->

<!-- <section name="deep-analysis" depends-on="foundations,landscape"> -->
  - analyze candidates
<!-- </section> -->
<!-- </scml> -->
`

	path := writeTempContract(t, content)
	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile depends-on contract failed: %v", err)
	}

	if got := contract.OrderedSections[0].DependsOn; got != "" {
		t.Fatalf("unexpected depends-on for foundations: %q", got)
	}
	if got := contract.OrderedSections[1].DependsOn; got != "foundations" {
		t.Fatalf("unexpected depends-on for landscape: %q", got)
	}
	if got := contract.OrderedSections[2].DependsOn; got != "foundations,landscape" {
		t.Fatalf("unexpected depends-on for deep-analysis: %q", got)
	}

	rendered := contract.RenderView()
	if got := rendered.Sections[1].DependsOn; got != "foundations" {
		t.Fatalf("unexpected rendered depends-on for landscape: %q", got)
	}
	if got := rendered.Sections[2].DependsOn; got != "foundations,landscape" {
		t.Fatalf("unexpected rendered depends-on for deep-analysis: %q", got)
	}

	tpl := contract.GoTemplate()
	tplExec, err := template.New("contract").Funcs(TemplateFuncMap()).Parse(tpl)
	if err != nil {
		t.Fatalf("template parse failed: %v", err)
	}
	var out bytes.Buffer
	if err := tplExec.Execute(&out, contract.TemplateView()); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}
	renderedTemplate := out.String()
	for _, token := range []string{
		`<!-- <section name="landscape" depends-on="foundations"> -->`,
		`<!-- <section name="deep-analysis" depends-on="foundations,landscape"> -->`,
	} {
		if !strings.Contains(renderedTemplate, token) {
			t.Fatalf("rendered template missing token %q", token)
		}
	}
}

func TestParseFile_StrictFailures(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		errContains string
	}{
		{
			name: "unknown element",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="ISSUE"> -->
<!-- <frobnicate> -->
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "invalid SCML comment marker",
		},
		{
			name: "unknown attribute",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section title="ISSUE"> -->
  - item
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "unknown XML attribute \"title\" on <section>",
		},
		{
			name: "data policy without data type",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-policy="write"> -->
  - /workspace/allowed.go
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "data-policy requires data-type",
		},
		{
			name: "unknown depends-on reference",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="FOUNDATIONS"> -->
  - item
<!-- </section> -->
<!-- <section name="ANALYSIS" depends-on="missing"> -->
  - item
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: `section "ANALYSIS" depends-on unknown section "missing"`,
		},
		{
			name: "invalid space in name",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="EXECUTION"> -->
<!-- <section name="failure modes"> -->
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "invalid section name \"failure modes\"",
		},
		{
			name: "malformed constants block",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
INVALID_LINE
</pre>
<!-- </constants> -->
<!-- <section name="ISSUE"> -->
  - item
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "invalid constant line",
		},
		{
			name: "duplicate constant key",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
K = "w"
</pre>
<!-- </constants> -->
<!-- <section name="ISSUE"> -->
  - item
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "duplicate constant key",
		},
		{
			name: "missing name attribute",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section> -->
<!-- </scml> -->
`,
			errContains: "invalid SCML comment marker",
		},
		{
			name: "raw xml rejected",
			content: `
<scml>
<section name="ISSUE">
</section>
</scml>
`,
			errContains: "raw XML-like syntax must be wrapped in HTML comments",
		},
		{
			name: "missing write path",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
  - missing.txt
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "no targeted files were found on disk",
		},
		{
			name: "empty write glob",
			content: `
<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
  - missing/*.go
<!-- </section> -->
<!-- </scml> -->
`,
			errContains: "no targeted files were found on disk",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempContract(t, tc.content)
			_, err := ParseFile(path)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %q", tc.errContains, err.Error())
			}
		})
	}
}

func TestParseFile_FileListPathValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCMLPATH", dir)

	if err := os.WriteFile(filepath.Join(dir, "allowed.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	path := writeNamedContract(t, dir, "contract.md", `
# Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
  - allowed.go
<!-- </section> -->
<!-- </scml> -->
`)

	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile file-list validation failed: %v", err)
	}

	if got := contract.PolicyView().WriteAllowlist; len(got) != 1 || got[0] != "allowed.go" {
		t.Fatalf("unexpected write allowlist: %v", got)
	}
}

func TestParseFile_FileListPathPartialResolution(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCMLPATH", dir)

	if err := os.WriteFile(filepath.Join(dir, "allowed.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	path := writeNamedContract(t, dir, "contract.md", `
# Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
  - allowed.go
  - missing.go
<!-- </section> -->
<!-- </scml> -->
`)

	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile partial file-list validation failed: %v", err)
	}

	if got := contract.PolicyView().WriteAllowlist; len(got) != 2 {
		t.Fatalf("expected raw write allowlist to remain unchanged, got %v", got)
	}
}

func TestParseFile_AbsoluteWritePathValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCMLPATH", dir)

	missingPath := filepath.Join(dir, "missing.txt")
	path := writeNamedContract(t, dir, "contract.md", fmt.Sprintf(`
# Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
  - %s
<!-- </section> -->
<!-- </scml> -->
`, missingPath))

	_, err := ParseFile(path)
	if err == nil || !strings.Contains(err.Error(), "no targeted files were found on disk") {
		t.Fatalf("expected missing absolute write path error, got %v", err)
	}
}

func TestParseFile_Imports(t *testing.T) {
	t.Run("explicit alias and replacement", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		writeNamedContract(t, dir, "solution.md", `
# Imported Solution

<!-- <scml> -->
<!-- <constants> -->
<pre>
	SOLUTION_FILE = "report.txt"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
## SOLUTION
  - ${SOLUTION_FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		if err := os.WriteFile(filepath.Join(dir, "report.txt"), []byte("report\n"), 0o600); err != nil {
			t.Fatalf("WriteFile report.txt failed: %v", err)
		}

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-source="SOLUTION.solution"> -->
  - local placeholder
<!-- </section> -->
<!-- <import path="solution.md" name="SOLUTION"/> -->
<!-- </scml> -->
`)

		contract, err := ParseFile(rootPath)
		if err != nil {
			t.Fatalf("ParseFile import contract failed: %v", err)
		}
		if len(contract.Imports) != 1 || contract.Imports[0].Name != "SOLUTION" {
			t.Fatalf("unexpected import metadata: %+v", contract.Imports)
		}
		if got := contract.OrderedSections[0]; got.DataType != "file-list" || got.DataPolicy != "write" {
			t.Fatalf("imported section not bound correctly: %+v", got)
		}
		if got := contract.Sections["solution"]; len(got) != 1 || got[0] != "report.txt" {
			t.Fatalf("imported body should replace local body, got %v", got)
		}
		if !contract.PolicyView().Allows(PolicyActionWrite, "report.txt") {
			t.Fatalf("expected write allowlist to include imported file")
		}
	})

	t.Run("default alias", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		writeNamedContract(t, dir, "builder.md", `
# Builder Module

<!-- <scml> -->
<!-- <constants> -->
<pre>
	BUILDER_FILE = "output.txt"
</pre>
<!-- </constants> -->
<!-- <section name="builder" data-type="file-list" data-policy="write"> -->
  - ${BUILDER_FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		if err := os.WriteFile(filepath.Join(dir, "output.txt"), []byte("output\n"), 0o600); err != nil {
			t.Fatalf("WriteFile output.txt failed: %v", err)
		}

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="builder.md"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="builder" data-source="builder.builder"> -->
<!-- </section> -->
<!-- </scml> -->
`)

		contract, err := ParseFile(rootPath)
		if err != nil {
			t.Fatalf("ParseFile default alias contract failed: %v", err)
		}
		if len(contract.Imports) != 1 || contract.Imports[0].Name != "" {
			t.Fatalf("unexpected raw import metadata: %+v", contract.Imports)
		}
		if got := contract.Sections["builder"]; len(got) != 1 || got[0] != "output.txt" {
			t.Fatalf("default alias import did not bind body correctly: %v", got)
		}
	})

	t.Run("plain scml extension", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		writeNamedContract(t, dir, "library.scml", `
# Library

<!-- <scml> -->
<!-- <constants> -->
<pre>
	LIB_FILE = "library.txt"
</pre>
<!-- </constants> -->
<!-- <section name="library" data-type="file-list" data-policy="write"> -->
  - ${LIB_FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		if err := os.WriteFile(filepath.Join(dir, "library.txt"), []byte("library\n"), 0o600); err != nil {
			t.Fatalf("WriteFile library.txt failed: %v", err)
		}

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="library.scml"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="library" data-source="library.library"> -->
<!-- </section> -->
<!-- </scml> -->
`)

		contract, err := ParseFile(rootPath)
		if err != nil {
			t.Fatalf("ParseFile plain scml import failed: %v", err)
		}
		if got := contract.Sections["library"]; len(got) != 1 || got[0] != "library.txt" {
			t.Fatalf("plain scml import did not bind body correctly: %v", got)
		}
	})

	t.Run("aggregate missing from import and root", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		modulesDir := filepath.Join(dir, "modules")
		if err := os.MkdirAll(modulesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll modules failed: %v", err)
		}

		writeNamedContract(t, modulesDir, "solution.md", `
# Imported Solution

<!-- <scml> -->
<!-- <constants> -->
<pre>
	FILE = "missing-import.txt"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - ${FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="modules/solution.md" name="SOLUTION"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-source="SOLUTION.solution"> -->
<!-- </section> -->
<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - missing-root.txt
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`)

		_, err := ParseFile(rootPath)
		if err == nil {
			t.Fatalf("expected aggregate missing error")
		}
		var missingErr *MissingWriteTargetsError
		if !errors.As(err, &missingErr) {
			t.Fatalf("expected MissingWriteTargetsError, got %T: %v", err, err)
		}
		if got, want := missingErr.Missing, []string{"missing-import.txt", "missing-root.txt"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("unexpected missing targets: got %v want %v", got, want)
		}
	})

	t.Run("imported missing and root found succeeds", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		modulesDir := filepath.Join(dir, "modules")
		if err := os.MkdirAll(modulesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll modules failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "root-found.txt"), []byte("root\n"), 0o600); err != nil {
			t.Fatalf("WriteFile root-found.txt failed: %v", err)
		}

		writeNamedContract(t, modulesDir, "solution.md", `
# Imported Solution

<!-- <scml> -->
<!-- <constants> -->
<pre>
	FILE = "missing-import.txt"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - ${FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="modules/solution.md" name="SOLUTION"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-source="SOLUTION.solution"> -->
<!-- </section> -->
<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - root-found.txt
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`)

		contract, err := ParseFile(rootPath)
		if err != nil {
			t.Fatalf("ParseFile mixed graph contract failed: %v", err)
		}
		resolution, err := ResolveContractWriteTargets(contract)
		if err != nil {
			t.Fatalf("ResolveContractWriteTargets failed: %v", err)
		}
		if got, want := resolution.Missing, []string{"missing-import.txt"}; len(got) != len(want) || got[0] != want[0] {
			t.Fatalf("unexpected missing targets: got %v want %v", got, want)
		}
		if got := resolution.Found["root-found.txt"]; len(got) != 1 || filepath.Base(got[0]) != "root-found.txt" {
			t.Fatalf("unexpected resolved root target: %v", got)
		}
	})

	t.Run("imported relative write paths resolve against imported contract directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		modulesDir := filepath.Join(dir, "modules")
		if err := os.MkdirAll(modulesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll modules failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(modulesDir, "report.txt"), []byte("report\n"), 0o600); err != nil {
			t.Fatalf("WriteFile report.txt failed: %v", err)
		}

		writeNamedContract(t, modulesDir, "solution.md", `
# Imported Solution

<!-- <scml> -->
<!-- <constants> -->
<pre>
	FILE = "report.txt"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - ${FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="modules/solution.md" name="SOLUTION"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-source="SOLUTION.solution"> -->
<!-- </section> -->
<!-- </scml> -->
`)

		contract, err := ParseFile(rootPath)
		if err != nil {
			t.Fatalf("ParseFile imported relative path contract failed: %v", err)
		}
		resolution, err := ResolveContractWriteTargets(contract)
		if err != nil {
			t.Fatalf("ResolveContractWriteTargets failed: %v", err)
		}
		if got := resolution.Found["report.txt"]; len(got) != 1 || got[0] != filepath.Join(modulesDir, "report.txt") {
			t.Fatalf("unexpected imported resolved target: %v", got)
		}
	})

	t.Run("recursive imports and cycles", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		writeNamedContract(t, dir, "leaf.md", `
# Leaf

<!-- <scml> -->
<!-- <constants> -->
<pre>
	LEAF_FILE = "leaf.txt"
</pre>
<!-- </constants> -->
<!-- <section name="mid" data-type="file-list" data-policy="write"> -->
  - ${LEAF_FILE}
<!-- </section> -->
<!-- </scml> -->
`)

		if err := os.WriteFile(filepath.Join(dir, "leaf.txt"), []byte("leaf\n"), 0o600); err != nil {
			t.Fatalf("WriteFile leaf.txt failed: %v", err)
		}
		writeNamedContract(t, dir, "mid.md", `
# Mid

<!-- <scml> -->
<!-- <import path="leaf.md" name="LEAF"/> -->
<!-- <constants> -->
<pre>
MID_ROOT = "/workspace/mid"
</pre>
<!-- </constants> -->
<!-- <section name="mid" data-source="LEAF.mid"> -->
<!-- </section> -->
<!-- </scml> -->
`)
		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="mid.md" name="MID"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="mid" data-source="MID.mid"> -->
<!-- </section> -->
<!-- </scml> -->
`)

		contract, err := ParseFile(rootPath)
		if err != nil {
			t.Fatalf("ParseFile recursive import contract failed: %v", err)
		}
		if got := contract.Sections["mid"]; len(got) != 1 || got[0] != "leaf.txt" {
			t.Fatalf("recursive import did not bind through intermediate module: %v", got)
		}

		cycleLeft := writeNamedContract(t, dir, "cycle-left.md", `
# Left

<!-- <scml> -->
<!-- <import path="cycle-right.md" name="RIGHT"/> -->
<!-- <constants> -->
<pre>
LEFT_ROOT = "/workspace/left"
</pre>
<!-- </constants> -->
<!-- <section name="left" data-source="RIGHT.right"> -->
<!-- </section> -->
<!-- </scml> -->
`)
		writeNamedContract(t, dir, "cycle-right.md", `
# Right

<!-- <scml> -->
<!-- <import path="cycle-left.md" name="LEFT"/> -->
<!-- <constants> -->
<pre>
RIGHT_ROOT = "/workspace/right"
</pre>
<!-- </constants> -->
<!-- <section name="right" data-source="LEFT.left"> -->
<!-- </section> -->
<!-- </scml> -->
`)

		_, err = ParseFile(cycleLeft)
		if err == nil || !strings.Contains(err.Error(), "import cycle detected") {
			t.Fatalf("expected import cycle error, got %v", err)
		}
	})

	t.Run("missing import file", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="missing.md" name="MISSING"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="root"> -->
  - item
<!-- </section> -->
<!-- </scml> -->
`)

		_, err := ParseFile(rootPath)
		if err == nil || (!strings.Contains(err.Error(), "file does not exist") && !strings.Contains(err.Error(), "unable to resolve SCML import")) {
			t.Fatalf("expected missing import file error, got %v", err)
		}
	})

	t.Run("unsupported import extension", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("SCMLPATH", dir)

		rootPath := writeNamedContract(t, dir, "root.md", `
# Root

<!-- <scml> -->
<!-- <import path="bad.txt" name="BAD"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="root"> -->
  - item
<!-- </section> -->
<!-- </scml> -->
`)

		_, err := ParseFile(rootPath)
		if err == nil || !strings.Contains(err.Error(), `unsupported import file extension ".txt"`) {
			t.Fatalf("expected unsupported import extension error, got %v", err)
		}
	})
}

func TestParse_WithOptions(t *testing.T) {
	t.Run("reader skip write validation", func(t *testing.T) {
		contractText := `# Reader Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - missing.txt
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`

		contract, err := Parse(strings.NewReader(contractText), &ParseOptions{
			BaseDir:             "/virtual/contracts",
			SkipWriteValidation: true,
		})
		if err != nil {
			t.Fatalf("Parse(reader) failed: %v", err)
		}

		validation := contract.RenderValidation()
		if len(validation.FileLists) != 1 {
			t.Fatalf("unexpected file-list count: got %d want 1", len(validation.FileLists))
		}
		if validation.FileLists[0].BaseDir != "/virtual/contracts" {
			t.Fatalf("unexpected base dir: got %q want %q", validation.FileLists[0].BaseDir, "/virtual/contracts")
		}
	})

	t.Run("reader keeps write validation by default", func(t *testing.T) {
		contractText := `# Reader Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - missing.txt
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`

		_, err := Parse(strings.NewReader(contractText), &ParseOptions{BaseDir: "/virtual/contracts"})
		if err == nil || err.Error() != "no targeted files were found on disk" {
			t.Fatalf("expected write validation error, got %v", err)
		}
	})

	t.Run("file parse with injected fs and search path override", func(t *testing.T) {
		t.Setenv("SCMLPATH", "/does/not/need/to/exist")

		fs := newMockFS(map[string]string{
			"/virtual/contracts/root.md": `# Root

<!-- <scml> -->
<!-- <import path="contracts/SOLUTION.md" name="SOLUTION"/> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-source="SOLUTION.solution"> -->
<!-- </section> -->
<!-- </scml> -->
`,
			"/virtual/contracts/SOLUTION.md": `# Solution

<!-- <scml> -->
<!-- <constants> -->
<pre>
SOLUTION_FILE = "report.txt"
</pre>
<!-- </constants> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - ${SOLUTION_FILE}
<!-- </section> -->
<!-- </scml> -->
`,
			"/virtual/contracts/report.txt": "report\n",
		})

		contract, err := ParseFileWithOptions("/virtual/contracts/root.md", &ParseOptions{
			FS:          fs,
			SearchPaths: []string{"/virtual"},
		})
		if err != nil {
			t.Fatalf("ParseFileWithOptions failed: %v", err)
		}
		if got := contract.Sections["solution"]; len(got) != 1 || got[0] != "report.txt" {
			t.Fatalf("unexpected imported section items: %v", got)
		}
		if got := contract.PolicyView().WriteAllowlist; len(got) != 1 || got[0] != "report.txt" {
			t.Fatalf("unexpected write allowlist: %v", got)
		}
	})
}

func writeTempContract(t *testing.T, content string) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "scml-contract-*.md")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}

	return file.Name()
}

func writeNamedContract(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", path, err)
	}
	return path
}

type mockFS struct {
	files map[string]string
}

func newMockFS(files map[string]string) *mockFS {
	copied := make(map[string]string, len(files))
	for path, content := range files {
		copied[filepath.Clean(path)] = content
	}
	return &mockFS{files: copied}
}

func (m *mockFS) ReadFile(name string) ([]byte, error) {
	content, ok := m.files[filepath.Clean(name)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return []byte(content), nil
}

func (m *mockFS) Stat(name string) (iofs.FileInfo, error) {
	cleaned := filepath.Clean(name)
	if content, ok := m.files[cleaned]; ok {
		return mockFileInfo{name: filepath.Base(cleaned), size: int64(len(content))}, nil
	}
	for path := range m.files {
		if path == cleaned {
			continue
		}
		if strings.HasPrefix(path, cleaned+string(filepath.Separator)) {
			return mockFileInfo{name: filepath.Base(cleaned), dir: true}, nil
		}
	}
	return nil, os.ErrNotExist
}

func (m *mockFS) Glob(pattern string) ([]string, error) {
	matches := make([]string, 0, len(m.files))
	for path := range m.files {
		ok, err := filepath.Match(pattern, path)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, path)
		}
	}
	return matches, nil
}

type mockFileInfo struct {
	name string
	size int64
	dir  bool
}

func (m mockFileInfo) Name() string { return m.name }
func (m mockFileInfo) Size() int64  { return m.size }

func (m mockFileInfo) Mode() iofs.FileMode {
	if m.dir {
		return iofs.ModeDir | 0o755
	}
	return 0o644
}

func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.dir }
func (m mockFileInfo) Sys() any           { return nil }
