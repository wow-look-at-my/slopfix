// Command rulegen turns the XML in rules/ into the Go the binary runs.
//
// It is the reason no rule table is parsed at start-up. The XML is build-time
// input: rulegen reads the folder, compiles every <pattern> with
// go-regex-compiler, and writes a table of literals plus a switch automaton per
// pattern. A shipped binary carries no XML, no encoding/xml and no regexp.
//
// Run it through go:generate, which is how go-toolchain drives it:
//
//	go-toolchain --generate <hash>
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/slopfix/cmd/rulegen/internal/gen"
)

func main() {
	rules := flag.String("rules", "", "the folder holding the rule XML")
	target := flag.String("for", "", "the `for` value to write, as a rules file declares it")
	pkg := flag.String("package", "", "the package name of the generated file")
	out := flag.String("out", "", "the generated table file to write")
	flag.Parse()

	if *rules == "" || *target == "" || *pkg == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "rulegen: -rules, -for, -package and -out are all required")
		os.Exit(2)
	}
	if err := gen.Run(gen.Request{
		Rules:   *rules,
		Target:  *target,
		Package: *pkg,
		Out:     *out,
		Dir:     filepath.Dir(*out),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "rulegen: %v\n", err)
		os.Exit(1)
	}
}
