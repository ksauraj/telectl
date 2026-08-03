package handlers

import (
	"testing"
	"time"
)

// The /logs flag parser had no direct coverage before it was extracted from
// Handle. These pin the kubectl-style spellings the help text advertises —
// each flag in both its separate-argument and its =-joined form.
func TestParseLogFlags(t *testing.T) {
	const pod, ns = "api-1", "default"

	t.Run("no flags", func(t *testing.T) {
		opts := parseLogFlags(pod, ns, []string{pod})
		if opts.PodName != pod || opts.Namespace != ns {
			t.Errorf("pod/namespace not carried through: %+v", opts)
		}
		if opts.Container != "" || opts.Follow || opts.Previous || opts.TailLines != nil {
			t.Errorf("unset flags should stay zero: %+v", opts)
		}
	})

	t.Run("container both spellings", func(t *testing.T) {
		for _, args := range [][]string{
			{pod, "-c", "sidecar"},
			{pod, "-c=sidecar"},
			{pod, "--container", "sidecar"},
			{pod, "--container=sidecar"},
		} {
			if got := parseLogFlags(pod, ns, args).Container; got != "sidecar" {
				t.Errorf("args %v: Container = %q, want sidecar", args, got)
			}
		}
	})

	t.Run("boolean flags", func(t *testing.T) {
		for _, args := range [][]string{{pod, "-f"}, {pod, "--follow"}} {
			if !parseLogFlags(pod, ns, args).Follow {
				t.Errorf("args %v: Follow not set", args)
			}
		}
		for _, args := range [][]string{{pod, "-p"}, {pod, "--previous"}} {
			if !parseLogFlags(pod, ns, args).Previous {
				t.Errorf("args %v: Previous not set", args)
			}
		}
		if !parseLogFlags(pod, ns, []string{pod, "--timestamps"}).Timestamps {
			t.Error("--timestamps not set")
		}
	})

	t.Run("tail", func(t *testing.T) {
		for _, args := range [][]string{{pod, "--tail", "50"}, {pod, "--tail=50"}} {
			opts := parseLogFlags(pod, ns, args)
			if opts.TailLines == nil || *opts.TailLines != 50 {
				t.Errorf("args %v: TailLines = %v, want 50", args, opts.TailLines)
			}
		}
	})

	t.Run("since", func(t *testing.T) {
		for _, args := range [][]string{{pod, "--since", "5m"}, {pod, "--since=5m"}} {
			opts := parseLogFlags(pod, ns, args)
			if opts.SinceSeconds == nil || *opts.SinceSeconds != int64(5*time.Minute/time.Second) {
				t.Errorf("args %v: SinceSeconds = %v, want 300", args, opts.SinceSeconds)
			}
		}
	})

	t.Run("combined", func(t *testing.T) {
		opts := parseLogFlags(pod, ns, []string{pod, "-c", "app", "-f", "--tail", "100", "--timestamps"})
		if opts.Container != "app" || !opts.Follow || opts.TailLines == nil ||
			*opts.TailLines != 100 || !opts.Timestamps {
			t.Errorf("combined flags mis-parsed: %+v", opts)
		}
	})

	// A malformed value is ignored rather than failing the command: the flag is
	// optional, and refusing to show logs over a bad --tail would be worse than
	// showing all of them.
	t.Run("malformed values are ignored", func(t *testing.T) {
		opts := parseLogFlags(pod, ns, []string{pod, "--tail", "abc", "--since", "notaduration"})
		if opts.TailLines != nil {
			t.Errorf("TailLines should stay nil for a non-numeric value, got %v", *opts.TailLines)
		}
		if opts.SinceSeconds != nil {
			t.Errorf("SinceSeconds should stay nil for a bad duration, got %v", *opts.SinceSeconds)
		}
	})

	// A flag at the very end with no value must not read past the slice.
	t.Run("dangling flag does not panic", func(t *testing.T) {
		for _, args := range [][]string{{pod, "-c"}, {pod, "--tail"}, {pod, "--since"}} {
			_ = parseLogFlags(pod, ns, args)
		}
	})
}
