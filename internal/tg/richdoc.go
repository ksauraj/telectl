package tg

import (
	"fmt"
	"strings"
)

// Rich Messages (Bot API 10.1+) render structured content — real tables,
// headings, dividers, collapsible sections — natively in Telegram clients,
// instead of the monospace code-block tables we used to emit.
//
// Wire format note: the *receive* type models.RichMessage carries a []RichBlock
// tree, but the *send* type models.InputRichMessage takes a markup string
// (`markdown` or `html`). Rich Markdown "follows GitHub Flavored Markdown where
// possible", so tables are ordinary GFM pipe tables. This file builds that
// markup; internal/tg/bot.go posts it via sendRichMessage.

// RichDoc accumulates Rich Markdown blocks.
type RichDoc struct {
	b strings.Builder
}

func NewRichDoc() *RichDoc { return &RichDoc{} }

// Heading adds a section heading. Level is clamped to 1..6.
func (d *RichDoc) Heading(level int, text string) *RichDoc {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	d.block(strings.Repeat("#", level) + " " + escapeRichInline(text))
	return d
}

// Paragraph adds a plain text paragraph.
func (d *RichDoc) Paragraph(text string) *RichDoc {
	d.block(escapeRichInline(text))
	return d
}

// Raw adds already-formatted markup without escaping. Use only for markup this
// package generated; never for cluster-supplied strings.
func (d *RichDoc) Raw(markup string) *RichDoc {
	d.block(markup)
	return d
}

// Divider adds a horizontal rule between sections.
func (d *RichDoc) Divider() *RichDoc {
	d.block("---")
	return d
}

// Code adds a fenced code block. Content is never escaped (fences are literal),
// but any fence sequence inside is neutralised so it cannot break out.
func (d *RichDoc) Code(language, content string) *RichDoc {
	content = strings.ReplaceAll(content, "```", "'''")
	d.block("```" + language + "\n" + content + "\n```")
	return d
}

// KeyValue renders a two-column borderless table, which reads better than a
// bullet list for metadata pairs.
func (d *RichDoc) KeyValue(pairs [][2]string) *RichDoc {
	if len(pairs) == 0 {
		return d
	}
	rows := make([][]string, 0, len(pairs))
	for _, p := range pairs {
		rows = append(rows, []string{"**" + escapeRichInline(p[0]) + "**", escapeRichInline(p[1])})
	}
	return d.Table([]string{"Field", "Value"}, rows, TableOpts{Align: []string{"left", "left"}})
}

// List adds a bulleted list.
func (d *RichDoc) List(items []string) *RichDoc {
	if len(items) == 0 {
		return d
	}
	var sb strings.Builder
	for i, it := range items {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("- " + escapeRichInline(it))
	}
	d.block(sb.String())
	return d
}

// Details adds a collapsible section — useful for long output (full describe
// dumps, log tails) that would otherwise dominate the chat.
func (d *RichDoc) Details(summary, body string) *RichDoc {
	d.block(fmt.Sprintf("<details>\n<summary>%s</summary>\n\n%s\n</details>",
		escapeRichInline(summary), body))
	return d
}

// TableOpts controls optional GFM table features.
type TableOpts struct {
	// Align holds "left", "center" or "right" per column. Missing or unknown
	// values fall back to left alignment.
	Align []string
	// Caption renders above the table as a bold line when set.
	Caption string
}

// Table renders a GFM pipe table, which Telegram displays as a native table.
// Cell contents are escaped so a value containing "|" cannot break the layout.
func (d *RichDoc) Table(headers []string, rows [][]string, opts TableOpts) *RichDoc {
	if len(headers) == 0 {
		return d
	}

	var sb strings.Builder
	if opts.Caption != "" {
		sb.WriteString("**" + escapeRichInline(opts.Caption) + "**\n\n")
	}

	sb.WriteString("| ")
	for i, h := range headers {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(escapeTableCell(h))
	}
	sb.WriteString(" |\n|")
	for i := range headers {
		sb.WriteString(alignMarker(opts.Align, i))
		if i < len(headers)-1 {
			sb.WriteString("|")
		}
	}
	sb.WriteString("|\n")

	for _, row := range rows {
		sb.WriteString("| ")
		for i := range headers {
			if i > 0 {
				sb.WriteString(" | ")
			}
			var cell string
			if i < len(row) {
				cell = row[i]
			}
			sb.WriteString(escapeTableCell(cell))
		}
		sb.WriteString(" |\n")
	}

	d.block(strings.TrimRight(sb.String(), "\n"))
	return d
}

func alignMarker(align []string, i int) string {
	mode := "left"
	if i < len(align) && align[i] != "" {
		mode = align[i]
	}
	switch mode {
	case "center":
		return ":---:"
	case "right":
		return "---:"
	default:
		return ":---"
	}
}

// String returns the assembled Rich Markdown document.
func (d *RichDoc) String() string {
	return strings.TrimSpace(d.b.String())
}

// IsEmpty reports whether nothing has been added.
func (d *RichDoc) IsEmpty() bool { return d.b.Len() == 0 }

func (d *RichDoc) block(s string) {
	if d.b.Len() > 0 {
		d.b.WriteString("\n\n")
	}
	d.b.WriteString(s)
}

// escapeRichInline neutralises Markdown control characters in cluster-supplied
// text. Kubernetes names are DNS labels, but labels, annotations and event
// messages are free-form and routinely contain *, _, [ and backticks.
func escapeRichInline(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		"*", `\*`,
		"_", `\_`,
		"[", `\[`,
		"]", `\]`,
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

// escapeTableCell additionally escapes "|" and flattens newlines, either of
// which would corrupt a GFM table row.
func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = escapeRichInline(s)
	return strings.ReplaceAll(s, "|", `\|`)
}
