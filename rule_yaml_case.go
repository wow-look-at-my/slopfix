package slopfix

// The workflow fixtures the yaml rules are driven on. Each rule file names
// the one it needs.

func workflowCase(name, body string) RuleCase {
	return RuleCase{Name: "yaml/" + name, Path: ".github/workflows/ci.yml", Text: body}
}

func workflowHeader() string {
	return "name: CI\n\non:\n  push:\n    branches: ['**']\n\n"
}

// workflowGate is the concurrency block every job in these repositories carries.
const workflowGate = "    concurrency:\n" +
	"      group: gha_${{ github.repository }}_${{ github.workflow }}_${{ github.ref != 'refs/heads/master' && github.ref || github.run_id }}\n" +
	"      cancel-in-progress: ${{ github.ref != 'refs/heads/master' }}\n"

func workflowTail() string {
	return "jobs:\n  build:\n    runs-on: ubuntu-latest\n" + workflowGate + "    steps:\n      - run: echo hi\n"
}
