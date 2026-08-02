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
