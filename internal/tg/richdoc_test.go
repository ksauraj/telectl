package tg

import (
	"strings"
	"testing"
)

func TestRichTableIsGFM(t *testing.T) {
	d := NewRichDoc()
	d.Heading(3, "Pods — 2 item(s)")
	d.Table(
		[]string{"NAME", "STATUS"},
		[][]string{
			{"nginx-1", "🟢 Running"},
			{"redis-1", "🔴 CrashLoopBackOff"},
		},
		TableOpts{Align: []string{"left", "center"}},
	)
	got := d.String()

	for _, want := range []string{
		"### Pods",
		"| NAME | STATUS |",
		"| nginx-1 | 🟢 Running |",
		":---:", // center alignment marker
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	// Header, separator and both data rows.
	if n := strings.Count(got, "\n|"); n < 3 {
		t.Errorf("expected at least 3 table lines, got:\n%s", got)
	}
}

// A cell containing "|" must not break the table into extra columns.
func TestRichTableEscapesPipes(t *testing.T) {
	d := NewRichDoc()
	d.Table([]string{"NAME", "VALUE"}, [][]string{
		{"weird|name", "a|b"},
	}, TableOpts{})
	got := d.String()

	if strings.Contains(got, "weird|name") {
		t.Errorf("raw pipe leaked into a cell, table will be malformed:\n%s", got)
	}
	if !strings.Contains(got, `weird\|name`) {
		t.Errorf("expected escaped pipe, got:\n%s", got)
	}
	// The data row must have exactly the 2 declared columns: 3 delimiters.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "weird") {
			if n := strings.Count(line, "|") - strings.Count(line, `\|`); n != 3 {
				t.Errorf("row has %d unescaped delimiters, want 3: %q", n, line)
			}
		}
	}
}

// Kubernetes labels and event messages routinely contain Markdown control
// characters; unescaped, they would corrupt the rendering.
func TestRichEscapesMarkdownControlChars(t *testing.T) {
	d := NewRichDoc()
	d.Paragraph("value with *bold* and _under_ and [link] and `code`")
	got := d.String()

	for _, raw := range []string{"*bold*", "_under_", "[link]", "`code`"} {
		if strings.Contains(got, raw) {
			t.Errorf("unescaped %q in:\n%s", raw, got)
		}
	}
}

// Newlines inside a cell would split the row across table lines.
func TestRichTableFlattensNewlines(t *testing.T) {
	d := NewRichDoc()
	d.Table([]string{"A"}, [][]string{{"line1\nline2"}}, TableOpts{})
	got := d.String()

	// Index returns -1 when absent, which would panic the slice below. Fail
	// with a readable message instead.
	i := strings.Index(got, "line1")
	if i < 0 {
		t.Fatalf("table cell content missing entirely:\n%q", got)
	}
	body := got[i:]
	if strings.Contains(body[:len("line1 line2")], "\n") {
		t.Errorf("newline survived inside a cell:\n%q", got)
	}
}

// Short rows must not shift columns; missing cells are padded.
func TestRichTablePadsShortRows(t *testing.T) {
	d := NewRichDoc()
	d.Table([]string{"A", "B", "C"}, [][]string{{"only-one"}}, TableOpts{})
	got := d.String()

	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "only-one") {
			if n := strings.Count(line, "|"); n != 4 {
				t.Errorf("padded row should have 4 delimiters for 3 columns, got %d: %q", n, line)
			}
		}
	}
}

func TestRichCodeBlockCannotBreakOut(t *testing.T) {
	d := NewRichDoc()
	d.Code("log", "normal line\n``` \nescaped?")
	got := d.String()

	// Exactly one opening and one closing fence.
	if n := strings.Count(got, "```"); n != 2 {
		t.Errorf("expected exactly 2 fences, got %d:\n%s", n, got)
	}
}

func TestRichDetailsWrapsBody(t *testing.T) {
	d := NewRichDoc()
	d.Details("Labels (2)", "| K | V |\n|:---|:---|\n| a | b |")
	got := d.String()

	for _, want := range []string{"<details>", "<summary>Labels", "</details>", "| a | b |"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRichDocBlockSeparation(t *testing.T) {
	d := NewRichDoc()
	d.Heading(1, "Title").Paragraph("Body").Divider().Paragraph("After")
	got := d.String()

	if !strings.Contains(got, "# Title\n\nBody") {
		t.Errorf("blocks must be separated by a blank line:\n%s", got)
	}
	if !strings.Contains(got, "---") {
		t.Errorf("missing divider:\n%s", got)
	}
	if strings.HasPrefix(got, "\n") || strings.HasSuffix(got, "\n") {
		t.Errorf("document should be trimmed:\n%q", got)
	}
}

func TestRichHeadingLevelClamped(t *testing.T) {
	if got := NewRichDoc().Heading(0, "x").String(); !strings.HasPrefix(got, "# ") {
		t.Errorf("level 0 should clamp to 1, got %q", got)
	}
	if got := NewRichDoc().Heading(99, "x").String(); !strings.HasPrefix(got, "###### ") {
		t.Errorf("level 99 should clamp to 6, got %q", got)
	}
}
