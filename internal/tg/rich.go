package tg

type InputRichMessage struct {
	Blocks []RichBlock
}

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
