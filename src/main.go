package main

import (
	_ "embed"
	"encoding/json"
	"strings"
	"time"

	"github.com/branchkit/plugin-sdk-go"
)

//go:embed settings.css
var snippetsCSS string

// Expansions longer than this, or containing newlines, go through the
// clipboard-paste path: keystroke-typing a long expansion is slow and
// fragile, and newline delivery varies by app.
const pasteThreshold = 200

// How long the paste keystroke gets to consume the clipboard before the
// saved contents are restored. Restoring too early races the paste and
// the target app receives the OLD clipboard.
const pasteSettle = 300 * time.Millisecond

func main() {
	h := newHost(branchkit.NewPlugin())

	// Two ways in, one source of truth. The voice capture arrives with
	// `text` pre-resolved (the matcher read the collection's value_field);
	// every other trigger — keybinds, HUD buttons, another plugin's
	// dispatch — sends `name` and the CURRENT expansion is read from the
	// collection here, so nothing ever carries a stale copy of a snippet.
	HandleType(h.plugin, func(p TypeParams, _ *branchkit.OnActionRequest) (any, error) {
		text := ""
		if p.Text != nil {
			text = *p.Text
		}
		if p.Name != nil && *p.Name != "" {
			resolved, err := h.lookupExpansion(*p.Name)
			if err != nil {
				return nil, err
			}
			text = resolved
		}
		text = expandTokens(text, time.Now())
		if text == "" {
			return nil, nil
		}
		if needsPaste(text) {
			return nil, h.pasteText(text)
		}
		return nil, h.plugin.InputTypeText(text)
	})

	// The Import settings tab (docs/design/DESIGN_SELECTION_PRIMITIVE.md, step
	// 3c): paste a pack, name it, done. Its snippets arrive as selection
	// targets, not vocabulary.
	h.plugin.SettingsCSS(snippetsCSS)
	h.plugin.SettingsTab("import", func(_ *branchkit.RenderSettingsRequest) (string, error) {
		return h.renderImportSettings()
	})

	branchkit.HandleCommand(h.plugin, "import_pack", h.handleImportPack)
	branchkit.HandleCommand(h.plugin, "remove_pack", h.handleRemovePack)

	h.plugin.Run()
}

// lookupExpansion resolves a snippet by its spoken name — records are keyed
// by it — reading the merged collection, so user edits in Settings and pack
// contributions reach by-name triggers immediately.
func (h *Host) lookupExpansion(name string) (string, error) {
	rec, err := h.plugin.Get("snippets", name)
	if err != nil {
		return "", err
	}
	if rec == nil {
		return "", nil
	}
	var payload struct {
		Expansion string `json:"expansion"`
	}
	if err := json.Unmarshal(rec.Payload, &payload); err != nil {
		return "", err
	}
	return payload.Expansion, nil
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
func (h *Host) pasteText(text string) error {
	_, _ = h.plugin.AssertEffect("signal_clipboard_in_use")
	defer func() { _, _, _ = h.plugin.RetractEffect("signal_clipboard_in_use") }()

	saved := ""
	restorable := false
	if r, err := h.plugin.InputClipboardRead("text"); err == nil && r != nil && r.Text != nil {
		saved, restorable = *r.Text, true
	}

	if err := h.plugin.InputClipboardAction("set", &text); err != nil {
		return err
	}
	if err := h.plugin.InputClipboardAction("paste", nil); err != nil {
		return err
	}

	if restorable {
		time.Sleep(pasteSettle)
		return h.plugin.InputClipboardAction("set", &saved)
	}
	return nil
}
