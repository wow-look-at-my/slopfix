package expect

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// note annotates a line. The marker is joined at run time, so this file carries no annotation of its own.
func note(want string) string { return " // " + Marker + " " + strconv.Quote(want) }

func TestParseStripsTheAnnotationAndItsOpener(t *testing.T) {
	a := Parse("keep\nx := 1" + note("x := 2") + "\n")
	require.Empty(t, a.Errors)
	assert.Equal(t, "keep\nx := 1\n", a.Stripped)
	assert.Equal(t, map[int]string{1: "x := 2"}, a.Want)
}

func TestAMarkerWithNoOpenerIsText(t *testing.T) {
	a := Parse(`const m = "` + Marker + `"` + "\n")
	assert.False(t, a.Any())
}

func TestAnUnquotedValueIsAnError(t *testing.T) {
	a := Parse("x // " + Marker + " bare\n")
	require.Len(t, a.Errors, 1)
	assert.Contains(t, a.Errors[0], "Go-quoted")
}

func TestABlockCommentAnnotationDropsItsCloser(t *testing.T) {
	a := Parse("x <!-- " + Marker + ` "y" -->` + "\n")
	require.Empty(t, a.Errors)
	assert.Equal(t, "x\n", a.Stripped)
	assert.Equal(t, "y", a.Want[0])
}

func TestShiftMovesPastRemovedAnnotations(t *testing.T) {
	text := "ab" + note("c") + "\nde\n"
	a := Parse(text)
	at := strings.Index(text, "de")
	assert.Equal(t, strings.Index(a.Stripped, "de"), a.Shift(at))
	assert.Equal(t, 1, a.Shift(1))
}

func TestCheckHolds(t *testing.T) {
	a := Parse("one" + note("ONE") + "\ntwo" + note("") + "\nthree\n")
	assert.Empty(t, a.Check("ONE\nthree\n"))
}

func TestCheckNamesAMismatch(t *testing.T) {
	a := Parse("one" + note("ONE") + "\nthree\n")
	got := a.Check("uno\nthree\n")
	require.Len(t, got, 1)
	assert.Contains(t, got[0], `slopfix wrote "uno"`)
}

func TestCheckNamesAnAnnotatedLineLeftAlone(t *testing.T) {
	a := Parse("one" + note("ONE") + "\n")
	got := a.Check("one\n")
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "made no change")
}

func TestCheckNamesAnUnannotatedChange(t *testing.T) {
	a := Parse("one" + note("ONE") + "\ntwo\n")
	got := a.Check("ONE\nTWO\n")
	require.Len(t, got, 1)
	assert.Contains(t, got[0], "line 2")
	assert.Contains(t, got[0], "carries no")
}
