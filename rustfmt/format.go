package rustfmt

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/wow-look-at-my/slopfix/treecomments"
)

// sameComments reports whether before and after hold the same comment words, in order.
func sameComments(path, before, after string) bool {
	words := func(text string) []string {
		var out []string
		for _, c := range treecomments.Extract(path, text) {
			out = append(out, strings.Fields(c.Text)...)
		}
		return out
	}
	return slices.Equal(words(before), words(after))
}

func isRust(path string) bool {
	return strings.HasSuffix(path, ".rs")
}

// FormatFile rewrites path in place with the rustfmt output. It feeds the file
// on stdin, so rustfmt does not follow `mod` declarations into other files.
// The working directory is the file's directory, so rustfmt finds the
// project's rustfmt.toml and rust-toolchain.toml.
func FormatFile(path string) (bool, error) {
	bin, err := exec.LookPath(rustfmtBin())
	if err != nil {
		return false, errors.New("rustfmt is not on PATH. Install it with `rustup component add rustfmt`")
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	args := []string{"--emit", "stdout"}
	if ed := cargoEdition(filepath.Dir(path)); ed != "" {
		args = append(args, "--edition", ed)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = filepath.Dir(path)
	cmd.Stdin = bytes.NewReader(src)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%v\n%s", err, strings.TrimSpace(errOut.String()))
	}
	if bytes.Equal(out.Bytes(), src) {
		return false, nil
	}
	// The layout gate: rustfmt moves code, and every comment comes back with the same words in the same order.
	if !sameComments(path, string(src), out.String()) {
		return false, fmt.Errorf("rustfmt changed the comments of %s", path)
	}
	if err := os.WriteFile(path, out.Bytes(), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// rustfmtBin lets a test point the hook at a stub.
func rustfmtBin() string {
	if b := os.Getenv("SLOPFIX_RUSTFMT_BIN"); b != "" {
		return b
	}
	return "rustfmt"
}

// cargoEdition gives the edition that `cargo fmt` passes for a file in dir.
// It returns "" outside a Cargo package, and rustfmt then reads the edition
// from rustfmt.toml or uses its own default.
func cargoEdition(dir string) string {
	inherit := false
	for d := dir; ; d = filepath.Dir(d) {
		manifest := readManifest(filepath.Join(d, "Cargo.toml"))
		if manifest != nil {
			if !inherit {
				if pkg, ok := manifest["package"].(map[string]any); ok {
					switch ed := pkg["edition"].(type) {
					case string:
						return ed
					case map[string]any:
						inherit = ed["workspace"] == true
					case nil:
						return "2015"
					}
				}
			}
			if ws, ok := manifest["workspace"].(map[string]any); ok && inherit {
				if pkg, ok := ws["package"].(map[string]any); ok {
					if ed, ok := pkg["edition"].(string); ok {
						return ed
					}
				}
			}
		}
		if parent := filepath.Dir(d); parent == d {
			return ""
		}
	}
}

func readManifest(path string) map[string]any {
	var m map[string]any
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil
	}
	return m
}
