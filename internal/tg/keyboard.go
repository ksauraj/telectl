package tg

import (
	"fmt"
	"strings"
)

func NewBotCommand(command, description string) BotCommand {
	return BotCommand{Command: command, Description: description}
}

func NewReplyKeyboard(rows ...[]KeyboardButton) ReplyKeyboardMarkup {
	return ReplyKeyboardMarkup{
		Keyboard:              rows,
		ResizeKeyboard:        true,
		OneTimeKeyboard:       false,
		IsPersistent:          false,
		InputFieldPlaceholder: "",
		Selective:             false,
	}
}

func NewKeyboardButtonRow(buttons ...KeyboardButton) []KeyboardButton {
	return buttons
}

func NewKeyboardButton(text string) KeyboardButton {
	return KeyboardButton{Text: text}
}

func NewInlineKeyboardMarkup(rows ...[]InlineKeyboardButton) InlineKeyboardMarkup {
	return InlineKeyboardMarkup{InlineKeyboard: rows}
}

func NewInlineKeyboardRow(buttons ...InlineKeyboardButton) []InlineKeyboardButton {
	return buttons
}

func NewInlineKeyboardButtonData(text, data string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, CallbackData: data}
}

func NewInlineKeyboardButtonURL(text, url string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, URL: url}
}

func KeyboardButtonText(text string) KeyboardButton {
	return KeyboardButton{Text: text}
}

// Backward-compat aliases matching old tgbotapi patterns
func ReplyKeyboard(rows ...[]KeyboardButton) ReplyKeyboardMarkup {
	return NewReplyKeyboard(rows...)
}

func KeyboardButtonRow(buttons ...KeyboardButton) []KeyboardButton {
	return NewKeyboardButtonRow(buttons...)
}

func InlineKeyboard(rows ...[]InlineKeyboardButton) InlineKeyboardMarkup {
	return NewInlineKeyboardMarkup(rows...)
}

func InlineKeyboardRow(buttons ...InlineKeyboardButton) []InlineKeyboardButton {
	return NewInlineKeyboardRow(buttons...)
}

func InlineKeyboardButtonData(text, data string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, CallbackData: data}
}

func InlineKeyboardButtonURL(text, url string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, URL: url}
}

// Alias for code using InlineButtonData name - returns InlineKeyboardButton
func InlineButtonData(text, data string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, CallbackData: data}
}

func InlineButtonURL(text, url string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, URL: url}
}

func TableHeader(cells ...string) []RichBlockTableCell {
	out := make([]RichBlockTableCell, len(cells))
	for i, c := range cells {
		out[i] = RichBlockTableCell{Text: c, IsHeader: true, Align: "center", Valign: "middle"}
	}
	return out
}

func TableRow(cells ...string) []RichBlockTableCell {
	out := make([]RichBlockTableCell, len(cells))
	for i, c := range cells {
		out[i] = RichBlockTableCell{Text: c, Align: "left", Valign: "middle"}
	}
	return out
}

func TableRowStyled(cells ...RichBlockTableCell) []RichBlockTableCell {
	return cells
}

func Bold(s string) RichBlockTableCell {
	return RichBlockTableCell{Text: s, Entities: []MessageEntity{{Type: "bold", Offset: 0, Length: len(s)}}}
}

func Code(s string) RichBlockTableCell {
	return RichBlockTableCell{Text: s, Entities: []MessageEntity{{Type: "code", Offset: 0, Length: len(s)}}}
}

func HeaderCell(s string) RichBlockTableCell {
	return RichBlockTableCell{Text: s, IsHeader: true, Align: "center", Valign: "middle", Entities: []MessageEntity{{Type: "bold", Offset: 0, Length: len(s)}}}
}

type TableModel struct {
	Kind     string
	Headers  []string
	Rows     [][]RichBlockTableCell
	Total    int
	Page     int
	PageSize int
}

func NewTableModel(kind string, headers []string) *TableModel {
	return &TableModel{Kind: kind, Headers: headers}
}

func (m *TableModel) AddRow(cells []RichBlockTableCell) {
	m.Rows = append(m.Rows, cells)
}

func (m *TableModel) ToRichBlocks() []RichBlock {
	return []RichBlock{
		{Type: RichBlockParagraph, Text: fmt.Sprintf("%s %d item(s)", m.Kind, len(m.Rows))},
		{
			Type:       RichBlockTable,
			Cells:      append([][]RichBlockTableCell{TableHeader(m.Headers...)}, m.Rows...),
			IsBordered: true,
			IsStriped:  true,
		},
	}
}

// Row builders using InlineKeyboardButton (for menus)
func PaginationRow(prefix string, page, totalPages int) []InlineKeyboardButton {
	var row []InlineKeyboardButton
	if page > 1 {
		row = append(row, InlineButtonData("◀ Prev", fmt.Sprintf("%s:page:%d", prefix, page-1)))
	}
	row = append(row, InlineButtonData(fmt.Sprintf("%d/%d", page, totalPages), "noop"))
	if page < totalPages {
		row = append(row, InlineButtonData("Next ▶", fmt.Sprintf("%s:page:%d", prefix, page+1)))
	}
	return row
}

func FilterRow(prefix, currentNS string, allNS []string) []InlineKeyboardButton {
	var row []InlineKeyboardButton
	if len(allNS) > 1 {
		row = append(row, InlineButtonData("NS: "+currentNS, fmt.Sprintf("%s:filter_ns", prefix)))
	}
	row = append(row, InlineButtonData("🔄 Refresh", fmt.Sprintf("%s:refresh", prefix)))
	return row
}

func DetailRow(prefix, kind, name string) []InlineKeyboardButton {
	return []InlineKeyboardButton{
		InlineButtonData("📋 Describe", fmt.Sprintf("%s:describe:%s:%s", prefix, kind, name)),
		InlineButtonData("📦 Logs", fmt.Sprintf("%s:logs:%s:%s", prefix, kind, name)),
		InlineButtonData("🗑 Delete", fmt.Sprintf("%s:delete:%s:%s", prefix, kind, name)),
		InlineButtonData("⬆️ Up", fmt.Sprintf("%s:back", prefix)),
	}
}

func EscapeMDV2(s string) string {
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, c := range special {
		s = strings.ReplaceAll(s, c, "\\"+c)
	}
	return s
}
