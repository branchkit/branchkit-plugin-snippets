package main

import (
	"strings"
	"testing"
)

func TestParseEspansoMatchesKey(t *testing.T) {
	out := parsePack(`
matches:
  - trigger: ":sig"
    replace: "Best regards,\nDrew"
  - trigger: ":tomorrow"
    label: Insert tomorrow's date
    replace: "{{mytime}}"
  - triggers: [":hi", ":hello"]
    replace: "hey there"
`)
	if out.format != "espanso YAML" {
		t.Fatalf("format = %q", out.format)
	}
	if len(out.imported) != 2 {
		t.Fatalf("imported = %+v", out.imported)
	}
	// Trigger punctuation stripped; label wins when present (the var-bearing
	// match is skipped regardless); multi-trigger takes the first.
	if out.imported[0].Spoken != "sig" || !strings.Contains(out.imported[0].Expansion, "\n") {
		t.Errorf("sig: %+v", out.imported[0])
	}
	if out.imported[1].Spoken != "hi" {
		t.Errorf("multi-trigger: %+v", out.imported[1])
	}
	if len(out.skipped) != 1 || !strings.Contains(out.skipped[0], "mytime") {
		t.Errorf("var-bearing match must be skipped with its variable named: %v", out.skipped)
	}
}

func TestParseEspansoBareListAndSupportedTokens(t *testing.T) {
	out := parsePack(`
- trigger: ":today"
  replace: "It is {{date}}"
- trigger: ":form"
`)
	if out.format != "espanso YAML" {
		t.Fatalf("format = %q", out.format)
	}
	if len(out.imported) != 1 || out.imported[0].Expansion != "It is {{date}}" {
		t.Fatalf("{{date}} is a token this plugin expands — must import: %+v", out.imported)
	}
	if len(out.skipped) != 1 || !strings.Contains(out.skipped[0], "no replace") {
		t.Errorf("replace-less match skipped with reason: %v", out.skipped)
	}
}

func TestParseCSVWithHeaderAndQuoting(t *testing.T) {
	out := parsePack("name,expansion\nshrug,\"¯\\_(ツ)_/¯\"\nsig,\"line one\nline two\"\n")
	if out.format != "CSV" {
		t.Fatalf("format = %q", out.format)
	}
	if len(out.imported) != 2 {
		t.Fatalf("imported = %+v (skipped %v)", out.imported, out.skipped)
	}
	if !strings.Contains(out.imported[1].Expansion, "\n") {
		t.Errorf("quoted multiline expansion survives: %+v", out.imported[1])
	}
}

func TestParseJSONSynonyms(t *testing.T) {
	out := parsePack(`[
  {"spoken": "a", "expansion": "1"},
  {"name": "b", "text": "2"},
  {"trigger": ":c", "replace": "3"},
  {"nonsense": true}
]`)
	if out.format != "JSON" {
		t.Fatalf("format = %q", out.format)
	}
	if len(out.imported) != 3 || len(out.skipped) != 1 {
		t.Fatalf("imported %+v skipped %v", out.imported, out.skipped)
	}
	if out.imported[2].Spoken != ":c" {
		// JSON rows are taken as-authored — punctuation stripping is an
		// espanso-trigger convention, not a general rule.
		t.Errorf("json trigger kept verbatim: %+v", out.imported[2])
	}
}

func TestUnsupportedTokens(t *testing.T) {
	if got := unsupportedTokens("plain {{date}} and {{time}}"); len(got) != 0 {
		t.Errorf("supported tokens flagged: %v", got)
	}
	if got := unsupportedTokens("{{clipboard}} x {{form.name}}"); len(got) != 2 {
		t.Errorf("want both flagged, got %v", got)
	}
}
