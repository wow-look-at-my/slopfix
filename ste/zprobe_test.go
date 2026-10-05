package ste

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

const probeDir = "/mnt/cc-tmpfs/claude-0/-home-user/86dd7738-c2be-5e9a-a8f0-a7d5d5de3580/scratchpad/r6/"

func TestZProbeCorpus(t *testing.T) {
	var b strings.Builder
	over := 0
	total := 0
	for _, name := range []string{"long.txt", "selflong.txt"} {
		data, err := os.ReadFile(probeDir + name)
		if err != nil {
			t.Skip()
		}
		for _, in := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			total++
			out := FixSelected(in, func(id string) bool {
				return id == IDSentenceCap || id == IDSemicolon || id == IDCommaSplice
			})
			flag := "OK  "
			if overCap(out) > 0 {
				flag = "OVER"
				over++
			}
			fmt.Fprintf(&b, "%s IN:  %s\n     OUT: %s\n", flag, in, out)
		}
	}
	fmt.Fprintf(&b, "\n%d of %d still over\n", over, total)
	_ = os.WriteFile(probeDir+"probe-out.txt", []byte(b.String()), 0o644)
}
