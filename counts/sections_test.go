package counts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const numbered = "# ADR\n\n## 1. The question\n\nText.\n\n## 9. Owning the renderer\n\nMore.\n\n### 9.2 The cap\n\nLast.\n"

func links(t *testing.T, path, doc string) []string {
	t.Helper()
	var out []string
	for _, hit := range CheckSections(path, doc) {
		out = append(out, hit.Number+" "+hit.Link)
	}
	return out
}

func TestASectionNumberBecomesALinkNamedByItsTitle(t *testing.T) {
	doc := numbered + "\nThe §9 trigger fires. §9.2 holds it, and §1 asks it.\n"
	assert.Equal(t, []string{
		"§9 [§owning-renderer](#9-owning-the-renderer)",
		"§9.2 [§cap](#92-the-cap)",
		"§1 [§question](#1-the-question)",
	}, links(t, "", doc))
}

// A citation nothing answers keeps its number: there is no slug to give it.
func TestACitationWithNoHeadingIsLeftAlone(t *testing.T) {
	assert.Empty(t, links(t, "", numbered+"\nSee §7 for that.\n"))
	assert.Empty(t, links(t, "", numbered+"\nThe span `§9` is code.\n"))
	assert.Empty(t, links(t, "", numbered+"\nIt is [§9](#9-owning-the-renderer) already.\n"))
}

// Sections with one title get distinct slugs, and their anchors follow GitHub.
func TestRepeatedTitlesGetUniqueSlugs(t *testing.T) {
	doc := "## 1. Notes\n\nA.\n\n## 2. Notes\n\nB.\n\nSee §1 and §2.\n"
	assert.Equal(t, []string{
		"§1 [§notes](#1-notes)",
		"§2 [§notes-2](#2-notes)",
	}, links(t, "", doc))
}

func TestACitationOfAnotherFileResolvesBesideIt(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "core.md"), []byte(numbered), 0o644))
	doc := "Read `core.md` §9 first, then §1 of [the core](core.md).\n"
	assert.Equal(t, []string{
		"§9 [§owning-renderer](core.md#9-owning-the-renderer)",
		"§1 [§question](core.md#1-the-question)",
	}, links(t, filepath.Join(dir, "doc.md"), doc))
}

// A slug keeps the first words of the title that name the section.
func TestALongTitleGetsAShortSlug(t *testing.T) {
	doc := "## 2. Is a fork actually cheap? Tested: mostly yes, with caveats\n\nA.\n\nSee §2.\n"
	assert.Equal(t, []string{"§2 [§fork-actually-cheap](#2-is-a-fork-actually-cheap-tested-mostly-yes-with-caveats)"}, links(t, "", doc))
}

// A heading renamed after the link was written gives the link its new slug.
func TestALinkTakesTheSlugItsHeadingHasNow(t *testing.T) {
	doc := numbered + "\nSee [§old-name](#9-owning-the-renderer) and [§cap](#92-the-cap).\n"
	assert.Equal(t, []string{"[§old-name](#9-owning-the-renderer) [§owning-renderer](#9-owning-the-renderer)"}, links(t, "", doc))
}

func TestASectionCitationIsNoCount(t *testing.T) {
	assert.Empty(t, Gate("The §9 trigger fires.\n"))
	assert.Empty(t, Check("This repo's §3 rules hold.\n"))
}
