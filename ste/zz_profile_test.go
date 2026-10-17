package ste

import (
	"os"
	"runtime/pprof"
	"testing"
)

func TestZZProfileRunOn(t *testing.T) {
	path := os.Getenv("SLOPFIX_RUNON_PROFILE")
	if path == "" {
		t.Skip("no profile path")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		t.Fatal(err)
	}
	fixSentenceCap(runOn(4000), capSpec{reorder: true, cap: SentenceWordCap, every: true})
	pprof.StopCPUProfile()
}
