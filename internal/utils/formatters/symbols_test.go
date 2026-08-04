package formatters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// One symbol vocabulary, enforced.
//
// The interface used to mix emoji and text symbols with no rule behind which
// appeared where, so a reader could not learn what a marker meant. These tests
// keep that from coming back by construction rather than by review.

// emojiRanges are the codepoints whose *default* presentation is emoji — the
// Unicode Emoji_Presentation=Yes set.
//
// The distinction is the whole point, so the set has to be precise rather than
// "everything non-ASCII". U+2713 CHECK MARK and U+2717 BALLOT X sit inside the
// dingbats block but are text-presentation, which is exactly why the vocabulary
// uses them; U+2705 WHITE HEAVY CHECK MARK sits in the same block and is emoji,
// so it is banned. Banning the whole block would outlaw the replacement along
// with the thing being replaced.
//
// Below U+1F000 the emoji-presentation codepoints are scattered, so they are
// enumerated. Above it, the blocks are emoji wholesale.
var emojiRanges = []struct{ lo, hi rune }{
	{0x1F000, 0x1FAFF}, // emoticons, pictographs, transport, symbols & flags
	{0x1F3FB, 0x1F3FF}, // skin tone modifiers
	{0xFE0F, 0xFE0F},   // VARIATION SELECTOR-16: forces emoji presentation

	// Emoji_Presentation=Yes below U+1F000.
	{0x231A, 0x231B}, {0x23E9, 0x23EC}, {0x23F0, 0x23F0}, {0x23F3, 0x23F3},
	{0x25FD, 0x25FE}, {0x2614, 0x2615}, {0x2648, 0x2653}, {0x267F, 0x267F},
	{0x2693, 0x2693}, {0x26A1, 0x26A1}, {0x26AA, 0x26AB}, {0x26BD, 0x26BE},
	{0x26C4, 0x26C5}, {0x26CE, 0x26CE}, {0x26D4, 0x26D4}, {0x26EA, 0x26EA},
	{0x26F2, 0x26F3}, {0x26F5, 0x26F5}, {0x26FA, 0x26FA}, {0x26FD, 0x26FD},
	{0x2705, 0x2705}, {0x270A, 0x270B}, {0x2728, 0x2728}, {0x274C, 0x274C},
	{0x274E, 0x274E}, {0x2753, 0x2755}, {0x2757, 0x2757}, {0x2795, 0x2797},
	{0x27B0, 0x27B0}, {0x27BF, 0x27BF}, {0x2B1B, 0x2B1C}, {0x2B50, 0x2B50},
	{0x2B55, 0x2B55},
}

func isEmoji(r rune) bool {
	for _, rng := range emojiRanges {
		if r >= rng.lo && r <= rng.hi {
			return true
		}
	}
	return false
}

// No Go source outside this file's own range table may contain an emoji.
//
// This is the test that would have caught the mixture in the first place. It
// walks the tree rather than taking a list of files, so a new package is covered
// the moment it is added.
func TestNoEmojiInSource(t *testing.T) {
	root := "../../.."

	// symbols.go documents the rule with an example, and this file declares the
	// ranges; both legitimately contain the characters they are about.
	exempt := map[string]bool{
		"symbols.go":      true,
		"symbols_test.go": true,
	}

	// A line carrying this marker is allowed an emoji, because the emoji is what
	// that line is testing. displayWidth still has to measure emoji correctly —
	// the bot no longer emits any, but labels, annotations and event messages
	// come from the cluster and are free-form — so the tests covering it need
	// real emoji as input.
	//
	// Marked per line rather than per file: a file-level exemption would also
	// wave through an emoji added to that file later for no reason, which is the
	// drift this test exists to catch.
	const allowMarker = "emoji-ok"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Vendored and build output is not ours to police.
			switch info.Name() {
			case ".git", "vendor", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || exempt[info.Name()] {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for lineNo, line := range strings.Split(string(src), "\n") {
			if strings.Contains(line, allowMarker) {
				continue
			}
			for _, r := range line {
				if isEmoji(r) {
					t.Errorf("%s:%d contains emoji %q (%U) — the interface uses the "+
						"text symbols in symbols.go, and mixing the two vocabularies "+
						"is what this replaced:\n\t%s",
						path, lineNo+1, r, r, strings.TrimSpace(line))
					break // one report per line is enough
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// Every glyph must be exactly one display column.
//
// The fixed-width tables in formatters.go align by counting columns, and the
// surrounding ASCII assumes one column per character. A two-column glyph in a
// status cell shifts every following cell in that row, which is the alignment
// bug emoji caused in the first place.
func TestGlyphsAreSingleWidth(t *testing.T) {
	glyphs := map[string]string{
		"GlyphHealthy":     GlyphHealthy,
		"GlyphDone":        GlyphDone,
		"GlyphWaiting":     GlyphWaiting,
		"GlyphBroken":      GlyphBroken,
		"GlyphStopping":    GlyphStopping,
		"GlyphUnknown":     GlyphUnknown,
		"GlyphNeutral":     GlyphNeutral,
		"GlyphAction":      GlyphAction,
		"GlyphDestructive": GlyphDestructive,
		"GlyphSelected":    GlyphSelected,
		"GlyphCancel":      GlyphCancel,
		"GlyphRefresh":     GlyphRefresh,
		"GlyphBack":        GlyphBack,
		"GlyphPrev":        GlyphPrev,
		"GlyphNext":        GlyphNext,
		"GlyphList":        GlyphList,
		"GlyphHome":        GlyphHome,
	}

	for name, g := range glyphs {
		if g == "" {
			t.Errorf("%s is empty; a missing marker shifts every table column", name)
			continue
		}
		if !utf8.ValidString(g) {
			t.Errorf("%s is not valid UTF-8; Telegram rejects the whole message", name)
		}
		if n := utf8.RuneCountInString(g); n != 1 {
			t.Errorf("%s is %d runes, want 1 — multi-rune markers (emoji with a "+
				"variation selector) are what broke column alignment", name, n)
		}
		if w := displayWidth(g); w != 1 {
			t.Errorf("%s has display width %d, want 1: %q", name, w, g)
		}
		for _, r := range g {
			if isEmoji(r) {
				t.Errorf("%s is an emoji (%U), not a text symbol", name, r)
			}
		}
	}
}

// The status glyphs must be distinguishable from one another. Two states sharing
// a glyph is worse than no glyph: it reads as information and conveys none.
func TestStatusGlyphsAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, tc := range []struct{ name, glyph string }{
		{"healthy", GlyphHealthy},
		{"done", GlyphDone},
		{"waiting", GlyphWaiting},
		{"broken", GlyphBroken},
		{"stopping", GlyphStopping},
		{"unknown", GlyphUnknown},
		{"neutral", GlyphNeutral},
	} {
		if prev, dup := seen[tc.glyph]; dup {
			t.Errorf("%s and %s both render as %q; the two states are indistinguishable",
				prev, tc.name, tc.glyph)
		}
		seen[tc.glyph] = tc.name
	}
}

func TestBtn(t *testing.T) {
	if got := Btn(GlyphAction, "Pods"); got != GlyphAction+" Pods" {
		t.Errorf("Btn = %q", got)
	}
	// No glyph means no leading space, or every such label is indented by one.
	if got := Btn("", "Pods"); got != "Pods" {
		t.Errorf("Btn with empty glyph = %q, want %q", got, "Pods")
	}
}
