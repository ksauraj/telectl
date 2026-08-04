package handlers

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/ksauraj/telectl/internal/k8s"
	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
	"github.com/ksauraj/telectl/internal/utils/formatters"
)

type LogsHandler struct {
	*BaseHandler
}

func NewLogsHandler(b types.BotInterface) *LogsHandler {
	return &LogsHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *LogsHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	if len(args) == 0 {
		h.sendResponse(msg.Chat.ID, "Usage: /logs <pod> [-n namespace] [-c container] [-f] [-p] [--tail N] [--since TIME]")
		return nil
	}

	podName := args[0]
	namespace := h.getNamespace(session, args[1:], h.getConfig().Kubernetes.DefaultNamespace)

	opts := parseLogFlags(podName, namespace, args)

	client := h.getK8sClient()

	// If follow mode, we need to stream
	if opts.Follow {
		h.sendResponse(msg.Chat.ID, fmt.Sprintf(
			"Following logs for %s/%s (container: %s)...\nUse /cancel to stop",
			namespace, podName, opts.Container))
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
		h.sendResponse(msg.Chat.ID, "No logs found")
		return nil
	}

	// The old text path wrapped output in ``` fences but sent it as HTML, so the
	// fences showed up literally. The rich path emits a real code block.
	formatted := formatters.FormatPodLogs(string(logs), 100)
	h.bot.SendRich(msg.Chat.ID,
		formatters.RichLogs(namespace+"/"+podName, opts.Container, formatted),
		fmt.Sprintf("Logs for %s/%s:\n%s", namespace, podName, formatted))
	return nil
}

// logValueFlags are the /logs flags that take a value. Reuses the valueFlag
// matcher from base.go so every command accepts the same four spellings of a
// flag ("-c x", "-c=x", "--container x", "--container=x").
var logValueFlags = []struct {
	flag valueFlag
	set  func(*k8s.PodLogOptions, string)
}{
	{valueFlag{short: "-c", long: "--container"},
		func(o *k8s.PodLogOptions, v string) { o.Container = v }},
	{valueFlag{long: "--tail"}, setTailLines},
	{valueFlag{long: "--since"}, setSince},
}

// logBoolFlags are the /logs flags that are simply present or absent.
var logBoolFlags = map[string]func(*k8s.PodLogOptions){
	"-f":           func(o *k8s.PodLogOptions) { o.Follow = true },
	"--follow":     func(o *k8s.PodLogOptions) { o.Follow = true },
	"-p":           func(o *k8s.PodLogOptions) { o.Previous = true },
	"--previous":   func(o *k8s.PodLogOptions) { o.Previous = true },
	"--timestamps": func(o *k8s.PodLogOptions) { o.Timestamps = true },
}

// parseLogFlags builds PodLogOptions from the kubectl-style flags accepted by
// /logs. Split out of Handle so flag handling can be read — and tested —
// without the fetch-and-render path around it.
//
// Namespace is resolved by the caller (getNamespace also consults the session),
// so -n is already accounted for and is skipped here.
func parseLogFlags(podName, namespace string, args []string) k8s.PodLogOptions {
	opts := k8s.PodLogOptions{Namespace: namespace, PodName: podName}

	// args[0] is the pod name.
	for i := 1; i < len(args); {
		arg := args[i]

		if apply, ok := logBoolFlags[arg]; ok {
			apply(&opts)
			i++
			continue
		}

		matched := false
		for _, lf := range logValueFlags {
			value, consumed, ok := lf.flag.match(arg, args[i+1:])
			if !ok {
				continue
			}
			if value != "" {
				lf.set(&opts, value)
			}
			i += consumed
			matched = true
			break
		}
		if !matched {
			i++
		}
	}
	return opts
}

// setTailLines applies --tail. A malformed value is ignored rather than
// rejected: the flag is optional, and refusing to show logs over a bad --tail
// would be worse than showing all of them.
func setTailLines(opts *k8s.PodLogOptions, raw string) {
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		opts.TailLines = &n
	}
}

// setSince applies --since, which takes a Go duration ("5m", "2h").
func setSince(opts *k8s.PodLogOptions, raw string) {
	if d, err := time.ParseDuration(raw); err == nil {
		sec := int64(d.Seconds())
		opts.SinceSeconds = &sec
	}
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
	h.bot.SendRich(chatID,
		formatters.RichLogs(opts.Namespace+"/"+opts.PodName, opts.Container, formatted),
		fmt.Sprintf("Logs for %s/%s (last 200 lines):\n%s", opts.Namespace, opts.PodName, formatted))
	return nil
}

func (h *LogsHandler) sendResponse(chatID int64, text string) {
	h.bot.SendLongMessage(chatID, text)
}
