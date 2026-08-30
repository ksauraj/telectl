package bot

import (
	"strings"
	"testing"

	"github.com/ksauraj/telectl/internal/utils/formatters"
)

// TestFollowLogsTruncatesToFitPane is the regression test for the
// MESSAGE_TOO_LONG bug: showFollowLogs used to pass 200 lines of raw logs
// straight to editView with no truncation, so Telegram rejected the edit and
// the follow silently froze while the updater goroutine retried (and failed)
// every 5s forever. The fix routes follow-logs through truncateForPaneTail, so
// the rendered body must always fit Telegram's limit.
func TestFollowLogsTruncatesToFitPane(t *testing.T) {
	// A pod producing many long lines yields a RichLogs body that comfortably
	// exceeds paneLimit.
	long := formatters.RichLogs("default/big-pod", "app",
		"line "+strings.Repeat("x", 5000)+"\n")
	if len(long) <= paneLimit {
		t.Fatalf("test fixture is not long enough: %d bytes (need > %d)", len(long), paneLimit)
	}

	got := truncateForPaneTail(long)
	// The truncation note adds a little overhead; allow a modest margin but the
	// result must stay well under Telegram's 4096-byte hard cap.
	if len(got) > paneLimit+200 {
		t.Fatalf("truncated log body is %d bytes, want <= %d", len(got), paneLimit+200)
	}
	// The tail (newest line) must survive — tail truncation keeps the end.
	if !strings.Contains(got, "xxxx") {
		t.Fatalf("truncated body dropped the log tail: %q", got)
	}
}
