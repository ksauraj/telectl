package handlers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
)

type ExecHandler struct {
	*BaseHandler
	sessions map[int64]*ExecSession
	mu       sync.Mutex
}

type ExecSession struct {
	PodName    string
	Namespace  string
	Container  string
	ChatID     int64
	Stdout     *strings.Builder
	Stderr     *strings.Builder
	Stdin      *io.PipeReader
	StdinWrite *io.PipeWriter
	Cancel     context.CancelFunc
	Active     bool
}

func NewExecHandler(b types.BotInterface) *ExecHandler {
	return &ExecHandler{
		BaseHandler: NewBaseHandler(b),
		sessions:    make(map[int64]*ExecSession),
	}
}

func (h *ExecHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /exec <pod> [-n namespace] [-c container] [command...]")
		return nil
	}

	podName := args[0]
	namespace := h.getNamespace(session, args[1:], h.getConfig().Kubernetes.DefaultNamespace)

	container, command := parseExecArgs(args)

	client := h.getK8sClient(session)

	// Check if pod exists and get containers
	pod, err := client.GetPod(ctx, namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	// If no container specified, use first container
	if container == "" {
		spec, _ := pod.Details["spec"].(map[string]interface{})
		if containers, ok := spec["containers"].([]interface{}); ok && len(containers) > 0 {
			if firstContainer, ok := containers[0].(map[string]interface{}); ok {
				if name, ok := firstContainer["name"].(string); ok {
					container = name
				}
			}
		}
		if container == "" {
			container = "main"
		}
	}

	// If interactive mode (no command or just shell)
	isShell := len(command) == 1 &&
		(command[0] == "sh" || command[0] == "bash" ||
			command[0] == "/bin/sh" || command[0] == "/bin/bash")
	if isShell {
		return h.startInteractiveSession(ctx, msg, session, podName, namespace, container)
	}

	// One-shot command execution
	return h.executeCommand(ctx, msg, client, podName, namespace, container, command)
}

// parseExecArgs splits /exec's arguments into the container flag and the
// command to run inside the pod.
//
// Only the leading run of telectl flags (-c/--container to pick the container,
// -n/--namespace which is consumed here but read by getNamespace) is
// interpreted. Everything from the first non-flag argument on is the command,
// passed through verbatim — it belongs to the container's shell, not to
// telectl, so its own flags (e.g. "sh -c 'echo hi'") are never parsed here.
//
// A "--" separator ends flag parsing explicitly: everything after it is the
// command, even if it looks like a flag. This is what lets
// "/exec pod -n ns -- printenv" work — without it the first bare command such
// as "printenv" already starts the command, but "-- <flag-like-command>" needs
// the separator. With no command, an interactive shell is started instead.
func parseExecArgs(args []string) (container string, command []string) {
	containerFlag := valueFlag{short: "-c", long: "--container"}
	namespaceFlag := valueFlag{short: flagNamespaceShort, long: flagNamespaceLong}

	// args[0] is the pod name.
	commandStart := 1
	for i := 1; i < len(args); {
		// An explicit "--" ends flag parsing; the command is the rest.
		if args[i] == "--" {
			commandStart = i + 1
			break
		}

		if value, consumed, ok := containerFlag.match(args[i], args[i+1:]); ok {
			if value != "" {
				container = value
			}
			i += consumed
			commandStart = i
			continue
		}

		// -n/--namespace is handled by getNamespace, but it must be skipped
		// here too or its flag and value would be mistaken for the command.
		if _, consumed, ok := namespaceFlag.match(args[i], args[i+1:]); ok {
			i += consumed
			commandStart = i
			continue
		}

		break // first non-flag argument begins the command
	}

	if commandStart < len(args) {
		return container, args[commandStart:]
	}
	// No command given: interactive shell.
	return container, []string{"sh"}
}

