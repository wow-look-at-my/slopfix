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

func repairIDs(r slopfix.Repair) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.ID)
	}
	for _, k := range r.Kept {
		out = append(out, k.ID)
	}
	return out
}

// A fork that rewords a line inside a run the base already had at that length
// did not make the run long. The finding is the base's, and the fork's words stay.
func TestAForkEditInsideAnUpstreamCommentRunKeepsItsWords(t *testing.T) {
	path := ".github/workflows/pr-test.yml"
	owned := forkscope.Changed(upstreamWorkflow, forkWorkflow)

	before := slopfix.Report(slopfix.Request{Path: path, Content: forkWorkflow, Owned: owned})
	assert.Zero(t, ofID(repairIDs(before), workflow.IDCommentBlock), "the base had the run at this length")

	repair := slopfix.Fix(slopfix.Request{Path: path, Content: forkWorkflow, Owned: owned})
	assert.Empty(t, repair.Findings)
	assert.Equal(t, forkWorkflow, repair.Text, "the fork's line stays as the fork wrote it")
}

const upstreamStage = `name: PR Test Stage (CPU)
on:
  workflow_call:
jobs:
  run:
    runs-on: ubuntu-latest
    steps:
      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.10'

      # This stage compiled the workspace too - 7+ minutes per partition on
      # billable hosted minutes. rust-ext-build's modules need an older glibc than
      # this runner has, which is the safe direction, and both pin Python 3.10.
      - uses: ./.github/actions/download-rust-ext
`

var forkStage = strings.ReplaceAll(upstreamStage, "3.10", "3.12")

// A repair never writes the base's fact over the fork's: the fork's Python
// version stays in the comment, and fix reports nothing on the run.
func TestAForkFactInAnUpstreamCommentRunIsNeverReverted(t *testing.T) {
	path := ".github/workflows/_pr-test-stage-cpu.yml"
	owned := forkscope.Changed(upstreamStage, forkStage)
	repair := slopfix.Fix(slopfix.Request{Path: path, Content: forkStage, Owned: owned})
	assert.Contains(t, repair.Text, "and both pin Python 3.12.")
	assert.NotContains(t, repair.Text, "3.10")
	assert.Zero(t, ofID(repairIDs(repair), workflow.IDCommentBlock))
	report := slopfix.Report(slopfix.Request{Path: path, Content: forkStage, Owned: owned})
	assert.Zero(t, ofID(repairIDs(report), workflow.IDCommentBlock))
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

// The base already had the run over the cap. The volume finding is the base's:
// neither the report nor the repair names it, and the fork's words stay.
func TestAForkRunTheBaseAlreadyHadOverTheCapIsTheBases(t *testing.T) {
	path := "docs/src/snippets/configs/Qwen/qwen3.8-flash-next.jsx"
	require.NotEqual(t, upstreamSnippet, forkSnippet)
	owned := forkscope.Changed(upstreamSnippet, forkSnippet)
	req := slopfix.Request{Path: path, Content: forkSnippet, Owned: owned, MaxCommentLines: tombstones.DefaultMaxCommentLines}

	before := slopfix.Report(req)
	assert.Zero(t, ofID(repairIDs(before), tombstones.IDVolume), "the base had the run over the cap, so the finding is the base's")

	repair := slopfix.Fix(req)
	assert.Zero(t, ofID(repairIDs(repair), tombstones.IDVolume), "fix left a volume finding the fork's check reports")
	flat := strings.Join(strings.Fields(strings.ReplaceAll(repair.Text, "//", " ")), " ")
	assert.Contains(t, flat, "The scheduler caps --max-running-requests to what the mamba pool admits.", "the fork's words stay")
	assert.NotContains(t, flat, "silently", "the base's wording never comes back")
}

// upstreamInnerDoc is the parent's module doc, written with Rust's inner doc marker.
var upstreamInnerDoc = "//! `/debug <what is wrong>` hands the model this process's context.\n" +
	strings.Repeat("//! The model reads the config layers and the model id.\n", 10) +
	"\nuse std::path::Path;\n"

// forkInnerDoc adds lines to that doc until the run is past the volume cap.
var forkInnerDoc = strings.Replace(upstreamInnerDoc, "//! The model reads",
	"//! - The debug-log file the firehose writes for this session. `/debug` turns\n"+
		"//!   the firehose on first, in this process and in the agent process.\n"+
		"//!   It is created if it does not exist.\n"+
		"//! - Whether the firehose ran since launch or only since this `/debug`.\n"+
		"//! - The rest of the execution context, assembled by the context module.\n"+
		"//! The model reads", 1)

// A fold joins the fork's added lines onto one line. A `//!` line gives up its whole marker, so no `!` lands in the prose.
func TestAForkFoldOfAnInnerDocDropsTheWholeMarker(t *testing.T) {
	path := "src/debug.rs"
	req := slopfix.Request{Path: path, Content: forkInnerDoc, Owned: forkscope.Changed(upstreamInnerDoc, forkInnerDoc), MaxCommentLines: tombstones.DefaultMaxCommentLines}
	repair := slopfix.Fix(req)
	for _, line := range strings.Split(repair.Text, "\n") {
		prose, ok := strings.CutPrefix(line, "//!")
		if !ok {
			continue
		}
		assert.NotContains(t, prose, " ! ", "a joined line kept the `!` of a `//!` marker: %q", line)
		assert.NotContains(t, prose, "//", "a joined line kept a marker: %q", line)
	}
	assert.Contains(t, repair.Text, "`/debug` turns the firehose on first", "the fork's words are joined, not lost")
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
	assert.Equal(t, []int{5, 25}, lines)
}

// Grown names a change inside the run only when it added lines.
func TestGrownNamesOnlyAChangeThatAddedLines(t *testing.T) {
	assert.Empty(t, forkscope.Grown(upstreamWorkflow, forkWorkflow, 5, 8), "a reworded line adds none")
	hunks := forkscope.Grown(upstreamSnippet, forkSnippet, 5, 20)
	require.Len(t, hunks, 1)
	assert.Equal(t, 2, hunks[0].Had)
	assert.Equal(t, 3, hunks[0].J2-hunks[0].J1)
	assert.Empty(t, forkscope.Grown(upstreamSnippet, forkSnippet, 25, 40), "a run that holds no change of the fork's grew by none")
}
