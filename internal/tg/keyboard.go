package tg

import (
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

func EscapeMDV2(s string) string {
	special := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, c := range special {
		s = strings.ReplaceAll(s, c, "\\"+c)
	}
	return s
}
