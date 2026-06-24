package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)

type stubClient struct {
	systemPrompts []string
	prompts       []string
	responses     []stubResponse
}

type stubResponse struct {
	text string
	err  error
}

func (s *stubClient) Complete(_ context.Context, systemPrompt, prompt string, _ map[string]any) (string, error) {
	s.systemPrompts = append(s.systemPrompts, systemPrompt)
	s.prompts = append(s.prompts, prompt)
	if len(s.responses) == 0 {
		return "", fmt.Errorf("unexpected forge call")
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response.text, response.err
}

func TestRun_RetriesUntilValidated(t *testing.T) {
	contract := writeForgeFixture(t)

	rendered := contract.RenderView()
	first := contract.RenderView()
	first.Sections[0].Items = []string{"mock issue"}
	first.Sections[1].Items = []string{"missing.txt"}

	second := contract.RenderView()
	second.Sections[0].Items = []string{""}
	second.Sections[1].Items = []string{"report.txt"}

	stub := &stubClient{
		responses: []stubResponse{
			{text: mustMarshalJSON(t, first)},
			{text: mustMarshalJSON(t, second)},
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	cfg.Timeout = time.Second

	var trace bytes.Buffer
	outputBytes, err := Run(contract, stub, cfg, &trace)
	if err != nil {
		t.Fatalf("Run failed: %v\n%s", err, trace.String())
	}

	if len(stub.prompts) != 2 {
		t.Fatalf("expected 2 forge calls, got %d", len(stub.prompts))
	}
	if !strings.Contains(stub.prompts[1], "missing.txt") {
		t.Fatalf("retry prompt missing validator feedback: %q", stub.prompts[1])
	}
	for _, want := range []string{
		"Mock tree snapshot:",
		"root: contracts",
		"  - report.txt",
	} {
		if !strings.Contains(stub.systemPrompts[0], want) {
			t.Fatalf("system prompt missing tree snapshot content %q: %q", want, stub.systemPrompts[0])
		}
	}
	for _, want := range []string{
		"Reference shape:",
		"Current checkpoint:",
		`"DataPolicy": "write"`,
		`"Items": [`,
	} {
		if !strings.Contains(stub.prompts[1], want) {
			t.Fatalf("retry prompt missing full-shape content %q: %q", want, stub.prompts[1])
		}
	}
	if !strings.Contains(stub.prompts[0], `[LLM TO FILL INSTRUCTIONS: Describe the objective.]`) {
		t.Fatalf("initial prompt should wrap prose instructions as placeholders: %q", stub.prompts[0])
	}
	if strings.Contains(stub.prompts[0], `[LLM TO FILL INSTRUCTIONS: report.txt]`) {
		t.Fatalf("prompt should not wrap file-list items as instruction placeholders: %q", stub.prompts[0])
	}

	output := string(outputBytes)
	if strings.Contains(output, "title:") {
		t.Fatalf("stdout should be final JSON only, got %q", output)
	}
	if !strings.Contains(output, `"report.txt"`) {
		t.Fatalf("stdout missing forged file path: %q", output)
	}
	if strings.Contains(output, `"missing.txt"`) {
		t.Fatalf("stdout should not contain the rejected retry path: %q", output)
	}
	if !strings.Contains(output, `"mock issue"`) {
		t.Fatalf("stdout should preserve previously valid content across retries: %q", output)
	}

	var forged scml.RenderContract
	if err := json.Unmarshal(outputBytes, &forged); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, output)
	}
	if forged.Title != rendered.Title {
		t.Fatalf("unexpected forged title: got %q want %q", forged.Title, rendered.Title)
	}

	traceOutput := trace.String()
	for _, want := range []string{
		"rendered contract:",
		"system prompt:",
		"user prompt:",
		"returned keys count:",
		"missing keys count:",
		"returned keys:",
		"  - Sections",
		"  - Sections[0].Items[0]",
		"attempt 1/2",
		"mode: initial scaffold",
		"prompt bytes: ",
		"response bytes: ",
		"merge: seeded working tree from full response",
		"retry 1/2",
		"attempt 2/2",
		"mode: fill missing pieces",
		"requested fixes: 1",
		"applied updates: 1",
		"  - file not found on disk: missing.txt -> Sections[1].Items[0]",
		"missing files:",
		"  - file not found on disk: missing.txt",
	} {
		if !strings.Contains(traceOutput, want) {
			t.Fatalf("trace missing retry diagnostics %q: %q", want, traceOutput)
		}
	}
}

func TestRun_InvalidModelJSONFails(t *testing.T) {
	contract := writeForgeFixture(t)
	stub := &stubClient{
		responses: []stubResponse{{text: `{"Title":`}},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 1
	cfg.Timeout = time.Second

	var trace bytes.Buffer
	_, err := Run(contract, stub, cfg, &trace)
	if err == nil || !strings.Contains(err.Error(), "invalid model JSON") {
		t.Fatalf("expected invalid model JSON error, got %v", err)
	}
}

func TestRun_RetryExhaustionFails(t *testing.T) {
	contract := writeForgeFixture(t)

	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Items = []string{"missing.txt"}

	stub := &stubClient{
		responses: []stubResponse{
			{text: mustMarshalJSON(t, rendered)},
			{text: mustMarshalJSON(t, rendered)},
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	cfg.Timeout = time.Second

	var trace bytes.Buffer
	_, err := Run(contract, stub, cfg, &trace)
	if err == nil {
		t.Fatalf("expected retry exhaustion error")
	}

	var retryErr *RetryError
	if !strings.Contains(err.Error(), "retry limit exhausted after 2 attempts") {
		t.Fatalf("unexpected retry error: %v", err)
	}
	if !strings.Contains(trace.String(), "validation missing count: 1") {
		t.Fatalf("trace missing validation count: %q", trace.String())
	}
	if !strings.Contains(trace.String(), "missing files:\n  - file not found on disk: missing.txt") {
		t.Fatalf("trace missing missing-file block: %q", trace.String())
	}
	if !errorAsRetry(err, &retryErr) || retryErr == nil {
		t.Fatalf("expected RetryError, got %T", err)
	}
}

func TestRun_RetryMergesMissingMetadataOnly(t *testing.T) {
	contract := writeForgeFixture(t)

	first := mustRenderMap(t, contract.RenderView())
	sections := first["Sections"].([]any)
	issue := sections[0].(map[string]any)
	issue["Items"] = []any{"mock issue"}
	solution := sections[1].(map[string]any)
	delete(solution, "DataPolicy")

	second := contract.RenderView()
	second.Sections[0].Items = []string{""}

	stub := &stubClient{
		responses: []stubResponse{
			{text: mustMarshalJSON(t, first)},
			{text: mustMarshalJSON(t, second)},
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	cfg.Timeout = time.Second

	outputBytes, err := Run(contract, stub, cfg, ioDiscard{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	output := string(outputBytes)
	if !strings.Contains(output, `"DataPolicy": "write"`) {
		t.Fatalf("stdout missing merged metadata field: %q", output)
	}
	if !strings.Contains(output, `"mock issue"`) {
		t.Fatalf("metadata merge should preserve prior valid content: %q", output)
	}
	if len(stub.prompts) != 1 {
		t.Fatalf("metadata-only gaps should converge on the first attempt, got %d prompts", len(stub.prompts))
	}
	if !strings.Contains(stub.prompts[0], `"DataPolicy": "write"`) {
		t.Fatalf("initial prompt should carry full-shape metadata from reference: %q", stub.prompts[0])
	}
}

func TestRun_RetryMergesMissingRouteEntry(t *testing.T) {
	contract := writeForgeRouteFixture(t)

	first := mustRenderMap(t, contract.RenderView())
	sections := first["Sections"].([]any)
	issue := sections[0].(map[string]any)
	issue["Items"] = []any{"mock issue"}
	execution := sections[1].(map[string]any)
	routes := execution["Routes"].([]any)
	execution["Routes"] = routes[:1]

	second := contract.RenderView()
	second.Sections[0].Items = []string{""}

	stub := &stubClient{
		responses: []stubResponse{
			{text: mustMarshalJSON(t, first)},
			{text: mustMarshalJSON(t, second)},
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	cfg.Timeout = time.Second

	outputBytes, err := Run(contract, stub, cfg, ioDiscard{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	output := string(outputBytes)
	for _, want := range []string{`"execution.scope"`, `"execution.steps"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing merged route entry %q: %q", want, output)
		}
	}
	if !strings.Contains(output, `"mock issue"`) {
		t.Fatalf("stdout should preserve existing valid section items: %q", output)
	}
	if !strings.Contains(stub.prompts[1], `"execution.steps"`) {
		t.Fatalf("retry prompt should preserve full route shape: %q", stub.prompts[1])
	}
}

func TestFormatMissingGroups_PrettyLabels(t *testing.T) {
	reference := scml.RenderContract{
		Title: "Contract",
		Sections: []scml.RenderSectionEntry{
			{Name: "ISSUE", Items: []string{"describe"}},
			{
				Name: "EXECUTION",
				Routes: []scml.RenderRouteEntry{
					{Path: "execution.scope", Items: []string{"report.txt"}},
					{Path: "execution.steps", Items: []string{"step"}},
				},
			},
		},
	}

	groups := formatMissingGroups(reference, []string{
		"Sections[0].Items",
		"Sections[1].Routes[0].Items[0]",
		"Sections[1].Routes[1].DataType",
		"missing.txt",
		"freeform value",
	})

	var builder strings.Builder
	for _, group := range groups {
		builder.WriteString(group.Title)
		builder.WriteByte('\n')
		for _, item := range group.Items {
			builder.WriteString(item)
			builder.WriteByte('\n')
		}
	}
	output := builder.String()
	for _, want := range []string{
		"missing section items\nISSUE\n",
		"missing route items\nexecution.scope\n",
		"missing fields\nexecution.steps missing data-type\n",
		"missing files\nmissing.txt\n",
		"other missing values\nfreeform value\n",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted groups missing %q: %q", want, output)
		}
	}
}

func TestMergeMissingValues_FilePath(t *testing.T) {
	contract := writeForgeFixture(t)

	first := contract.RenderView()
	first.Sections[0].Items = []string{"mock issue"}
	first.Sections[1].Items = []string{"missing.txt"}

	second := contract.RenderView()
	second.Sections[0].Items = []string{""}
	second.Sections[1].Items = []string{"report.txt"}

	working := mustRenderMap(t, first)
	source := mustRenderMap(t, second)
	paths := locateFileItemPaths(working, toValidationSpec(contract.RenderValidation()), "missing.txt")
	if len(paths) != 1 || paths[0] != "Sections[1].Items[0]" {
		t.Fatalf("unexpected located file paths: %v", paths)
	}
	sourceValue, ok := readJSONPath(source, "Sections[1].Items[0]")
	if !ok || sourceValue.(string) != "report.txt" {
		t.Fatalf("unexpected source file value: %#v ok=%v", sourceValue, ok)
	}
	checkpoint := buildAttemptScaffold(contract.RenderView(), working)
	merge := mergeMissingValues(working, source, checkpoint, []string{"file not found on disk: missing.txt"}, toValidationSpec(contract.RenderValidation()))
	if len(merge.Applied) != 1 || merge.Applied[0] != "file not found on disk: missing.txt -> Sections[1].Items[0]" {
		t.Fatalf("unexpected merge applied paths: %v", merge.Applied)
	}
	if len(merge.Skipped) != 0 {
		t.Fatalf("unexpected merge skipped paths: %v", merge.Skipped)
	}

	value, ok := readJSONPath(working, "Sections[1].Items[0]")
	if !ok {
		t.Fatalf("expected merged item path")
	}
	if got, _ := value.(string); got != "report.txt" {
		t.Fatalf("unexpected merged file value: got %q want %q", got, "report.txt")
	}
}

func TestBuildAttemptScaffold_PreservesFullShape(t *testing.T) {
	contract := writeForgeRouteFixture(t)

	working := map[string]any{
		"Sections": []any{
			map[string]any{
				"Items": []any{"mock issue"},
			},
		},
	}

	scaffold := buildAttemptScaffold(contract.RenderView(), working)
	sections, ok := scaffold["Sections"].([]any)
	if !ok || len(sections) != 2 {
		t.Fatalf("unexpected scaffold sections: %#v", scaffold["Sections"])
	}
	issue := sections[0].(map[string]any)
	if items := issue["Items"].([]any); len(items) != 1 || items[0] != "mock issue" {
		t.Fatalf("unexpected issue items: %#v", issue["Items"])
	}
	execution := sections[1].(map[string]any)
	if execution["Name"] != "execution" {
		t.Fatalf("unexpected execution name: %#v", execution["Name"])
	}
	routes, ok := execution["Routes"].([]any)
	if !ok || len(routes) != 2 {
		t.Fatalf("unexpected routes scaffold: %#v", execution["Routes"])
	}
	scope := routes[0].(map[string]any)
	if scope["DataPolicy"] != "write" {
		t.Fatalf("unexpected scope data policy: %#v", scope["DataPolicy"])
	}
	if items := scope["Items"].([]any); len(items) != 0 {
		t.Fatalf("unexpected scope items: %#v", scope["Items"])
	}
	steps := routes[1].(map[string]any)
	if steps["Path"] != "execution.steps" {
		t.Fatalf("unexpected steps path: %#v", steps["Path"])
	}
}

func TestBuildPromptScaffold_UsesInstructionPlaceholders(t *testing.T) {
	contract := writeForgeRouteFixture(t)

	checkpoint := map[string]any{
		"Sections": []any{
			map[string]any{
				"Items": []any{"mock issue"},
			},
		},
	}

	scaffold := buildPromptScaffold(contract.RenderView(), checkpoint)
	sections, ok := scaffold["Sections"].([]any)
	if !ok || len(sections) != 2 {
		t.Fatalf("unexpected scaffold sections: %#v", scaffold["Sections"])
	}

	issue := sections[0].(map[string]any)
	issueItems := issue["Items"].([]any)
	if issueItems[0] != "mock issue" {
		t.Fatalf("expected populated checkpoint prose to survive, got %#v", issueItems[0])
	}

	execution := sections[1].(map[string]any)
	if execution["Name"] != "execution" {
		t.Fatalf("unexpected execution name: %#v", execution["Name"])
	}
	routes := execution["Routes"].([]any)
	scope := routes[0].(map[string]any)
	if scope["DataPolicy"] != "write" {
		t.Fatalf("unexpected scope data policy: %#v", scope["DataPolicy"])
	}
	scopeItems := scope["Items"].([]any)
	if len(scopeItems) != 0 {
		t.Fatalf("expected file-list items to remain unwrapped when empty, got %#v", scope["Items"])
	}

	steps := routes[1].(map[string]any)
	stepItems := steps["Items"].([]any)
	want := `[LLM TO FILL INSTRUCTIONS: describe step]`
	if len(stepItems) != 1 || stepItems[0] != want {
		t.Fatalf("unexpected prompt placeholder items: %#v", steps["Items"])
	}
	if steps["Path"] != "execution.steps" || steps["Term"] != "steps" {
		t.Fatalf("unexpected route metadata: %#v", steps)
	}
}

func writeForgeFixture(t *testing.T) *scml.Contract {
	t.Helper()

	repoRoot := t.TempDir()
	t.Setenv("SCMLPATH", repoRoot)
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n"), 0o600); err != nil {
		t.Fatalf("WriteFile go.mod failed: %v", err)
	}

	contractsDir := filepath.Join(repoRoot, "dev-contracts", "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "report.txt"), []byte("report\n"), 0o600); err != nil {
		t.Fatalf("WriteFile report.txt failed: %v", err)
	}

	contractPath := filepath.Join(contractsDir, "valid.md")
	contractText := `# Valid Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->

<!-- <section name="issue"> -->
  - Describe the objective.
<!-- </section> -->

<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - report.txt
<!-- </section> -->
<!-- </scml> -->
`
	if err := os.WriteFile(contractPath, []byte(contractText), 0o600); err != nil {
		t.Fatalf("WriteFile contract failed: %v", err)
	}

	contract, err := scml.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}
	return contract
}

func writeForgeRouteFixture(t *testing.T) *scml.Contract {
	t.Helper()

	repoRoot := t.TempDir()
	t.Setenv("SCMLPATH", repoRoot)
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n"), 0o600); err != nil {
		t.Fatalf("WriteFile go.mod failed: %v", err)
	}

	contractsDir := filepath.Join(repoRoot, "dev-contracts", "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "report.txt"), []byte("report\n"), 0o600); err != nil {
		t.Fatalf("WriteFile report.txt failed: %v", err)
	}

	contractPath := filepath.Join(contractsDir, "routes.md")
	contractText := `# Route Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->

<!-- <section name="issue"> -->
  - Describe the objective.
<!-- </section> -->

<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - report.txt
<!-- </section> -->
<!-- <section name="steps"> -->
  - describe step
<!-- </section> -->
<!-- </section> -->
<!-- </scml> -->
`
	if err := os.WriteFile(contractPath, []byte(contractText), 0o600); err != nil {
		t.Fatalf("WriteFile contract failed: %v", err)
	}

	contract, err := scml.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}
	return contract
}

func mustMarshalJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return string(data)
}

func mustRenderMap(t *testing.T, rendered scml.RenderContract) map[string]any {
	t.Helper()
	data, err := json.Marshal(rendered)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	return out
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

func errorAsRetry(err error, target **RetryError) bool {
	if err == nil {
		return false
	}
	retryErr, ok := err.(*RetryError)
	if ok {
		*target = retryErr
		return true
	}
	return false
}

func TestRunFile_InvalidContractError(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(contractPath, []byte(`<contract>
<section name="issue">
</section>
</contract>
`), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	stub := &stubClient{}
	cfg := DefaultConfig()
	cfg.MaxRetries = 1
	cfg.Timeout = time.Second

	_, err := RunFile(contractPath, stub, cfg, nil)
	if err == nil {
		t.Fatalf("expected error from RunFile")
	}

	var ice *InvalidContractError
	if !errors.As(err, &ice) {
		t.Fatalf("expected InvalidContractError, got %T: %v", err, err)
	}
	if ice.Path != contractPath {
		t.Fatalf("unexpected path: got %q want %q", ice.Path, contractPath)
	}
	if ice.Reason == nil {
		t.Fatalf("expected non-nil Reason")
	}
	if !strings.Contains(ice.Reason.Error(), "raw XML-like syntax must be wrapped in HTML comments") {
		t.Fatalf("unexpected reason: %v", ice.Reason)
	}
}

func TestRunFile_InvalidContractError_Unwrap(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(contractPath, []byte(`<contract></contract>`), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	stub := &stubClient{}
	cfg := DefaultConfig()
	cfg.MaxRetries = 1
	cfg.Timeout = time.Second

	_, err := RunFile(contractPath, stub, cfg, nil)
	if err == nil {
		t.Fatalf("expected error from RunFile")
	}

	var ice *InvalidContractError
	if !errors.As(err, &ice) {
		t.Fatalf("expected InvalidContractError, got %T: %v", err, err)
	}
	if ice.Unwrap() == nil {
		t.Fatalf("expected Unwrap to return underlying error")
	}
}

func TestRunFile_DelegatesToRun(t *testing.T) {
	contract := writeForgeFixture(t)
	contractPath := findContractPath(t, contract)

	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Items = []string{"report.txt"}

	stub := &stubClient{
		responses: []stubResponse{
			{text: mustMarshalJSON(t, rendered)},
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 1
	cfg.Timeout = time.Second

	output, err := RunFile(contractPath, stub, cfg, nil)
	if err != nil {
		t.Fatalf("RunFile failed: %v", err)
	}
	if !strings.Contains(string(output), `"mock issue"`) {
		t.Fatalf("output missing expected content: %s", output)
	}
}

func TestWriteReturnedKeys_UsesStructuralDenominator(t *testing.T) {
	contract := writeForgeFixture(t)
	reference := contract.RenderView()
	validation := toValidationSpec(contract.RenderValidation())
	expected := expectedReturnedKeys(reference, validation)

	candidate := mustRenderMap(t, reference)
	sections := candidate["Sections"].([]any)
	solution := sections[1].(map[string]any)
	items := solution["Items"].([]any)
	solution["Items"] = append(items, "extra-report.txt")

	var buf bytes.Buffer
	writeReturnedKeys(&buf, candidate, expected)
	output := buf.String()

	wantReturned := fmt.Sprintf("returned keys count: 100%% (%d/%d)", len(expected), len(expected))
	if !strings.Contains(output, wantReturned) {
		t.Fatalf("expected full structural coverage, got %q", output)
	}
	wantMissing := fmt.Sprintf("missing keys count: 0%% (0/%d)", len(expected))
	if !strings.Contains(output, wantMissing) {
		t.Fatalf("expected zero missing structural keys, got %q", output)
	}
	if !strings.Contains(output, "  - Sections[1].Items[1]") {
		t.Fatalf("expected raw returned keys to include extra file-list item key, got %q", output)
	}
}

func TestRunFile_SkipsTemplateWriteValidation(t *testing.T) {
	dir := t.TempDir()
	contractPath := filepath.Join(dir, "template.md")
	if err := os.WriteFile(contractPath, []byte(`# Template

<!-- <scml> -->
<!-- <constants> -->
<pre>
ROOT = "/workspace/root"
</pre>
<!-- </constants> -->
<!-- <section name="issue"> -->
  - Describe the objective.
<!-- </section> -->
<!-- <section name="solution" data-type="file-list" data-policy="write"> -->
  - missing-template-file.txt
<!-- </section> -->
<!-- </scml> -->
`), 0o600); err != nil {
		t.Fatalf("WriteFile template contract failed: %v", err)
	}

	if _, err := scml.ParseFile(contractPath); err == nil || err.Error() != "no targeted files were found on disk" {
		t.Fatalf("expected strict scml parse to fail on missing file target, got %v", err)
	}

	rendered := scml.RenderContract{
		Title: "Template",
		Sections: []scml.RenderSectionEntry{
			{Name: "issue", Items: []string{"mock issue"}},
			{Name: "solution", DataType: "file-list", DataPolicy: "write", Items: []string{"real.txt"}},
		},
	}

	if err := os.WriteFile(filepath.Join(dir, "real.txt"), []byte("real\n"), 0o600); err != nil {
		t.Fatalf("WriteFile real.txt failed: %v", err)
	}

	stub := &stubClient{
		responses: []stubResponse{
			{text: mustMarshalJSON(t, rendered)},
		},
	}
	cfg := DefaultConfig()
	cfg.MaxRetries = 1
	cfg.Timeout = time.Second

	output, err := RunFile(contractPath, stub, cfg, nil)
	if err != nil {
		var ice *InvalidContractError
		if errors.As(err, &ice) {
			t.Fatalf("RunFile should not reject template contract at load time: %v", ice)
		}
		t.Fatalf("RunFile failed: %v", err)
	}
	if !strings.Contains(string(output), `"real.txt"`) {
		t.Fatalf("output missing validated file-list path: %s", output)
	}
}

func TestRetryError_WriteFormattedMissing(t *testing.T) {
	re := &RetryError{
		reference: scml.RenderContract{
			Title: "Test",
			Sections: []scml.RenderSectionEntry{
				{Name: "ISSUE", Items: []string{"describe"}},
			},
		},
		Missing: []string{"Sections[0].Items"},
		Retries: 1,
	}

	var buf bytes.Buffer
	re.WriteFormattedMissing(&buf)
	output := buf.String()
	if !strings.Contains(output, "missing section items") {
		t.Fatalf("WriteFormattedMissing output missing expected group: %q", output)
	}
	if !strings.Contains(output, "ISSUE") {
		t.Fatalf("WriteFormattedMissing output missing section name: %q", output)
	}
}

func findContractPath(t *testing.T, _ *scml.Contract) string {
	t.Helper()
	scmlpath := os.Getenv("SCMLPATH")
	return filepath.Join(scmlpath, "dev-contracts", "contracts", "valid.md")
}
