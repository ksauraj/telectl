package handlers

import (
	"context"
	"strings"

	"github.com/ksauraj/telectl/internal/config"
	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"go.uber.org/zap"
)

// Flag spellings accepted by the command parsers, and the one output format
// name compared against in more than one place.
const (
	flagNamespaceShort = "-n"
	flagNamespaceLong  = "--namespace"
	formatWide         = "wide"
)

type CommandHandler interface {
	Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error
}

type BaseHandler struct {
	bot types.BotInterface
}

func NewBaseHandler(b types.BotInterface) *BaseHandler {
	return &BaseHandler{bot: b}
}

func (h *BaseHandler) getNamespace(session *types.UserSession, args []string, defaultNS string) string {
	// Check for -n or --namespace flag
	for i, arg := range args {
		if arg == flagNamespaceShort || arg == flagNamespaceLong {
			if i+1 < len(args) {
				return args[i+1]
			}
		}
		if strings.HasPrefix(arg, "-n=") {
			return strings.TrimPrefix(arg, "-n=")
		}
		if strings.HasPrefix(arg, "--namespace=") {
			return strings.TrimPrefix(arg, "--namespace=")
		}
	}
	// Check session
	ns := session.GetNamespace()
	if ns != "" {
		return ns
	}
	return defaultNS
}

func (h *BaseHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}

// valueFlag describes a flag that takes a value, in all the spellings the
// command parsers accept: "-n prod", "-n=prod", "--namespace prod" and
// "--namespace=prod".
type valueFlag struct {
	short string // "-n"; empty if the flag has no short form
	long  string // "--namespace"
	set   func(*parsedFlags, string)
}

// parsedFlags holds the flags common to the resource commands.
type parsedFlags struct {
	namespace     string
	output        string
	selector      string
	fieldSelector string
	remaining     []string
}

// commonValueFlags is the flag table shared by /get and friends. Each entry
// used to be four near-identical switch cases; that repetition is what made the
// parser complex, not the logic.
var commonValueFlags = []valueFlag{
	{short: flagNamespaceShort, long: flagNamespaceLong,
		set: func(f *parsedFlags, v string) { f.namespace = v }},
	{short: "-o", long: "--output",
		set: func(f *parsedFlags, v string) { f.output = v }},
	{short: "-l", long: "--selector",
		set: func(f *parsedFlags, v string) { f.selector = v }},
	{long: "--field-selector",
		set: func(f *parsedFlags, v string) { f.fieldSelector = v }},
}

// match reports whether arg is this flag, returning its value and how many
// arguments were consumed.
//
// A separated flag at the very end of args ("... -n") consumes only itself and
// yields no value, so a half-typed command cannot read past the slice.
func (vf valueFlag) match(arg string, rest []string) (value string, consumed int, ok bool) {
	switch {
	case arg == vf.long, vf.short != "" && arg == vf.short:
		if len(rest) == 0 {
			return "", 1, true // dangling flag: nothing to take
		}
		return rest[0], 2, true
	case strings.HasPrefix(arg, vf.long+"="):
		return strings.TrimPrefix(arg, vf.long+"="), 1, true
	case vf.short != "" && strings.HasPrefix(arg, vf.short+"="):
		return strings.TrimPrefix(arg, vf.short+"="), 1, true
	}
	return "", 0, false
}

// parseFlags extracts the common flags, returning anything unrecognised as
// positional arguments.
func parseFlags(args []string) (namespace, output, selector, fieldSelector string, remaining []string) {
	f := parsedFlags{remaining: []string{}}

	for i := 0; i < len(args); {
		arg := args[i]

		// -A/--all-namespaces clears the namespace rather than setting one.
		if arg == "--all-namespaces" || arg == "-A" {
			f.namespace = ""
			i++
			continue
		}

		matched := false
		for _, vf := range commonValueFlags {
			value, consumed, ok := vf.match(arg, args[i+1:])
			if !ok {
				continue
			}
			if consumed == 2 || value != "" {
				vf.set(&f, value)
			}
			i += consumed
			matched = true
			break
		}
		if !matched {
			f.remaining = append(f.remaining, arg)
			i++
		}
	}

	return f.namespace, f.output, f.selector, f.fieldSelector, f.remaining
}

func (h *BaseHandler) getK8sClient(session *types.UserSession) *k8s.Client {
	var userID int64
	if session != nil {
		userID = session.UserID
	}
	if c, ok := h.bot.K8sClient().(*k8s.Client); ok {
		// Check if bot has K8sClientForUser method (for impersonation)
		type impersonatedBot interface {
			K8sClientForUser(userID int64) *k8s.Client
		}
		if b, ok := h.bot.(impersonatedBot); ok {
			return b.K8sClientForUser(userID)
		}
		return c
	}
	return nil
}

func (h *BaseHandler) getLogger() *zap.Logger {
	if l, ok := h.bot.Logger().(*zap.Logger); ok {
		return l
	}
	return nil
}

// getConfig returns the typed *config.Config from the bot.
func (h *BaseHandler) getConfig() *config.Config {
	if c, ok := h.bot.Config().(*config.Config); ok {
		return c
	}
	return nil
}
