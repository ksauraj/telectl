package bot

import (
	"context"
	"strings"

	"github.com/ksauraj/telectl/internal/menus"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
	"go.uber.org/zap"
)

// The single pane.
//
// Every menu interaction happens in one message, which is edited in place. The
// alternative — what this replaces — was that navigation edited the pane but
// every *verb* sent a new message, so a few taps buried the keyboard several
// screens up and the user had to scroll back to carry on. Tapping four verbs
// produced five messages, only one of which had buttons.
//
// The rule: a callback never sends. It edits the message its button lives on.
// Sending is reserved for entry points the user initiates with a typed command,
// where there is no pane to edit yet.
//
// Two consequences worth stating, because they are the cost of this design:
//
//   - Output is transient. Tapping Events replaces the Labels output that was
//     there. Chat scrollback is no longer a record of what was inspected; the
//     pane shows current state and nothing else. This is the behaviour that was
//     asked for, and it is the same tradeoff a TUI makes.
//   - Output must fit one message. A rendered pane cannot be split across
//     messages the way SendLongMessage splits, so long content is truncated to
//     fit with a pointer to the typed command that prints it in full. That
//     truncation is explicit in the text rather than silent.

// paneLimit is the size a pane body is truncated to.
//
// Telegram's hard cap is 4096 characters for a message, but a pane also carries
// a keyboard and, for rich content, markup that expands on the way out. The
// margin keeps an edit from being rejected for length after the body already
// passed a check against the raw limit.
const paneLimit = 3500

// pane is one rendered view: the body, the keyboard under it, and the plain-text
// fallback used if the server rejects the rich body.
type pane struct {
	rich     string // Rich Markdown body
	fallback string // plain HTML, used when rich is rejected
	kb       *tg.InlineKeyboardMarkup
	// keepTail truncates from the top instead of the bottom when the body is
	// too long. Logs are the canonical case: the newest lines are at the end,
	// and cutting the tail would discard exactly the output the user asked for.
	keepTail bool
}

// showPane renders a pane into the message the callback came from.
//
// messageID == 0 means there is no pane to edit — the caller reached here from a
// typed command rather than a button tap — so it sends one instead. That is the
// only path that adds a message to the chat.
func (b *Bot) showPane(ctx context.Context, chatID int64, messageID int, p pane) {
	if p.keepTail {
		p.rich = truncateForPaneTail(p.rich)
		p.fallback = truncateForPaneTail(p.fallback)
	} else {
		p.rich = truncateForPane(p.rich)
		p.fallback = truncateForPane(p.fallback)
	}

	if messageID == 0 {
		b.SendRichKeyboard(chatID, p.rich, p.fallback, p.kb)
		return
	}
	b.editRichView(ctx, chatID, messageID, p.rich, p.fallback, p.kb)
}

// truncateForPane cuts a body to paneLimit on a line boundary where it can, so
// a truncated table does not end mid-row. The note is part of the returned text
// rather than a separate message: a silent cut looks like missing data.
func truncateForPane(s string) string {
	if len(s) <= paneLimit {
		return s
	}
	// Rune-aware: slicing bytes can split a multi-byte character and produce
	// invalid UTF-8, which Telegram rejects outright.
	runes := []rune(s)
	if len(runes) <= paneLimit {
		return s
	}
	cut := string(runes[:paneLimit])
	if nl := strings.LastIndex(cut, "\n"); nl > paneLimit/2 {
		cut = cut[:nl]
	}
	return cut + "\n\n… truncated to fit one message."
}

// truncateForPaneTail keeps the *last* paneLimit runes instead of the first.
// Used for content whose newest output is at the end (log tails, event feeds):
// when something must give, it is the older head of the output, not the lines
// the user most likely wants.
func truncateForPaneTail(s string) string {
	if len(s) <= paneLimit {
		return s
	}
	runes := []rune(s)
	if len(runes) <= paneLimit {
		return s
	}
	cut := string(runes[len(runes)-paneLimit:])
	if nl := strings.Index(cut, "\n"); nl > 0 && nl < paneLimit/2 {
		cut = cut[nl+1:]
	}
	return "… earlier output truncated to fit one message.\n\n" + cut
}

// verbPane wraps the output of a detail-pane verb: the body it produced, plus a
// keyboard that leads back to the resource it was invoked on.
//
// Every verb gets this keyboard. A pane with no way out is how the old code
// stranded the user — output arrived as a bare message, and the only route back
// was scrolling up to a keyboard from several taps ago.
func (b *Bot) verbPane(r detailReq, rich, fallback string) pane {
	kb := b.menuBuilder.GetVerbResultKeyboard(r.kind, r.ns, r.name)
	return pane{rich: rich, fallback: fallback, kb: &kb}
}

// showVerbResult renders a verb's output into the pane, with a route back.
func (b *Bot) showVerbResult(ctx context.Context, r detailReq, rich, fallback string) {
	b.showPane(ctx, r.chatID, r.messageID, b.verbPane(r, rich, fallback))
}

// showLogResult renders log output into the pane. Logs differ from other verb
// output in one critical way: the newest lines are at the end, so when the
// pane must truncate to fit one message, it is the old head that gets cut —
// never the tail the user asked for.
func (b *Bot) showLogResult(ctx context.Context, r detailReq, rich, fallback string) {
	p := b.verbPane(r, rich, fallback)
	p.keepTail = true
	b.showPane(ctx, r.chatID, r.messageID, p)
}

