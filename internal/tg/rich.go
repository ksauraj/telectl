package tg

// The Bot API models a *received* rich message as a tree of blocks
// (models.RichMessage). Sending uses models.InputRichMessage, which takes a
// markup string — see richdoc.go for the builder and bot.go for the send path.
//
// These types are retained for parsing inbound rich messages. Nothing in the
// send path uses them.

type RichBlockType string

const (
	RichBlockParagraph  RichBlockType = "paragraph"
	RichBlockTable      RichBlockType = "table"
	RichBlockHeading    RichBlockType = "heading"
	RichBlockPre        RichBlockType = "pre"
	RichBlockDivider    RichBlockType = "divider"
	RichBlockBlockquote RichBlockType = "blockquote"
)

type RichBlock struct {
	Type       RichBlockType
	Text       string
	Size       int
	Language   string
	Cells      [][]RichBlockTableCell
	IsBordered bool
	IsStriped  bool
}

type RichBlockTableCell struct {
	Text     string
	IsHeader bool
	Align    string
	Valign   string
	Colspan  int
	Rowspan  int
	Entities []MessageEntity
}
