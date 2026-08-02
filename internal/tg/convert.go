package tg

import (
	botmodels "github.com/go-telegram/bot/models"
)

func fromModelMessage(m *botmodels.Message) *Message {
	if m == nil {
		return nil
	}
	return &Message{
		ID:     m.ID,
		ChatID: m.Chat.ID,
		Text:   m.Text,
		From:   fromModelUser(m.From),
		Chat:   fromModelChat(&m.Chat),
	}
}

func fromModelUser(u *botmodels.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:        u.ID,
		FirstName: u.FirstName,
		Username:  u.Username,
	}
}

func fromModelChat(c *botmodels.Chat) *Chat {
	if c == nil {
		return nil
	}
	return &Chat{
		ID:    c.ID,
		Type:  string(c.Type),
		Title: c.Title,
	}
}

// toModelInlineKeyboard converts the local keyboard type to the library model.
// This matters: the local tg structs carry no JSON tags, so marshalling them
// directly produces {"InlineKeyboard":...} instead of {"inline_keyboard":...}
// and Telegram silently drops the markup. The library models have correct tags.
func toModelInlineKeyboard(kb *InlineKeyboardMarkup) *botmodels.InlineKeyboardMarkup {
	if kb == nil {
		return nil
	}
	rows := make([][]botmodels.InlineKeyboardButton, len(kb.InlineKeyboard))
	for i, row := range kb.InlineKeyboard {
		rows[i] = make([]botmodels.InlineKeyboardButton, len(row))
		for j, btn := range row {
			rows[i][j] = botmodels.InlineKeyboardButton{
				Text:                         btn.Text,
				IconCustomEmojiID:            btn.IconCustomEmojiID,
				Style:                        btn.Style,
				URL:                          btn.URL,
				CallbackData:                 btn.CallbackData,
				SwitchInlineQuery:            btn.SwitchInlineQuery,
				SwitchInlineQueryCurrentChat: btn.SwitchInlineQueryCurrentChat,
				Pay:                          btn.Pay,
			}
			if btn.WebApp != nil {
				rows[i][j].WebApp = &botmodels.WebAppInfo{URL: btn.WebApp.URL}
			}
			if btn.CopyText != nil {
				rows[i][j].CopyText = &botmodels.CopyTextButton{Text: btn.CopyText.Text}
			}
		}
	}
	return &botmodels.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// toModelReplyKeyboard converts the local reply keyboard to the library model.
func toModelReplyKeyboard(kb *ReplyKeyboardMarkup) *botmodels.ReplyKeyboardMarkup {
	if kb == nil {
		return nil
	}
	rows := make([][]botmodels.KeyboardButton, len(kb.Keyboard))
	for i, row := range kb.Keyboard {
		rows[i] = make([]botmodels.KeyboardButton, len(row))
		for j, btn := range row {
			rows[i][j] = botmodels.KeyboardButton{
				Text:            btn.Text,
				RequestContact:  btn.RequestContact,
				RequestLocation: btn.RequestLocation,
			}
			if btn.WebApp != nil {
				rows[i][j].WebApp = &botmodels.WebAppInfo{URL: btn.WebApp.URL}
			}
		}
	}
	return &botmodels.ReplyKeyboardMarkup{
		Keyboard:              rows,
		IsPersistent:          kb.IsPersistent,
		ResizeKeyboard:        kb.ResizeKeyboard,
		OneTimeKeyboard:       kb.OneTimeKeyboard,
		InputFieldPlaceholder: kb.InputFieldPlaceholder,
		Selective:             kb.Selective,
	}
}

func toModelBlocks(blocks []RichBlock) []botmodels.RichBlock {
	out := make([]botmodels.RichBlock, len(blocks))
	for i, b := range blocks {
		out[i] = botmodels.RichBlock{Type: botmodels.RichBlockType(b.Type)}
		switch b.Type {
		case RichBlockParagraph:
			rt := toRichText(b.Text, nil)
			out[i].RichBlockParagraph = &botmodels.RichBlockParagraph{Text: rt}
		case RichBlockHeading:
			rt := toRichText(b.Text, nil)
			out[i].RichBlockSectionHeading = &botmodels.RichBlockSectionHeading{
				Text: rt,
				Size: b.Size,
			}
		case RichBlockPre:
			rt := toRichText(b.Text, nil)
			out[i].RichBlockPreformatted = &botmodels.RichBlockPreformatted{
				Text:     rt,
				Language: b.Language,
			}
		case RichBlockDivider:
			out[i].RichBlockDivider = &botmodels.RichBlockDivider{}
		case RichBlockTable:
			modelCells := make([][]botmodels.RichBlockTableCell, len(b.Cells))
			for r, row := range b.Cells {
				modelCells[r] = make([]botmodels.RichBlockTableCell, len(row))
				for c, cell := range row {
					rt := toRichText(cell.Text, cell.Entities)
					modelCells[r][c] = botmodels.RichBlockTableCell{
						Text:     &rt,
						IsHeader: cell.IsHeader,
						Align:    cell.Align,
						Valign:   cell.Valign,
						Colspan:  cell.Colspan,
						Rowspan:  cell.Rowspan,
					}
				}
			}
			out[i].RichBlockTable = &botmodels.RichBlockTable{
				Cells:      modelCells,
				IsBordered: b.IsBordered,
				IsStriped:  b.IsStriped,
			}
		case RichBlockBlockquote:
			nested := make([]botmodels.RichBlock, 0)
			for _, row := range b.Cells {
				for _, cell := range row {
					rt := toRichText(cell.Text, cell.Entities)
					nested = append(nested, botmodels.RichBlock{
						Type: botmodels.RichBlockType(RichBlockParagraph),
						RichBlockParagraph: &botmodels.RichBlockParagraph{
							Text: rt,
						},
					})
				}
			}
			out[i].RichBlockBlockQuotation = &botmodels.RichBlockBlockQuotation{
				Blocks: nested,
			}
		}
	}
	return out
}

func toRichText(text string, entities []MessageEntity) botmodels.RichText {
	if len(entities) == 0 {
		return botmodels.RichText{PlainText: text}
	}
	return botmodels.RichText{
		Array: toModelEntities(text, entities),
	}
}

func toModelEntities(text string, entities []MessageEntity) []botmodels.RichText {
	out := make([]botmodels.RichText, len(entities))
	for i, e := range entities {
		out[i] = botmodels.RichText{
			Type:      botmodels.RichTextType(e.Type),
			PlainText: text[e.Offset : e.Offset+e.Length],
		}
	}
	return out
}
