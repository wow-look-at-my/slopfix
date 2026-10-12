package slopfix

import (
	"os"
	"path/filepath"

	"github.com/wow-look-at-my/slopfix/commentfix"
	"github.com/wow-look-at-my/slopfix/fixer"
	"github.com/wow-look-at-my/slopfix/tombstones"
)

// IsDocument reports whether path names prose rather than source.
func IsDocument(path string) bool { return tombstones.IsDocument(path) }

// FixFile repairs a file in place under every rule.
func FixFile(path string) (Repair, error) {
	return FixFileWith(path, Request{})
}

// FixFileWith repairs a file in place under the caller's own selection, and
// reports what it did.
//
// The Content and Path of req are the file's, whatever the caller put there.
// Everything else is the caller's. A run that names a rule on the command line
// has to reach the repair, or the selection is silently ignored.
func FixFileWith(path string, req Request) (Repair, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Repair{}, err
	}
	req.Content, req.Path = string(content), path
	repair := Fix(req)
	if len(repair.Unmet) > 0 {
		return repair, &UnmetError{Path: path, Unmet: repair.Unmet}
	}
	if !repair.Changed {
		return repair, nil
	}
	if err := writeCreated(repair.Created); err != nil {
		return repair, err
	}
	return repair, commentfix.WriteFile(path, repair.Text)
}

// writeCreated writes each new file a repair depends on, with its directories.
func writeCreated(created []fixer.Created) error {
	for _, c := range created {
		if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
			return err
		}
		if err := commentfix.WriteFile(c.Path, c.Text); err != nil {
			return err
		}
	}
	return nil
}
