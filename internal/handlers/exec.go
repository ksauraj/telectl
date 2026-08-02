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

	// Parse flags
	container := ""
	commandStart := 1
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-c" || arg == "--container" {
			if i+1 < len(args) {
				container = args[i+1]
				commandStart = i + 2
				i++
			}
		} else if strings.HasPrefix(arg, "-c=") {
			container = strings.TrimPrefix(arg, "-c=")
			commandStart = i + 1
		} else if strings.HasPrefix(arg, "--container=") {
			container = strings.TrimPrefix(arg, "--container=")
			commandStart = i + 1
		}
	}

	// Build command
	var command []string
	if commandStart < len(args) {
		command = args[commandStart:]
	} else {
		// Interactive mode
		command = []string{"sh"}
	}

	client := h.getK8sClient()

	// Check if pod exists and get containers
	pod, err := client.GetPod(ctx, namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	// If no container specified, use first container
	if container == "" {
		if containers, ok := pod.Details["spec"].(map[string]interface{})["containers"].([]interface{}); ok && len(containers) > 0 {
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
	if len(command) == 1 && (command[0] == "sh" || command[0] == "bash" || command[0] == "/bin/sh" || command[0] == "/bin/bash") {
		return h.startInteractiveSession(ctx, msg, session, podName, namespace, container)
	}

	// One-shot command execution
	return h.executeCommand(ctx, msg, client, podName, namespace, container, command)
}

func (h *ExecHandler) startInteractiveSession(ctx context.Context, msg *tg.Message, session *types.UserSession, podName, namespace, container string) error {
	chatID := msg.Chat.ID

	// Check if user already has an active session
	h.mu.Lock()
	if existing, ok := h.sessions[chatID]; ok && existing.Active {
		h.mu.Unlock()
		h.sendResponse(chatID, "❌ You already have an active exec session. Type /exit to end it first.")
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
			tg.NewInlineKeyboardButtonData("🔴 Exit Session", "exec:exit"),
		),
	)

	h.bot.SendKeyboard(chatID, fmt.Sprintf("🖥️ *Interactive session started*\nPod: %s/%s\nContainer: %s\nType commands below. Use /exit to quit.", namespace, podName, container), &keyboard)

	// Start exec in background
	go func() {
		err := h.getK8sClient().ExecInPod(execCtx, k8s.ExecOptions{
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
			h.bot.SendText(chatID, fmt.Sprintf("❌ Exec session ended with error: %s", err))
		} else {
			h.bot.SendText(chatID, "👋 Exec session ended")
		}
	}()

	return nil
}

func (h *ExecHandler) executeCommand(ctx context.Context, msg *tg.Message, client *k8s.Client, podName, namespace, container string, command []string) error {
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
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("❌ Command failed: %s\n%s", err, errOutput))
		return nil
	}

	if output == "" && errOutput == "" {
		h.sendResponse(msg.Chat.ID, "✅ Command executed successfully (no output)")
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
	rich.Heading(3, "🖥️ "+strings.Join(command, " "))
	if result == "" {
		rich.Paragraph("(no output)")
	} else {
		rich.Code("", result)
	}
	h.bot.SendRich(msg.Chat.ID, rich.String(),
		fmt.Sprintf("🖥️ Command: %s\n%s", strings.Join(command, " "), result))
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
		h.sendResponse(chatID, "👋 Exiting exec session...")
		return
	}

	// Send command to stdin
	_, err := execSession.StdinWrite.Write([]byte(text + "\n"))
	if err != nil {
		h.sendResponse(chatID, fmt.Sprintf("❌ Failed to send command: %s", err))
	}
}

func (h *ExecHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}
