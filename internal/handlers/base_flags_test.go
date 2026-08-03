package handlers

import (
	"reflect"
	"testing"
)

// parseFlags is shared by the resource commands and had no direct coverage
// before it was rewritten from a 22-branch switch into a flag table. These pin
// every spelling the help text advertises, in both separated and =-joined form.
func TestParseFlags(t *testing.T) {
	cases := []struct {
		name          string
		args          []string
		namespace     string
		output        string
		selector      string
		fieldSelector string
		remaining     []string
	}{
		{name: "empty", args: nil, remaining: []string{}},
		{name: "positional only", args: []string{"pods", "api-1"},
			remaining: []string{"pods", "api-1"}},

		{name: "namespace short separated", args: []string{"-n", "prod"},
			namespace: "prod", remaining: []string{}},
		{name: "namespace short joined", args: []string{"-n=prod"},
			namespace: "prod", remaining: []string{}},
		{name: "namespace long separated", args: []string{"--namespace", "prod"},
			namespace: "prod", remaining: []string{}},
		{name: "namespace long joined", args: []string{"--namespace=prod"},
			namespace: "prod", remaining: []string{}},

		{name: "output short", args: []string{"-o", "wide"},
			output: "wide", remaining: []string{}},
		{name: "output joined", args: []string{"-o=json"},
			output: "json", remaining: []string{}},
		{name: "output long joined", args: []string{"--output=yaml"},
			output: "yaml", remaining: []string{}},

		{name: "selector short", args: []string{"-l", "app=api"},
			selector: "app=api", remaining: []string{}},
		{name: "selector joined", args: []string{"-l=app=api"},
			selector: "app=api", remaining: []string{}},
		{name: "selector long", args: []string{"--selector", "app=api"},
			selector: "app=api", remaining: []string{}},

		{name: "field selector", args: []string{"--field-selector", "spec.nodeName=n1"},
			fieldSelector: "spec.nodeName=n1", remaining: []string{}},
		{name: "field selector joined", args: []string{"--field-selector=spec.nodeName=n1"},
			fieldSelector: "spec.nodeName=n1", remaining: []string{}},

		// -A clears any namespace already seen.
		{name: "all namespaces short", args: []string{"-n", "prod", "-A"},
			namespace: "", remaining: []string{}},
		{name: "all namespaces long", args: []string{"--namespace=prod", "--all-namespaces"},
			namespace: "", remaining: []string{}},

		{name: "everything together",
			args: []string{"pods", "-n", "prod", "-o", "wide", "-l", "app=api",
				"--field-selector", "status.phase=Running", "extra"},
			namespace: "prod", output: "wide", selector: "app=api",
			fieldSelector: "status.phase=Running",
			remaining:     []string{"pods", "extra"}},

		// A flag value must not be mistaken for a positional argument.
		{name: "value not treated as positional", args: []string{"get", "-n", "prod", "pods"},
			namespace: "prod", remaining: []string{"get", "pods"}},

		// Dangling flags: a half-typed command must not panic or swallow the
		// next argument.
		{name: "dangling namespace", args: []string{"pods", "-n"},
			remaining: []string{"pods"}},
		{name: "dangling output", args: []string{"-o"}, remaining: []string{}},

		// Unknown flags fall through as positional rather than being dropped.
		{name: "unknown flag kept", args: []string{"--not-a-flag", "pods"},
			remaining: []string{"--not-a-flag", "pods"}},

		// Last spelling wins when a flag is repeated.
		{name: "repeated flag last wins", args: []string{"-n", "a", "-n", "b"},
			namespace: "b", remaining: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ns, out, sel, fieldSel, rest := parseFlags(tc.args)
			if ns != tc.namespace {
				t.Errorf("namespace = %q, want %q", ns, tc.namespace)
			}
			if out != tc.output {
				t.Errorf("output = %q, want %q", out, tc.output)
			}
			if sel != tc.selector {
				t.Errorf("selector = %q, want %q", sel, tc.selector)
			}
			if fieldSel != tc.fieldSelector {
				t.Errorf("fieldSelector = %q, want %q", fieldSel, tc.fieldSelector)
			}
			if !reflect.DeepEqual(rest, tc.remaining) {
				t.Errorf("remaining = %#v, want %#v", rest, tc.remaining)
			}
		})
	}
}

// parseExecArgs decides both which container to enter and what command to run.
// Getting the boundary wrong would either swallow the user's command or pass
// the container flag into the pod as an argument.
func TestParseExecArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		container string
		command   []string
	}{
		// No command: interactive shell.
		{name: "pod only", args: []string{"api-1"},
			command: []string{"sh"}},
		{name: "container separated, no command", args: []string{"api-1", "-c", "sidecar"},
			container: "sidecar", command: []string{"sh"}},
		{name: "container joined, no command", args: []string{"api-1", "-c=sidecar"},
			container: "sidecar", command: []string{"sh"}},
		{name: "container long, no command", args: []string{"api-1", "--container", "sidecar"},
			container: "sidecar", command: []string{"sh"}},
		{name: "container long joined", args: []string{"api-1", "--container=sidecar"},
			container: "sidecar", command: []string{"sh"}},

		// With a command.
		{name: "command only", args: []string{"api-1", "ls", "-la"},
			command: []string{"ls", "-la"}},
		{name: "container then command", args: []string{"api-1", "-c", "sidecar", "ls", "-la"},
			container: "sidecar", command: []string{"ls", "-la"}},
		{name: "joined container then command", args: []string{"api-1", "-c=sidecar", "cat", "/etc/hosts"},
			container: "sidecar", command: []string{"cat", "/etc/hosts"}},

		// The command's own flags must reach the container untouched, not be
		// parsed as telectl flags.
		{name: "command flags passed through",
			args:    []string{"api-1", "sh", "-c", "echo hi"},
			command: []string{"sh", "-c", "echo hi"}},

		{name: "dangling container flag", args: []string{"api-1", "-c"},
			command: []string{"sh"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			container, command := parseExecArgs(tc.args)
			if container != tc.container {
				t.Errorf("container = %q, want %q", container, tc.container)
			}
			if !reflect.DeepEqual(command, tc.command) {
				t.Errorf("command = %#v, want %#v", command, tc.command)
			}
		})
	}
}
