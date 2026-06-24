package gating

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)

func TestValidateContractCompleted(t *testing.T) {
	_, contract := writeValidatorFixture(t)
	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Routes[0].Items = []string{"report.txt"}
	rendered.Sections[1].Routes[1].Items = []string{"mock step"}

	result := ValidateContract(mustRenderMap(t, rendered), contract.RenderView(), toValidationSpec(contract.RenderValidation()))
	if result.Status != JSONContractCompletedStatus {
		t.Fatalf("unexpected status: got %q want %q missing=%v", result.Status, JSONContractCompletedStatus, result.Missing)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("expected no missing items, got %v", result.Missing)
	}
}

func TestValidateContractMissingStructure(t *testing.T) {
	_, contract := writeValidatorFixture(t)
	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Routes[0].Items = []string{"report.txt"}
	rendered.Sections[1].Routes[1].Items = []string{"mock step"}

	candidate := mustRenderMap(t, rendered)
	sections := candidate["Sections"].([]any)
	section0 := sections[0].(map[string]any)
	delete(section0, "Name")

	result := ValidateContract(candidate, contract.RenderView(), toValidationSpec(contract.RenderValidation()))
	if result.Status != JSONContractMissingStatus {
		t.Fatalf("unexpected status: got %q want %q", result.Status, JSONContractMissingStatus)
	}
	if !slices.Contains(result.Missing, "Sections[0].Name") {
		t.Fatalf("missing list does not contain structural path: %v", result.Missing)
	}
}

func TestValidateContractMissingFile(t *testing.T) {
	_, contract := writeValidatorFixture(t)
	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Routes[0].Items = []string{"missing.txt"}
	rendered.Sections[1].Routes[1].Items = []string{"mock step"}

	result := ValidateContract(mustRenderMap(t, rendered), contract.RenderView(), toValidationSpec(contract.RenderValidation()))
	if result.Status != JSONContractMissingStatus {
		t.Fatalf("unexpected status: got %q want %q", result.Status, JSONContractMissingStatus)
	}
	if !slices.Contains(result.Missing, "file not found on disk: missing.txt") {
		t.Fatalf("missing list does not contain missing file: %v", result.Missing)
	}
}

func TestValidateContractFileListAllowsDifferentCount(t *testing.T) {
	_, contract := writeValidatorFixture(t)
	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Routes[0].Items = []string{"report.txt", "other.txt"}
	rendered.Sections[1].Routes[1].Items = []string{"mock step"}

	result := ValidateContract(mustRenderMap(t, rendered), contract.RenderView(), toValidationSpec(contract.RenderValidation()))
	if result.Status != JSONContractCompletedStatus {
		t.Fatalf("unexpected status: got %q want %q missing=%v", result.Status, JSONContractCompletedStatus, result.Missing)
	}
}

func TestValidateContractFileListMixedValidityFailsAllBadEntries(t *testing.T) {
	_, contract := writeValidatorFixture(t)
	rendered := contract.RenderView()
	rendered.Sections[0].Items = []string{"mock issue"}
	rendered.Sections[1].Routes[0].Items = []string{"report.txt", "missing-a.txt", "missing-b.txt"}
	rendered.Sections[1].Routes[1].Items = []string{"mock step"}

	result := ValidateContract(mustRenderMap(t, rendered), contract.RenderView(), toValidationSpec(contract.RenderValidation()))
	if result.Status != JSONContractMissingStatus {
		t.Fatalf("unexpected status: got %q want %q", result.Status, JSONContractMissingStatus)
	}
	for _, want := range []string{
		"file not found on disk: missing-a.txt",
		"file not found on disk: missing-b.txt",
	} {
		if !slices.Contains(result.Missing, want) {
			t.Fatalf("missing list does not contain %q: %v", want, result.Missing)
		}
	}
}

func writeValidatorFixture(t *testing.T) (string, *scml.Contract) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("SCMLPATH", root)
	contractsDir := filepath.Join(root, "contracts")
	if err := os.MkdirAll(contractsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "report.txt"), []byte("report\n"), 0o600); err != nil {
		t.Fatalf("WriteFile report.txt failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contractsDir, "other.txt"), []byte("other\n"), 0o600); err != nil {
		t.Fatalf("WriteFile other.txt failed: %v", err)
	}

	contractPath := filepath.Join(contractsDir, "validator.md")
	contractText := `# Validator Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
K = "v"
</pre>
<!-- </constants> -->

<!-- <section name="issue"> -->
  - Describe the issue.
<!-- </section> -->

<!-- <section name="execution"> -->
<!-- <section name="scope" data-type="file-list" data-policy="write"> -->
  - report.txt
<!-- </section> -->
<!-- <section name="steps"> -->
  - Describe the steps.
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
	return contractPath, contract
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

func toValidationSpec(validation scml.RenderValidation) ValidationSpec {
	spec := ValidationSpec{
		FileLists: make([]FileListSpec, 0, len(validation.FileLists)),
	}
	for _, fileList := range validation.FileLists {
		spec.FileLists = append(spec.FileLists, FileListSpec{
			ItemsPath: fileList.ItemsPath,
			BaseDir:   fileList.BaseDir,
			Mode:      fileList.Mode,
		})
	}
	return spec
}
