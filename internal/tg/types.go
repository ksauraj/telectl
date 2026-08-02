package tg

type Message struct {
	ID       int
	ChatID   int64
	Text     string
	Entities []MessageEntity
	From     *User
	Chat     *Chat
}

type User struct {
	ID        int64
	FirstName string
	Username  string
}

type Chat struct {
	ID    int64
	Type  string
	Title string
}

type CallbackQuery struct {
	ID              string
	From            *User
	Message         *Message
	InlineMessageID string
	Data            string
}

type Update struct {
	ID            int64
	Message       *Message
	CallbackQuery *CallbackQuery
}

type MessageEntity struct {
	Type   string
	Offset int
	Length int
	URL    string
	User   *User
}

type InlineButton struct {
	Text         string
	CallbackData string
	URL          string
}

type BotCommand struct {
	Command     string
	Description string
}

type KeyboardButton struct {
	Text            string
	RequestContact  bool
	RequestLocation bool
	WebApp          *WebAppInfo
}

type WebAppInfo struct {
	URL string
}

type ReplyKeyboardMarkup struct {
	Keyboard              [][]KeyboardButton
	ResizeKeyboard        bool
	OneTimeKeyboard       bool
	InputFieldPlaceholder string
	Selective             bool
	IsPersistent          bool
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton
}

type InlineKeyboardButton struct {
	Text                         string
	IconCustomEmojiID            string
	Style                        string
	URL                          string
	CallbackData                 string
	WebApp                       *WebAppInfo
	LoginURL                     *LoginURL
	SwitchInlineQuery            *string
	SwitchInlineQueryCurrentChat *string
	SwitchInlineQueryChosenChat  *SwitchInlineQueryChosenChat
	CopyText                     *CopyTextButton
	CallbackGame                 *CallbackGame
	Pay                          bool
}

type LoginURL struct {
	URL                string
	ForwardText        string
	BotUsername        string
	RequestWriteAccess bool
}

type SwitchInlineQueryChosenChat struct {
	Query                   string
	AllowUserChats          bool
	AllowBotChats           bool
	AllowGroupChats         bool
	AllowChannelChats       bool
	UserAdministratorRights *ChatAdministratorRights
	BotAdministratorRights  *ChatAdministratorRights
}

type ChatAdministratorRights struct {
	IsAnonymous         bool
	CanManageChat       bool
	CanManageVideoChats bool
	CanDeleteMessages   bool
	CanRestrictMembers  bool
	CanPromoteMembers   bool
	CanChangeInfo       bool
	CanInviteUsers      bool
	CanPostMessages     bool
	CanEditMessages     bool
	CanPinMessages      bool
	CanManageTopics     bool
}

type CopyTextButton struct {
	Text string
}

type CallbackGame struct{}

type Bot = RealBot

type InlineQuery struct {
	ID       string
	From     *User
	Query    string
	Offset   string
	ChatType string
	Location *Location
}

type Location struct {
	Longitude float64
	Latitude  float64
}

type InlineQueryResultArticle struct {
	Type                string
	ID                  string
	Title               string
	Description         string
	URL                 string
	HideURL             bool
	ThumbURL            string
	ThumbWidth          int
	ThumbHeight         int
	InputMessageContent InputMessageContent
	ReplyMarkup         *InlineKeyboardMarkup
}

type InputMessageContent interface {
	isInputMessageContent()
}

type InputTextMessageContent struct {
	MessageText           string
	ParseMode             string
	Entities              []MessageEntity
	LinkPreviewOptions    *LinkPreviewOptions
	DisableWebPagePreview bool
}

func (InputTextMessageContent) isInputMessageContent() {}

type LinkPreviewOptions struct {
	IsDisabled       bool
	URL              string
	PreferSmallMedia bool
	PreferLargeMedia bool
	ShowAboveText    bool
}
