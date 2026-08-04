package formatters

// HelpText is the command reference, shared by the /help command and the Help
// button so the two cannot drift.
//
// HTML, not MarkdownV2. This text is full of the characters MarkdownV2 requires
// escaping — parentheses, dots, hyphens, pipes — and a single unescaped one
// makes Telegram reject the whole message with a 400. That is exactly how /help
// was silently dead: the text used Markdown V1 `*bold*` but was sent as
// MarkdownV2, and the error was discarded.
//
// Angle brackets in placeholders must stay entity-escaped (&lt;pod&gt;), or
// Telegram parses them as unknown tags and rejects the message.
const HelpText = `<b>telectl command reference</b>

<b>Resources</b>
/get &lt;resource&gt; [name] [-n namespace] [-o json|yaml|wide] [-l selector]
  Resources: pods, deployments, services, replicasets, namespaces, nodes,
  configmaps, secrets, pvcs, pvs, ingresses, events
  Examples:
    /get pods
    /get pods -n kube-system
    /get deployment my-app -o wide
    /get pods -l app=nginx

/describe &lt;resource&gt; &lt;name&gt; [-n namespace]
  Full detail for one object.

/logs &lt;pod&gt; [-n namespace] [-c container] [-f] [-p] [--tail N] [--since TIME]
  -c, --container    Container name
  -f, --follow       Follow log output
  -p, --previous     Previous container's logs
  --tail N           Lines from the end
  --since TIME       Since a duration ago (1h, 30m)

/exec &lt;pod&gt; [-n namespace] [-c container] -- &lt;command&gt;
  Run a command in a container. Everything after -- goes to the container.

/portforward &lt;pod&gt; &lt;local:remote&gt; [-n namespace]

<b>Context</b>
/contexts                  List kubeconfig contexts
/use-context &lt;name&gt;        Switch context (this bot session only)
/config                    Show current configuration

<b>Monitoring</b>
/top pods [-n namespace] [--sort cpu|memory]
/top nodes [--sort cpu|memory]
/events [-n namespace] [--since TIME]
/watch &lt;resource&gt; [name] [-n namespace]

<b>Operations</b>
/restart deployment &lt;name&gt; [-n namespace]
/scale deployment &lt;name&gt; &lt;replicas&gt; [-n namespace]

<b>Global flags</b>
-n, --namespace      Namespace (default: session, else 'default')
-o, --output         json, yaml, wide, name
-l, --selector       Label selector
-A, --all-namespaces All namespaces

<b>Menus</b>
Buttons operate a single pane: each tap replaces what the pane shows rather
than posting a new message, so the keyboard stays put. Every view has a way
back. Send /start to open a fresh pane.`
