package goembed

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurveyScratch(t *testing.T) {
	root := os.Getenv("SURVEY_ROOT")
	if root == "" {
		t.Skip()
	}
	var b strings.Builder
	byLang := map[string]int{}
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		src, _ := os.ReadFile(p)
		for _, f := range scan(string(src)) {
			byLang[f.lang.Name]++
			first := strings.SplitN(strings.TrimSpace(f.value), "\n", 2)[0]
			fmt.Fprintf(&b, "%s:%d %s fn=%s spec=%v | %.70q\n", p, f.line, f.lang.Name, f.fn, f.spec != nil, first)
		}
		return nil
	})
	fmt.Fprintln(&b, byLang)
	os.WriteFile(os.Getenv("SURVEY_OUT"), []byte(b.String()), 0o644)
}
