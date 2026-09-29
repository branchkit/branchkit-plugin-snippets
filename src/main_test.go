package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/branchkit/plugin-sdk-go/harness"
)

// Unlike a capture against another plugin's collection, <snippets> is
// self-seeded from this manifest's collection_data, so the whole loop —
// seed → grammar → capture → value_field resolution → action params — is
// testable right here.

func TestSnippetResolvesExpansion(t *testing.T) {
	h := harness.Start(t, "..")
	result := h.MustSimulateCommand("snippet shrug")
	if result.ActionType() != "snippets.type" {
		t.Fatalf("expected action type %q, got %q", "snippets.type", result.ActionType())
	}
	var params struct {
		Text string `json:"text"`
	}
	if err := result.ActionParams(&params); err != nil {
		t.Fatalf("ActionParams: %v", err)
	}
	if params.Text != `¯\_(ツ)_/¯` {
		t.Fatalf("value_field should resolve the spoken key to the expansion, got %q", params.Text)
	}
}

func TestMultiwordSpokenName(t *testing.T) {
	h := harness.Start(t, "..")
	result := h.MustSimulateCommand("snippet table flip")
	var params struct {
		Text string `json:"text"`
	}
	if err := result.ActionParams(&params); err != nil {
		t.Fatalf("ActionParams: %v", err)
	}
	if params.Text != "(╯°□°)╯︵ ┻━┻" {
		t.Fatalf("multiword spoken names must resolve too, got %q", params.Text)
	}
}

// The keybind path: a trigger that carries only `name` gets the CURRENT
// expansion, read from the collection when it fires. The harness stubs the
// platform's input.type_text and logs the call.
func TestByNameTypesTheLiveRecord(t *testing.T) {
	h := harness.Start(t, "..")
	var resp struct {
		Status string `json:"status"`
	}
	h.CallPlugin("on_action", map[string]any{
		"action": "snippets.type",
		"params": map[string]any{"name": "table flip"},
	}, &resp)
	if resp.Status != "ok" {
		t.Fatalf("on_action status %q", resp.Status)
	}
	for _, call := range h.GetRpcLog() {
		if call.Method != "input.type_text" {
			continue
		}
		var params struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(call.Params, &params); err != nil {
			t.Fatal(err)
		}
		if params.Text != "(╯°□°)╯︵ ┻━┻" {
			t.Fatalf("typed %q, want the record's expansion", params.Text)
		}
		return
	}
	t.Fatal("no input.type_text call — the by-name lookup found nothing")
}

func TestTokensExpand(t *testing.T) {
	at := time.Date(2026, 9, 5, 14, 30, 0, 0, time.Local)
	got := expandTokens("Notes — {{date}} at {{time}}", at)
	want := "Notes — 2026-09-05 at 14:30"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if expandTokens("no tokens here", at) != "no tokens here" {
		t.Fatal("token-free text must pass through untouched")
	}
}

func TestPasteRouting(t *testing.T) {
	if needsPaste("short") {
		t.Fatal("short single-line text types directly")
	}
	if !needsPaste("line one\nline two") {
		t.Fatal("multi-line text must take the paste path")
	}
	if !needsPaste(strings.Repeat("x", pasteThreshold+1)) {
		t.Fatal("long text must take the paste path")
	}
}

// "snippet" alone is a command too: with `discovery: "select"` the bare verb
// opens the codeword browse. So an unknown name still matches the verb, and
// what must not happen is a snippets.type action.
func TestUnknownSnippetDoesNotMatch(t *testing.T) {
	h := harness.Start(t, "..")
	result, err := h.TrySimulateCommand("snippet nonsense")
	if err != nil {
		t.Fatalf("TrySimulateCommand: %v", err)
	}
	if result.ActionType() == "snippets.type" {
		t.Fatal("a name absent from the collection must not type anything — the grammar is closed")
	}
}
