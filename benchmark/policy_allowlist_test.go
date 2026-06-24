package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PromptFunctions/promptcontrol/dev-contracts/scml"
)

func TestSCMLPolicyAllowlist(t *testing.T) {
	contractPath := writePolicyContractFixture(t)

	contract, err := scml.ParseFile(contractPath)
	if err != nil {
		t.Fatalf("ParseFile(%q) failed: %v", contractPath, err)
	}

	policy := contract.PolicyView()
	if got, want := policy.WriteAllowlist, []string{
		"allowed.go",
		"config.yaml",
	}; !equalStringSlices(got, want) {
		t.Fatalf("unexpected write allowlist: got %v want %v", got, want)
	}
	if got, want := policy.PermissionTable, []scml.PermissionEntry{
		{Path: "allowed.go", Write: true},
		{Path: "config.yaml", Write: true},
	}; !equalPermissionEntries(got, want) {
		t.Fatalf("unexpected permission table: got %v want %v", got, want)
	}
	if got, want := policy.ReadAllowlist, []string{"input.txt"}; !equalStringSlices(got, want) {
		t.Fatalf("unexpected read allowlist: got %v want %v", got, want)
	}

	if !policy.Allows(scml.PolicyActionWrite, "allowed.go") {
		t.Fatalf("expected write access to allowed.go")
	}
	if policy.Allows(scml.PolicyActionWrite, "blocked.go") {
		t.Fatalf("expected write access to blocked.go to be denied")
	}
	if !policy.Allows(scml.PolicyActionRead, "input.txt") {
		t.Fatalf("expected read access to input.txt")
	}
	if policy.Allows(scml.PolicyActionRead, "../workspace/input.txt") {
		t.Fatalf("expected normalized traversal path to be denied")
	}
	if policy.Allows("delete", "allowed.go") {
		t.Fatalf("unexpected access for unsupported action")
	}
}

func writePolicyContractFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "allowed.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("WriteFile allowed.go failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("name: config\n"), 0o600); err != nil {
		t.Fatalf("WriteFile config.yaml failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("input\n"), 0o600); err != nil {
		t.Fatalf("WriteFile input.txt failed: %v", err)
	}

	content := `# Policy Contract

<!-- <scml> -->
<!-- <constants> -->
<pre>
SCOPE_CORE = "changes limited to explicitly listed files and functions"
</pre>
<!-- </constants> -->

<!-- <section name="WRITES" data-type="file-list" data-policy="write"> -->
## WRITES
  - allowed.go
  - config.yaml
<!-- </section> -->

<!-- <section name="READS" data-type="read-list"> -->
## READS
  - input.txt
<!-- </section> -->
<!-- </scml> -->
`

	path := filepath.Join(dir, "policy-contract.md")
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
