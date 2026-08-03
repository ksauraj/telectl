package handlers

import (
	"context"

	"github.com/ksauraj/telectl/internal/tg"
	"github.com/ksauraj/telectl/internal/types"
)

type HelpHandler struct {
	*BaseHandler
}

func NewHelpHandler(b types.BotInterface) *HelpHandler {
	return &HelpHandler{BaseHandler: NewBaseHandler(b)}
}

func (h *HelpHandler) Handle(ctx context.Context, msg *tg.Message, args []string, session *types.UserSession) error {
	help := `📚 <b>telectl Command Reference</b>

<b>Resource Commands:</b>
/get &lt;resource&gt; [name] [-n namespace] [-o json|yaml|wide] [-l selector]
  Resources: pods, deployments, services, replicasets, namespaces, nodes, configmaps, secrets, pvcs, pvs, ingresses
  Examples:
    /get pods
    /get pods -n kube-system
    /get deployment my-app -o wide
    /get pods -l app=nginx

/describe &lt;resource&gt; &lt;name&gt; [-n namespace]
  Get detailed information about a resource
  Example: /describe pod my-pod -n default

/logs &lt;pod&gt; [-n namespace] [-c container] [-f] [-p] [--tail N] [--since TIME]
  View pod logs
  Flags:
    -c, --container    Container name
    -f, --follow       Follow log output
    -p, --previous     Show previous container logs
    --tail N           Lines from end of log
    --since TIME       Show logs since timestamp (e.g., 1h, 30m)
  Example: /logs my-pod -c nginx -f --tail 100

/exec &lt;pod&gt; [-n namespace] [-c container] [command]
  Execute a command in a container
  If no command provided, starts interactive session
  Example: /exec my-pod -c nginx -- ls -la

/portforward &lt;pod&gt; &lt;local:remote&gt; [-n namespace]
  Forward local port to pod port
  Example: /portforward my-pod 8080:80

<b>Context Commands:</b>
/contexts
  List all available kubeconfig contexts

/use-context &lt;context-name&gt;
  Switch to a different context

/config
  Show current bot configuration

<b>Monitoring Commands:</b>
/top pods [-n namespace] [--sort cpu|memory]
  Show pod resource usage (requires metrics-server)

/top nodes [--sort cpu|memory]
  Show node resource usage

/events [-n namespace] [--since TIME]
  Show recent events

/watch &lt;resource&gt; [name] [-n namespace]
  Watch for resource changes (sends updates)
  Example: /watch pods

<b>Operations:</b>
/restart deployment &lt;name&gt; [-n namespace]
  Restart a deployment by updating annotation

/scale deployment &lt;name&gt; &lt;replicas&gt; [-n namespace]
  Scale a deployment to specified replicas
  Example: /scale deployment my-app 3

<b>Global Flags:</b>
-n, --namespace      Namespace (default: from session or 'default')
-o, --output         Output format: json, yaml, wide, name
-l, --selector       Label selector
-A, --all-namespaces All namespaces

<b>Interactive Features:</b>
• Inline keyboards for namespace/context switching
• Callback buttons for common actions
• Session-based namespace context

<b>Examples:</b>
  /get pods -n production -o wide
  /logs my-pod -c app -f --tail 50
  /exec my-pod -- /bin/bash
  /scale deployment frontend 5 -n prod`

	h.bot.SendHTML(msg.Chat.ID, help)
	return nil
}
