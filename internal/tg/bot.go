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

func (r *RealBot) SendRich(ctx context.Context, chatID int64, blocks []RichBlock, keyboard *InlineKeyboardMarkup) (*Message, error) {
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"rich_message": map[string]interface{}{"blocks": toModelBlocks(blocks)},
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	var result botmodels.Message
	if err := r.rawCall(ctx, "sendRichMessage", payload, &result); err != nil {
		return nil, err
	}
	return fromModelMessage(&result), nil
}

func (r *RealBot) EditRich(ctx context.Context, chatID int64, messageID int, blocks []RichBlock, keyboard *InlineKeyboardMarkup) (*Message, error) {
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"message_id":   messageID,
		"rich_message": map[string]interface{}{"blocks": toModelBlocks(blocks)},
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	var result botmodels.Message
	if err := r.rawCall(ctx, "editMessageText", payload, &result); err != nil {
		return nil, err
	}
	return fromModelMessage(&result), nil
}

func (r *RealBot) rawCall(ctx context.Context, method string, payload map[string]interface{}, result interface{}) error {
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

func (r *RealBot) EditKeyboard(ctx context.Context, chatID int64, messageID int, keyboard *InlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":      chatID,
		"message_id":   messageID,
		"reply_markup": keyboard,
	}
	var result botmodels.Message
	return r.rawCall(ctx, "editMessageReplyMarkup", payload, &result)
}

func (r *RealBot) SendText(ctx context.Context, chatID int64, text string, parseMode string, keyboard *InlineKeyboardMarkup) (*Message, error) {
	var pm botmodels.ParseMode
	switch parseMode {
	case "HTML":
		pm = botmodels.ParseModeHTML
	case "MarkdownV2":
		pm = botmodels.ParseModeMarkdown
	default:
		pm = botmodels.ParseModeHTML
	}
	params := &bottg.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: pm,
	}
	if keyboard != nil {
		params.ReplyMarkup = keyboard
	}
	msg, err := r.b.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}
	return fromModelMessage(msg), nil
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