// reportPaneError renders a failure into the pane instead of sending it.
//
// The API server's own message is shown: it is almost always the actionable part
// (Forbidden, NotFound, "metrics-server not available"). The keyboard is still
// attached, so a failed verb leaves the user where they were rather than at a
// dead end.
func (b *Bot) reportPaneError(ctx context.Context, r detailReq, what string, err error) {
	b.logger.Error("Menu action failed",
		zap.String("action", what),
		zap.String("kind", r.kind),
		zap.String("name", r.name),
		zap.Error(err))

	d := tg.NewRichDoc()
	d.Heading(3, formatters.Btn(formatters.GlyphBroken, "Failed to "+what))
	d.Paragraph(err.Error())
	b.showVerbResult(ctx, r, d.String(),
		formatters.Btn(formatters.GlyphBroken, "Failed to ")+
			formatters.EscapeHTML(what)+": "+formatters.EscapeHTML(err.Error()))
}

// noticePane renders a short informational body with the standard way back —
// for verbs whose answer is a sentence rather than a rendered object.
func (b *Bot) showNotice(ctx context.Context, r detailReq, heading, body string) {
	d := tg.NewRichDoc()
	d.Heading(3, heading)
	d.Paragraph(body)
	b.showVerbResult(ctx, r, d.String(),
		"<b>"+formatters.EscapeHTML(heading)+"</b>\n"+formatters.EscapeHTML(body))
}

// editToMainMenu replaces the pane with the main menu.
func (b *Bot) editToMainMenu(ctx context.Context, chatID int64, messageID int, session *types.UserSession) {
	session.SetMenuState(&types.MenuState{CurrentView: "main"})
	kb := b.menuBuilder.GetMainMenuInlineKeyboard()
	b.editView(ctx, chatID, messageID, b.mainMenuText(session), &kb)
}

// usagePane renders a usage hint for a verb that has no in-chat implementation
// (exec, port-forward), keeping the user on the resource they came from.
func (b *Bot) showUsage(ctx context.Context, r detailReq, what, usage string) {
	d := tg.NewRichDoc()
	d.Heading(3, what)
	d.Paragraph("Not available from a button — run it as a command:")
	d.Code("", usage)
	b.showVerbResult(ctx, r, d.String(),
		"<b>"+formatters.EscapeHTML(what)+"</b>\nUsage: <code>"+
			formatters.EscapeHTML(usage)+"</code>")
}

// detailReqFor builds a detailReq for a resource the bot is acting on outside
// the callback dispatcher, so those paths render through the same pane helpers.
func detailReqFor(chatID int64, messageID int, kind, ns, name string, session *types.UserSession) detailReq {
	return detailReq{
		chatID:    chatID,
		messageID: messageID,
		kind:      menus.CanonicalResource(kind),
		ns:        ns,
		name:      name,
		session:   session,
	}
}

// showHelpPane renders the command reference into the pane.
//
// Sent as plain HTML rather than rich markup: the reference is dense with the
// characters Markdown treats as control (*, _, [, `), and it already carries its
// own <b> markup. Escaping it into rich text would either mangle it or require a
// second copy that could drift from the typed /help.
func (b *Bot) showHelpPane(ctx context.Context, chatID int64, messageID int) {
	kb := b.menuBuilder.GetSectionResultKeyboard(menus.SectionMain)
	text := truncateForPane(formatters.HelpText)
	if messageID == 0 {
		b.SendKeyboard(chatID, text, &kb)
		return
	}
	b.editView(ctx, chatID, messageID, text, &kb)
}

// showSectionPane renders output belonging to a top-level section (Monitoring,
// Operations, Settings) with a keyboard back to that section.
//
// These panes have no resource behind them, so verbPane's back-to-resource
// keyboard does not apply. Without this they went out as plain messages, which
// is how the monitor and settings views used to add to the pile.
func (b *Bot) showSectionPane(
	ctx context.Context,
	chatID int64,
	messageID int,
	section, rich, fallback string,
) {
	kb := b.menuBuilder.GetSectionResultKeyboard(section)
	b.showPane(ctx, chatID, messageID, pane{rich: rich, fallback: fallback, kb: &kb})
}

// showSectionNotice renders a short body into a section pane.
func (b *Bot) showSectionNotice(
	ctx context.Context,
	chatID int64,
	messageID int,
	section, heading, body string,
) {
	d := tg.NewRichDoc()
	d.Heading(3, heading)
	d.Paragraph(body)
	b.showSectionPane(ctx, chatID, messageID, section, d.String(),
		"<b>"+formatters.EscapeHTML(heading)+"</b>\n"+formatters.EscapeHTML(body))
}

// showSectionUsage renders a usage hint into a section pane, for the operations
// entries that are signposts to typed commands: the operations menu has no
// resource selected, so there is nothing for the verb to act on yet.
func (b *Bot) showSectionUsage(
	ctx context.Context,
	chatID int64,
	messageID int,
	section, what, usage, note string,
) {
	d := tg.NewRichDoc()
	d.Heading(3, what)
	if note != "" {
		d.Paragraph(note)
	}
	d.Code("", usage)
	b.showSectionPane(ctx, chatID, messageID, section, d.String(),
		"<b>"+formatters.EscapeHTML(what)+"</b>\nUsage: <code>"+
			formatters.EscapeHTML(usage)+"</code>")
}