func (h *ExecHandler) startInteractiveSession(
	ctx context.Context,
	msg *tg.Message,
	session *types.UserSession,
	podName,
	namespace,
	container string,
) error {
	chatID := msg.Chat.ID

	// Check if user already has an active session
	h.mu.Lock()
	if existing, ok := h.sessions[chatID]; ok && existing.Active {
		h.mu.Unlock()
		h.sendResponse(chatID, formatters.Btn(formatters.GlyphBroken,
			"You already have an active exec session. Type /exit to end it first."))
		return nil
	}
	h.mu.Unlock()

	// Create pipes for stdin
	stdinR, stdinW := io.Pipe()

	execCtx, cancel := context.WithCancel(ctx)
	execSession := &ExecSession{
		PodName:    podName,
		Namespace:  namespace,
		Container:  container,
		ChatID:     chatID,
		Stdout:     &strings.Builder{},
		Stderr:     &strings.Builder{},
		Stdin:      stdinR,
		StdinWrite: stdinW,
		Cancel:     cancel,
		Active:     true,
	}

	h.mu.Lock()
	h.sessions[chatID] = execSession
	h.mu.Unlock()

	// Set user session to exec mode
	session.SetExecMode(podName, namespace, container)

	// Send welcome message with keyboard
	keyboard := tg.NewInlineKeyboardMarkup(
		tg.NewInlineKeyboardRow(
			tg.NewInlineKeyboardButtonData(formatters.Btn(formatters.GlyphCancel, "Exit session"), "exec:exit"),
		),
	)

	h.bot.SendKeyboard(chatID, fmt.Sprintf(
		"*Interactive session started*\nPod: %s/%s\nContainer: %s\n"+
			"Type commands below. Use /exit to quit.",
		namespace, podName, container), &keyboard)

	// Start exec in background
	go func() {
		err := h.getK8sClient(session).ExecInPod(execCtx, k8s.ExecOptions{
			Namespace: namespace,
			PodName:   podName,
			Container: container,
			Command:   []string{"sh"},
			Stdin:     stdinR,
			Stdout:    execSession.Stdout,
			Stderr:    execSession.Stderr,
			TTY:       true,
		})

		h.mu.Lock()
		delete(h.sessions, chatID)
		h.mu.Unlock()

		session.ClearExecMode()

		if err != nil && execCtx.Err() == nil {
			h.bot.SendText(chatID, formatters.Btn(formatters.GlyphBroken,
				fmt.Sprintf("Exec session ended with error: %s", err)))
		} else {
			h.bot.SendText(chatID, "Exec session ended")
		}
	}()

	return nil
}

func (h *ExecHandler) executeCommand(
	ctx context.Context,
	msg *tg.Message,
	client *k8s.Client,
	podName,
	namespace,
	container string,
	command []string,
) error {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	err := client.ExecInPod(ctx, k8s.ExecOptions{
		Namespace: namespace,
		PodName:   podName,
		Container: container,
		Command:   command,
		Stdout:    stdout,
		Stderr:    stderr,
		TTY:       false,
	})

	output := stdout.String()
	errOutput := stderr.String()

	if err != nil {
		h.sendResponse(msg.Chat.ID, formatters.Btn(formatters.GlyphBroken,
			fmt.Sprintf("Command failed: %s\n%s", err, errOutput)))
		return nil
	}

	if output == "" && errOutput == "" {
		h.sendResponse(msg.Chat.ID, formatters.Btn(formatters.GlyphDone, "Command executed successfully (no output)"))
		return nil
	}

	result := ""
	if output != "" {
		result += fmt.Sprintf("STDOUT:\n%s\n", output)
	}
	if errOutput != "" {
		result += fmt.Sprintf("STDERR:\n%s\n", errOutput)
	}

	// Same fence-in-HTML issue as logs: send a real code block instead.
	rich := tg.NewRichDoc()
	rich.Heading(3, strings.Join(command, " "))
	if result == "" {
		rich.Paragraph("(no output)")
	} else {
		rich.Code("", result)
	}
	h.bot.SendRich(msg.Chat.ID, rich.String(),
		fmt.Sprintf("Command: %s\n%s", strings.Join(command, " "), result))
	return nil
}

func (h *ExecHandler) HandleExecInput(ctx context.Context, msg *tg.Message, session *types.UserSession) {
	chatID := msg.Chat.ID

	h.mu.Lock()
	execSession, ok := h.sessions[chatID]
	h.mu.Unlock()

	if !ok || !execSession.Active {
		return
	}

	text := msg.Text
	if text == "/exit" || text == "exit" {
		execSession.Cancel()
		execSession.StdinWrite.Close()
		h.sendResponse(chatID, "Exiting exec session...")
		return
	}

	// Send command to stdin
	_, err := execSession.StdinWrite.Write([]byte(text + "\n"))
	if err != nil {
		h.sendResponse(chatID, formatters.Btn(formatters.GlyphBroken, fmt.Sprintf("Failed to send command: %s", err)))
	}
}

func (h *ExecHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}
