package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PromptFunctions/promptcontrol/DevContracts/scml"
)

func TestSCMLPolicyAllowlist(t *testing.T) {
	contractPath := writePolicyContractFixture(t)

	contract, err := scml.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}

	policy := contract.PolicyView()
	if got, want := policy.WriteAllowlist, []string{
		"/workspace/allowed.go",
		"/workspace/config.yaml",
	}; !equalStringSlices(got, want) {
		t.Fatalf("unexpected write allowlist: got %v want %v", got, want)
	}
	if got, want := policy.PermissionTable, []scml.PermissionEntry{
		{Path: "/workspace/allowed.go", Write: true},
		{Path: "/workspace/config.yaml", Write: true},
	}; !equalPermissionEntries(got, want) {
		t.Fatalf("unexpected permission table: got %v want %v", got, want)
	}
	if got, want := policy.ReadAllowlist, []string{"/workspace/input.txt"}; !equalStringSlices(got, want) {
		t.Fatalf("unexpected read allowlist: got %v want %v", got, want)
	}

	if !policy.Allows(scml.PolicyActionWrite, "/workspace/allowed.go") {
		t.Fatalf("expected write access to /workspace/allowed.go")
	}
	if policy.Allows(scml.PolicyActionWrite, "/workspace/blocked.go") {
		t.Fatalf("expected write access to /workspace/blocked.go to be denied")
	}
	if !policy.Allows(scml.PolicyActionRead, "/workspace/input.txt") {
		t.Fatalf("expected read access to /workspace/input.txt")
	}
	if policy.Allows(scml.PolicyActionRead, "../workspace/input.txt") {
		t.Fatalf("expected normalized traversal path to be denied")
	}
	if policy.Allows("delete", "/workspace/allowed.go") {
		t.Fatalf("unexpected access for unsupported action")
	}
}

func writePolicyContractFixture(t *testing.T) string {
	t.Helper()

	content := `# Policy Contract

<!-- <contract> -->
<!-- <constants> -->
<pre>
SCOPE_CORE = "changes limited to explicitly listed files and functions"
</pre>
<!-- </constants> -->

<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
## WRITES
  - /workspace/allowed.go
  - /workspace/config.yaml
<!-- </section> -->

<!-- <section name="READS" data-type="read-list"> -->
## READS
  - /workspace/input.txt
<!-- </section> -->
<!-- </contract> -->
`

	path := filepath.Join(t.TempDir(), "policy-contract.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	return path
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func equalPermissionEntries(got, want []scml.PermissionEntry) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
