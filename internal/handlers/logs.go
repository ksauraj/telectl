package handlers

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/k8s-telegram-bot/internal/bot"
	"github.com/ksauraj/k8s-telegram-bot/internal/k8s"
	"github.com/ksauraj/k8s-telegram-bot/internal/utils/formatters"
	"go.uber.org/zap"
)

type LogsHandler struct {
	*BaseHandler
}

func NewLogsHandler(b *bot.Bot) *LogsHandler {
	return &LogsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *LogsHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *bot.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /logs <pod> [-n namespace] [-c container] [-f] [-p] [--tail N] [--since TIME]")
		return nil
	}

	podName := args[0]
	namespace := h.getNamespace(session, args[1:], h.bot.config.Kubernetes.DefaultNamespace)

	// Parse flags
	opts := k8s.PodLogOptions{
		Namespace: namespace,
		PodName:   podName,
	}

	remaining := []string{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c" || arg == "--container":
			if i+1 < len(args) {
				opts.Container = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "-c="):
			opts.Container = strings.TrimPrefix(arg, "-c=")
		case strings.HasPrefix(arg, "--container="):
			opts.Container = strings.TrimPrefix(arg, "--container=")
		case arg == "-f" || arg == "--follow":
			opts.Follow = true
		case arg == "-p" || arg == "--previous":
			opts.Previous = true
		case arg == "--timestamps":
			opts.Timestamps = true
		case arg == "--tail":
			if i+1 < len(args) {
				if n, err := strconv.ParseInt(args[i+1], 10, 64); err == nil {
					opts.TailLines = &n
				}
				i++
			}
		case strings.HasPrefix(arg, "--tail="):
			if n, err := strconv.ParseInt(strings.TrimPrefix(arg, "--tail="), 10, 64); err == nil {
				opts.TailLines = &n
			}
		case arg == "--since":
			if i+1 < len(args) {
				if d, err := time.ParseDuration(args[i+1]); err == nil {
					sec := int64(d.Seconds())
					opts.SinceSeconds = &sec
				}
				i++
			}
		case strings.HasPrefix(arg, "--since="):
			if d, err := time.ParseDuration(strings.TrimPrefix(arg, "--since=")); err == nil {
				sec := int64(d.Seconds())
				opts.SinceSeconds = &sec
			}
		default:
			remaining = append(remaining, arg)
		}
	}

	client := h.getK8sClient()

	// If follow mode, we need to stream
	if opts.Follow {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf("📋 Following logs for %s/%s (container: %s)...\nUse /cancel to stop", namespace, podName, opts.Container))
		return h.streamLogs(ctx, msg.Chat.ID, client, opts)
	}

	// One-time log fetch
	reader, err := client.GetPodLogs(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read logs: %w", err)
	}

	if len(logs) == 0 {
		h.sendResponse(msg.Chat.ID, "📭 No logs found")
		return nil
	}

	formatted := formatters.FormatPodLogs(string(logs), 100)
	h.sendResponse(msg.Chat.ID, fmt.Sprintf("📋 Logs for %s/%s:\n```\n%s\n```", namespace, podName, formatted))
	return nil
}

func (h *LogsHandler) streamLogs(ctx context.Context, chatID int64, client *k8s.Client, opts k8s.PodLogOptions) error {
	// For now, just do a one-time fetch with follow=false
	// Real streaming would require a more complex implementation with WebSocket or long-polling
	opts.Follow = false
	reader, err := client.GetPodLogs(ctx, opts)
	if err != nil {
		return err
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	formatted := formatters.FormatPodLogs(string(logs), 200)
	h.bot.sendMessage(chatID, fmt.Sprintf("📋 Logs for %s/%s (last 200 lines):\n```\n%s\n```", opts.Namespace, opts.PodName, formatted))
	return nil
}

func (h *LogsHandler) sendResponse(chatID int64, text string) {
	h.bot.sendLongMessage(chatID, text)
}