package slopfix_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/slopfix"
	"github.com/wow-look-at-my/slopfix/forkscope"
	"github.com/wow-look-at-my/slopfix/tombstones"
	"github.com/wow-look-at-my/slopfix/workflow"
)

// upstreamWorkflow is the parent's workflow. Its comment run was over the
// limit before the fork touched it.
const upstreamWorkflow = `name: PR Test Base
on:
  pull_request:
concurrency:
  # - event_name prevents scheduled runs from colliding with fork PRs whose branch is named 'main'
  #   (without it, both resolve the branch segment to 'main' and block each other)
  # - a PR keys on its number: github.head_ref is a bare branch name with no owner,
  #   so two forks using the same name would share one group and cancel each other
  group: pr-test-${{ github.event_name }}-${{ github.event.pull_request.number || github.ref_name }}
  cancel-in-progress: true
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: echo ok
`

// forkWorkflow rewords one line of that run, so the run holds a line the fork wrote.
var forkWorkflow = strings.Replace(upstreamWorkflow, "so two forks using", "so forks using", 1)

func ofID(findings []string, id string) int {
	n := 0
	for _, f := range findings {
		if f == id {
			n++
		}
	}
	return n
}

func findingIDs(r slopfix.Repair) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.ID)
	}
	for _, k := range r.Kept {
		out = append(out, k.ID)
	}
	return out
}

func TestAForkCommentRunTheForkWroteInPartIsRepaired(t *testing.T) {
	path := ".github/workflows/pr-test.yml"
	owned := forkscope.Changed(upstreamWorkflow, forkWorkflow)

	before := slopfix.Report(slopfix.Request{Path: path, Content: forkWorkflow, Owned: owned})
	require.Equal(t, 1, ofID(findingIDs(before), workflow.IDCommentBlock), "the run holds a line the fork wrote, so it is the fork's finding")

	repair := slopfix.Fix(slopfix.Request{Path: path, Content: forkWorkflow, Owned: owned})
	assert.Empty(t, repair.Findings, "fix left a finding the fork's check reports")
	assert.Equal(t, upstreamWorkflow, repair.Text, "the fork's line in the run goes back to the base, and nothing else moves")

	after := slopfix.Report(slopfix.Request{Path: path, Content: repair.Text, Owned: forkscope.Changed(upstreamWorkflow, repair.Text)})
	assert.Empty(t, after.Findings)
}

// A run the fork wrote whole is the fork's to join, every word kept.
func TestAForkCommentRunTheForkWroteWholeIsJoined(t *testing.T) {
	path := ".github/workflows/pr-test.yml"
	fork := strings.Replace(upstreamWorkflow, "jobs:\n", "# Every job runs on a hosted runner,\n# and none of them needs a secret.\njobs:\n", 1)
	repair := slopfix.Fix(slopfix.Request{Path: path, Content: fork, Owned: forkscope.Changed(upstreamWorkflow, fork)})
	assert.Contains(t, repair.Text, "# Every job runs on a hosted runner, and none of them needs a secret.\njobs:\n")
	assert.Empty(t, repair.Findings)
}

