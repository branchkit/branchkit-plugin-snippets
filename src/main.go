package main

import (
	"strings"
	"time"

	"github.com/branchkit/plugin-sdk-go"
)

// Expansions longer than this, or containing newlines, go through the
// clipboard-paste path: keystroke-typing a long expansion is slow and
// fragile, and newline delivery varies by app.
const pasteThreshold = 200

// How long the paste keystroke gets to consume the clipboard before the
// saved contents are restored. Restoring too early races the paste and
// the target app receives the OLD clipboard.
const pasteSettle = 300 * time.Millisecond

var plugin *branchkit.Plugin

func main() {
	plugin = branchkit.NewPlugin()

	// The matcher resolves the <snippets> capture through the collection's
	// value_field, so the param already carries the expansion — the spoken
	// key never reaches this handler.
	HandleType(plugin, func(p TypeParams, _ *branchkit.OnActionRequest) (any, error) {
		text := expandTokens(p.Text, time.Now())
		if text == "" {
			return nil, nil
		}
		if needsPaste(text) {
			return nil, pasteText(text)
		}
		return nil, plugin.InputTypeText(text)
	})

	plugin.Run()
}

// expandTokens resolves the dynamic {{...}} tokens an expansion may carry —
// usable in the seeded "today"/"time now" snippets and inside any snippet a
// user writes ("Meeting notes — {{date}}").
func expandTokens(s string, now time.Time) string {
	s = strings.ReplaceAll(s, "{{date}}", now.Format("2006-01-02"))
	s = strings.ReplaceAll(s, "{{time}}", now.Format("15:04"))
	return s
}

func needsPaste(s string) bool {
	return len(s) > pasteThreshold || strings.Contains(s, "\n")
}

// pasteText delivers a long or multi-line expansion via the clipboard:
// set, paste, and — when the user granted the OPTIONAL `clipboard` read
// privilege — restore what the clipboard held. Without that grant the
// paste still works and the clipboard keeps the expansion; the README
// documents the trade. The signal_clipboard_in_use effect announces the
// dance so clipboard-aware plugins can stand off.
func pasteText(text string) error {
	_, _ = plugin.AssertEffect("signal_clipboard_in_use")
	defer func() { _, _, _ = plugin.RetractEffect("signal_clipboard_in_use") }()

	saved := ""
	restorable := false
	if r, err := plugin.InputClipboardRead("text"); err == nil && r != nil && r.Text != nil {
		saved, restorable = *r.Text, true
	}

	if err := plugin.InputClipboardAction("set", &text); err != nil {
		return err
	}
	if err := plugin.InputClipboardAction("paste", nil); err != nil {
		return err
	}

	if restorable {
		time.Sleep(pasteSettle)
		return plugin.InputClipboardAction("set", &saved)
	}
	return nil
}
