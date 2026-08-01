package handlers

import (
	"context"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/ksauraj/telectl/internal/types"
)

type HelpHandler struct {
	*BaseHandler
}

func NewHelpHandler(b types.BotInterface) *HelpHandler {
	return &HelpHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *HelpHandler) Handle(ctx context.Context, msg *tgbotapi.Message, args []string, session *types.UserSession) error {
	help := `📚 *k8s-telegram-bot Command Reference*

*Resource Commands:*
/get <resource> [name] [-n namespace] [-o json|yaml|wide] [-l selector]
  Resources: pods, deployments, services, replicasets, namespaces, nodes, configmaps, secrets, pvcs, pvs, ingresses
  Examples:
    /get pods
    /get pods -n kube-system
    /get deployment my-app -o wide
    /get pods -l app=nginx

/describe <resource> <name> [-n namespace]
  Get detailed information about a resource
  Example: /describe pod my-pod -n default

/logs <pod> [-n namespace] [-c container] [-f] [-p] [--tail N] [--since TIME]
  View pod logs
  Flags:
    -c, --container    Container name
    -f, --follow       Follow log output
    -p, --previous     Show previous container logs
    --tail N           Lines from end of log
    --since TIME       Show logs since timestamp (e.g., 1h, 30m)
  Example: /logs my-pod -c nginx -f --tail 100

/exec <pod> [-n namespace] [-c container] [command]
  Execute a command in a container
  If no command provided, starts interactive session
  Example: /exec my-pod -c nginx -- ls -la

/portforward <pod> <local:remote> [-n namespace]
  Forward local port to pod port
  Example: /portforward my-pod 8080:80

*Context Commands:*
/contexts
  List all available kubeconfig contexts

/use-context <context-name>
  Switch to a different context

/config
  Show current bot configuration

*Monitoring Commands:*
/top pods [-n namespace] [--sort cpu|memory]
  Show pod resource usage (requires metrics-server)

/top nodes [--sort cpu|memory]
  Show node resource usage

/events [-n namespace] [--since TIME]
  Show recent events

/watch <resource> [name] [-n namespace]
  Watch for resource changes (sends updates)
  Example: /watch pods

*Operations:*
/restart deployment <name> [-n namespace]
  Restart a deployment by updating annotation

/scale deployment <name> <replicas> [-n namespace]
  Scale a deployment to specified replicas
  Example: /scale deployment my-app 3

*Global Flags:*
-n, --namespace      Namespace (default: from session or 'default')
-o, --output         Output format: json, yaml, wide, name
-l, --selector       Label selector
-A, --all-namespaces All namespaces

*Interactive Features:*
• Inline keyboards for namespace/context switching
• Callback buttons for common actions
• Session-based namespace context

*Examples:*
  /get pods -n production -o wide
  /logs my-pod -c app -f --tail 50
  /exec my-pod -- /bin/bash
  /scale deployment frontend 5 -n prod`

	h.bot.SendMarkdown(msg.Chat.ID, help)
	return nil
}