// upstreamSnippet is the parent's config. Its comment run was over the volume
// cap before the fork reworded some of its lines.
const upstreamSnippet = `export const config = {
  modelName: "Qwen3.8-Flash-Next",

  deployments: [
    // ==== NVFP4 on 2x DGX Spark (GB10, sm_121) - the only multi-node shape ====
    // GB10 is a single Blackwell GPU with 128 GB unified memory per node.
    // The checkpoint does not fit one box with the N-gram table resident.
    // So it runs TP=2 across two of them over the ConnectX-7 link.
    // The cell keeps the BF16 SSM state the MTP verify path needs.
    // It also keeps the mamba scheduler strategy at extra_buffer.
    // The verify path writes intermediate states the decode path reads.
    // That needs one spare state slot per running request.
    // The hybrid model reserves mamba state slots per running request.
    // The default strategy reserves more of them than the lazy one does.
    // The scheduler silently caps --max-running-requests to what the mamba pool
    // admits unless --max-mamba-cache-size = requests x slots is set.
    // The PLE Offload row is forced to Off on this hardware.
    // The FP8 table stays GPU-resident and TP-sharded.
    // On unified memory the offloaded copy would sit in the same pool.
    {
      match: { hw: "dgx-spark", nodes: "multi-2" },
      verified: true,
    },

    // ==== NVFP4 on 1x RTX PRO 6000 Blackwell (SM120, 96 GB) ====
    // A single card serves the checkpoint once the table is offloaded.
    // The table lives in pinned host memory.
    // Keep enough host memory free for it.
    // Docker needs an unlimited memlock for the pinned copy.
    // The KV pool is small on this card.
    // Long-context work needs fewer running requests.
    // The cell is verified on the local development image.
    // The release image predates the loader this export needs.
    // The SM120 kernels come from the same build.
    // No other image carries them yet.
    // The low-latency cell runs a single request at a time.
    // The high-throughput cell runs many requests at once.
    // Both cells share the offload flags.
    // Both cells share the memlock requirement.
    // Both cells share the image requirement.
    {
      match: { hw: "rtx6000", nodes: "single" },
      verified: true,
    },
  ],
};
`

var forkSnippet = strings.Replace(upstreamSnippet,
	"    // The scheduler silently caps --max-running-requests to what the mamba pool\n    // admits unless --max-mamba-cache-size = requests x slots is set.\n",
	"    // The scheduler caps --max-running-requests to what the mamba pool admits.\n    // Without a pin that pool is sized for the requested concurrency only up\n    // to 47% of the budget, so --max-mamba-cache-size = requests x slots is set.\n", 1)

func TestAForkVolumeFindingNamesItsBlockAndIsRepaired(t *testing.T) {
	path := "docs/src/snippets/configs/Qwen/qwen3.8-flash-next.jsx"
	require.NotEqual(t, upstreamSnippet, forkSnippet)
	owned := forkscope.Changed(upstreamSnippet, forkSnippet)
	req := slopfix.Request{Path: path, Content: forkSnippet, Owned: owned, MaxCommentLines: tombstones.DefaultMaxCommentLines}

	before := slopfix.Report(req)
	var volume []int
	for _, k := range before.Kept {
		if k.ID == tombstones.IDVolume {
			volume = append(volume, k.LineNo)
		}
	}
	assert.Equal(t, []int{5}, volume, "only the block the fork wrote a line of is the fork's finding, and it names where that block starts")

	repair := slopfix.Fix(req)
	assert.Zero(t, ofID(findingIDs(repair), tombstones.IDVolume), "fix left a volume finding the fork's check reports")
	assert.Equal(t, upstreamSnippet, repair.Text, "the fork's lines in the block go back to the base, and nothing else moves")
}

// Outside a fork, every volume finding names the line its block starts on.
func TestAVolumeFindingNamesItsBlock(t *testing.T) {
	repair := slopfix.Report(slopfix.Request{Path: "config.jsx", Content: upstreamSnippet, MaxCommentLines: tombstones.DefaultMaxCommentLines})
	var lines []int
	for _, k := range repair.Kept {
		if k.ID == tombstones.IDVolume {
			lines = append(lines, k.LineNo)
			assert.Greater(t, k.EndLineNo, k.LineNo)
		}
	}
	assert.Equal(t, []int{5, 29}, lines)
}

// GiveBack writes back only the changes inside the run it is given.
func TestGiveBackWritesTheBaseLinesInsideTheRun(t *testing.T) {
	edits := forkscope.GiveBack(upstreamWorkflow, forkWorkflow, 5, 8)
	require.Len(t, edits, 1)
	assert.Equal(t, "  #   so two forks using the same name would share one group and cancel each other", edits[0].Text)
	assert.Empty(t, forkscope.GiveBack(upstreamWorkflow, forkWorkflow, 1, 4), "a run that holds no change of the fork's gives nothing back")
}
