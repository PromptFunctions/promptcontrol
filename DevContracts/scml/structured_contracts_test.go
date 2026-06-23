package scml

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestSCMLLanguageConventions(t *testing.T) {
	expectedTypes := []string{"contract", "constants", "pre", "section"}
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
	if _, ok := SCMLLanguageConventions.Attributes["data-type"]; !ok {
		t.Fatalf("missing allowed attribute data-type")
	}
	if _, ok := SCMLLanguageConventions.Attributes["data-policy"]; !ok {
		t.Fatalf("missing allowed attribute data-policy")
	}
	if len(SCMLLanguageConventions.Attributes) != 3 {
		t.Fatalf("unexpected number of allowed attributes: got %d want 3", len(SCMLLanguageConventions.Attributes))
	}
}

func TestParseFile_SCMLContract(t *testing.T) {
	contractPath := filepath.Clean("../contracts/IRSEV_CONTRACT_SCML.md")
	contract, err := ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}

	wantSections := []string{"ISSUE", "ROOT_CAUSE", "SOLUTION", "EXECUTION", "VALIDATION"}
	if len(contract.OrderedSections) != len(wantSections) {
		t.Fatalf("unexpected ordered section count: got %d want %d", len(contract.OrderedSections), len(wantSections))
	}
	for i, want := range wantSections {
		if got := contract.OrderedSections[i].Name; got != want {
			t.Fatalf("unexpected section order at index %d: got %q want %q", i, got, want)
		}
	}

	if got := contract.Sections["ISSUE"]; len(got) < 2 {
		t.Fatalf("ISSUE must keep its top-level list items, got %v", got)
	} else {
		if got[0] != "Describe the objective, change request, or observed problem." {
			t.Fatalf("unexpected ISSUE[0]: %q", got[0])
		}
		if got[1] != "Use concrete examples when applicable (before → after)." {
			t.Fatalf("unexpected ISSUE[1]: %q", got[1])
		}
	}

	if got := contract.Sections["EXECUTION"]; len(got) != 0 {
		t.Fatalf("EXECUTION should have no direct section items in the current fixture, got %v", got)
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
	if !strings.Contains(templateText, "<!-- <contract> -->") {
		t.Fatalf("template missing contract root")
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
		"<!-- <section name=\"ISSUE\"> -->",
		"<!-- <section name=\"ROOT_CAUSE\"> -->",
		"<!-- <section name=\"SOLUTION\"> -->",
		"<!-- <section name=\"EXECUTION\"> -->",
		"<!-- <section name=\"VALIDATION\"> -->",
	} {
		if !strings.Contains(renderedTemplate, token) {
			t.Fatalf("rendered template missing token %q", token)
		}
	}
}

func TestParseFile_CommentWrappedNestedRoutes(t *testing.T) {
	content := `
# IRSEV Framework

<!-- <contract> -->
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
<!-- </contract> -->
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
	if sec.Routes[0].Term != "STEPS" || sec.Routes[1].Term != "FAILURE_MODES" || sec.Routes[2].Term != "BLOCK" {
		t.Fatalf("unexpected top route order: %+v", sec.Routes)
	}
	if len(sec.Routes[2].Children) != 1 || sec.Routes[2].Children[0].Term != "NESTED_BLOCK" {
		t.Fatalf("unexpected nested route tree under block: %+v", sec.Routes[2].Children)
	}

	routesBySection, ok := contract.SectionRoutes["execution"]
	if !ok {
		t.Fatalf("expected section routes for execution")
	}
	expectedPaths := []string{"execution.steps", "execution.failure_modes", "execution.block", "execution.block.nested_block"}
	for _, path := range expectedPaths {
		if _, ok := routesBySection[path]; !ok {
			t.Fatalf("missing canonical route path %q", path)
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
	rendered := out.String()
	if !strings.Contains(rendered, "<!-- <section name=\"STEPS\"> -->") {
		t.Fatalf("rendered template missing steps section: %s", rendered)
	}
	if !strings.Contains(rendered, "<!-- <section name=\"NESTED_BLOCK\"> -->") {
		t.Fatalf("rendered template missing nested route block: %s", rendered)
	}
}

func TestParseFile_HyphenatedSectionNames(t *testing.T) {
	content := `
# Hyphenated Contract

<!-- <contract> -->
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
<!-- </contract> -->
`

	path := writeTempContract(t, content)
	contract, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile hyphenated SCML failed: %v", err)
	}

	if len(contract.OrderedSections) != 1 {
		t.Fatalf("expected 1 ordered section, got %d", len(contract.OrderedSections))
	}
	if got := contract.OrderedSections[0].Name; got != "ROOT_CAUSE" {
		t.Fatalf("unexpected section name: %q", got)
	}
	if got := contract.OrderedSections[0].Routes; len(got) != 1 || got[0].Term != "FAILURE_MODES" {
		t.Fatalf("unexpected nested route tree: %+v", got)
	}

	routesBySection, ok := contract.SectionRoutes["root_cause"]
	if !ok {
		t.Fatalf("expected canonical route map entry for root_cause")
	}
	if _, ok := routesBySection["root_cause.failure_modes"]; !ok {
		t.Fatalf("missing canonical path for hyphenated nested route")
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
<!-- <contract> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="ISSUE"> -->
<!-- <frobnicate> -->
<!-- </section> -->
<!-- </contract> -->
`,
			errContains: "invalid SCML comment marker",
		},
		{
			name: "unknown attribute",
			content: `
<!-- <contract> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section title="ISSUE"> -->
  - item
<!-- </section> -->
<!-- </contract> -->
`,
			errContains: "unknown XML attribute \"title\" on <section>",
		},
		{
			name: "data policy without data type",
			content: `
<!-- <contract> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="WRITES" data-policy="write"> -->
  - /workspace/allowed.go
<!-- </section> -->
<!-- </contract> -->
`,
			errContains: "data-policy requires data-type",
		},
		{
			name: "invalid space in name",
			content: `
<!-- <contract> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section name="EXECUTION"> -->
<!-- <section name="failure modes"> -->
<!-- </section> -->
<!-- </section> -->
<!-- </contract> -->
`,
			errContains: "invalid section name \"failure modes\"",
		},
		{
			name: "malformed constants block",
			content: `
<!-- <contract> -->
<!-- <constants> -->
<pre>
INVALID_LINE
</pre>
<!-- </constants> -->
<!-- <section name="ISSUE"> -->
  - item
<!-- </section> -->
<!-- </contract> -->
`,
			errContains: "invalid constant line",
		},
		{
			name: "duplicate constant key",
			content: `
<!-- <contract> -->
<!-- <constants> -->
<pre>
K = "v"
K = "w"
</pre>
<!-- </constants> -->
<!-- <section name="ISSUE"> -->
  - item
<!-- </section> -->
<!-- </contract> -->
`,
			errContains: "duplicate constant key",
		},
		{
			name: "missing name attribute",
			content: `
<!-- <contract> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->
<!-- <section> -->
<!-- </contract> -->
`,
			errContains: "invalid SCML comment marker",
		},
		{
			name: "raw xml rejected",
			content: `
<contract>
<section name="ISSUE">
</section>
</contract>
`,
			errContains: "raw XML-like syntax must be wrapped in HTML comments",
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
