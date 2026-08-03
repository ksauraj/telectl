package tg

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	bottg "github.com/go-telegram/bot"
	botmodels "github.com/go-telegram/bot/models"
)

type BotClient struct {
	Token  string
	Base   string
	Client *http.Client
}

func NewBotClient(token string) *BotClient {
	return &BotClient{
		Token:  token,
		Base:   "https://api.telegram.org/bot" + token + "/",
		Client: &http.Client{Timeout: 30 * 1e9},
	}
}

func (c *BotClient) call(ctx context.Context, method string, payload interface{}) (*http.Response, error) {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.Base+method, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	return c.Client.Do(req)
}

type RealBot struct {
	b     *bottg.Bot
	raw   *BotClient
	token string
}

func NewRealBot(token string, opts ...bottg.Option) (*RealBot, error) {
	b, err := bottg.New(token, opts...)
	if err != nil {
		return nil, err
	}
	return &RealBot{b: b, raw: NewBotClient(token), token: token}, nil
}

// SendRich sends a Rich Message (Bot API 10.1+), which renders tables,
// headings and dividers natively instead of as monospace text.
//
// The previous implementation posted {"rich_message":{"blocks":[...]}} by hand.
// That is the *receive* shape (models.RichMessage); the send type
// models.InputRichMessage takes a markup string, so the old payload could never
// have worked. It was also never called from anywhere.
func (r *RealBot) SendRich(
	ctx context.Context,
	chatID int64,
	markdown string,
	keyboard *InlineKeyboardMarkup,
) (*Message, error) {
	params := &bottg.SendRichMessageParams{
		ChatID:      chatID,
		RichMessage: botmodels.InputRichMessage{Markdown: markdown},
	}
	if keyboard != nil {
		params.ReplyMarkup = toModelInlineKeyboard(keyboard)
	}
	msg, err := r.b.SendRichMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return fromModelMessage(msg), nil
}

// EditRich replaces an existing message with rich content.
func (r *RealBot) EditRich(
	ctx context.Context,
	chatID int64,
	messageID int,
	markdown string,
	keyboard *InlineKeyboardMarkup,
) (*Message, error) {
	params := &bottg.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		RichMessage: &botmodels.InputRichMessage{Markdown: markdown},
	}
	if keyboard != nil {
		params.ReplyMarkup = toModelInlineKeyboard(keyboard)
	}
	msg, err := r.b.EditMessageText(ctx, params)
	if err != nil {
		return nil, err
	}
	return fromModelMessage(msg), nil
}

func (r *RealBot) rawCall(
	ctx context.Context,
	method string,
	payload map[string]interface{},
	result interface{},
) error {
	resp, err := r.raw.call(ctx, method, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("telegram API %s: %s", method, resp.Status)
	}
	var wrapper struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return err
	}
	if !wrapper.OK {
		return fmt.Errorf("telegram API %s: ok=false", method)
	}
	return json.Unmarshal(wrapper.Result, result)
}

// EditText replaces the text and inline keyboard of an existing message. Used
// for single-pane menu navigation so drilling into a menu does not spam the chat.
func (r *RealBot) EditText(
	ctx context.Context,
	chatID int64,
	messageID int,
	text string,
	parseMode string,
	keyboard *InlineKeyboardMarkup,
) (*Message, error) {
	params := &bottg.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: toModelParseMode(parseMode),
	}
	if keyboard != nil {
		params.ReplyMarkup = toModelInlineKeyboard(keyboard)
	}
	msg, err := r.b.EditMessageText(ctx, params)
	if err != nil {
		return nil, err
	}
	return fromModelMessage(msg), nil
}

func (r *RealBot) EditKeyboard(ctx context.Context, chatID int64, messageID int, keyboard *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"message_id":   messageID,
		"reply_markup": toModelInlineKeyboard(keyboard),
	}
	var result botmodels.Message
	return r.rawCall(ctx, "editMessageReplyMarkup", payload, &result)
}

func (r *RealBot) SendText(
	ctx context.Context,
	chatID int64,
	text string,
	parseMode string,
	keyboard *InlineKeyboardMarkup,
) (*Message, error) {
	params := &bottg.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: toModelParseMode(parseMode),
	}
	if keyboard != nil {
		params.ReplyMarkup = toModelInlineKeyboard(keyboard)
	}
	msg, err := r.b.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return fromModelMessage(msg), nil
}

// SendTextReplyKeyboard sends text with a persistent reply keyboard attached.
func (r *RealBot) SendTextReplyKeyboard(
	ctx context.Context,
	chatID int64,
	text string,
	parseMode string,
	keyboard *ReplyKeyboardMarkup,
) (*Message, error) {
	params := &bottg.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: toModelParseMode(parseMode),
	}
	if keyboard != nil {
		params.ReplyMarkup = toModelReplyKeyboard(keyboard)
	}
	msg, err := r.b.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return fromModelMessage(msg), nil
}

func toModelParseMode(parseMode string) botmodels.ParseMode {
	switch parseMode {
	case "HTML":
		return botmodels.ParseModeHTML
	case "MarkdownV2":
		return botmodels.ParseModeMarkdown
	case "Markdown":
		return botmodels.ParseModeMarkdownV1
	default:
		return botmodels.ParseModeHTML
	}
}

func (r *RealBot) GetMe(ctx context.Context) (*botmodels.User, error) {
	return r.b.GetMe(ctx)
}

func (r *RealBot) AnswerCallbackQuery(ctx context.Context, callbackID, text string, showAlert bool) error {
	params := &bottg.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       showAlert,
	}
	_, err := r.b.AnswerCallbackQuery(ctx, params)
	return err
}

func (r *RealBot) LibraryBot() *bottg.Bot {
	return r.b
}
