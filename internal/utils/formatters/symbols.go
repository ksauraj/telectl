package formatters

// The symbol vocabulary. Every glyph the bot renders — in message bodies, in
// tables and on inline buttons — comes from here.
//
// These are text-presentation Unicode symbols, not emoji, and the distinction
// is load-bearing rather than cosmetic:
//
//   - Emoji are emoji-presentation by default, so a client renders them from a
//     colour font at roughly two columns. The fixed-width tables in
//     formatters.go align by counting columns (displayWidth), and every glyph
//     below is exactly one column, which is what the surrounding ASCII already
//     assumes.
//   - Mixing the two vocabularies in one interface is what this replaces. A
//     button reading "📦 Pods" next to one reading "▸ Describe" has no rule
//     behind it; a reader cannot tell what a marker means because it means
//     nothing consistent.
//
// The rule: a marker encodes state or direction. Decoration gets no glyph.
// Anything not covered here should be plain text rather than a new symbol.
const (
	// Status glyphs, keyed to the lifecycle a Kubernetes object is in rather
	// than to a specific phase string, so kinds with different vocabularies
	// ("Running", "Active", "Bound") render alike when they mean alike.
	GlyphHealthy  = "●" // running, active, ready, bound, available
	GlyphDone     = "✓" // succeeded, completed — finished, not currently doing work
	GlyphWaiting  = "○" // pending, creating, progressing — not there yet
	GlyphBroken   = "✗" // failed, crashlooping, errored
	GlyphStopping = "◌" // terminating — going away
	GlyphUnknown  = "?" // the object reports its state as literally Unknown
	GlyphNeutral  = "·" // no opinion: a status string this table does not model

	// Action markers.
	GlyphAction      = "▸" // a verb or a drill-down: this button does something
	GlyphDestructive = "!" // deletes, evicts, or otherwise cannot be undone
	GlyphSelected    = "✓" // the option currently in effect
	GlyphCancel      = "✗" // dismiss without acting
	GlyphRefresh     = "↻" // re-read and re-render the same view

	// Navigation markers, distinguished by direction so a keyboard's shape is
	// readable without reading the labels.
	GlyphBack = "«" // up one level, back to where this was opened from
	GlyphPrev = "‹" // previous page of the same list
	GlyphNext = "›" // next page of the same list
	GlyphList = "↑" // back out to the list this item came from
	GlyphHome = "⌂" // the main menu
)

// Btn prefixes a button label with a marker, which is the only place button
// text is assembled. Going through one function is what keeps a stray emoji
// from reappearing in a keyboard builder later.
func Btn(glyph, label string) string {
	if glyph == "" {
		return label
	}
	return glyph + " " + label
}
