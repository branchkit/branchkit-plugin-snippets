package main

// Pack import (docs/design/DESIGN_SELECTION_PRIMITIVE.md, step 3c): paste a
// pack, name it, and its snippets arrive as SELECTION TARGETS — category =
// pack identity, speakable = false. Import is a parse problem, not a
// vocabulary problem: nothing here touches the grammar, because the
// collection's `grammar_only_when: "speakable"` keeps unpromoted names out
// of the engine no matter how many records land. Re-importing under the
// same name replaces the pack (update); removing the category uninstalls it.
//
// Formats (verified against espanso.org/docs/matches/basics, 2026-09-05):
//   - espanso YAML: `matches:` list (or a bare list) of
//     {trigger|triggers, label?, replace}. The spoken-ish name is the label
//     when present, else the trigger stripped of punctuation. Matches whose
//     replace carries variables this plugin cannot expand ({{anything}}
//     other than {{date}}/{{time}}) are skipped and counted — importing
//     them would type literal template braces.
//   - CSV: name,expansion rows (a leading header row is detected and
//     skipped). encoding/csv, so quoting and embedded newlines work.
//   - JSON: an array of objects; name from spoken|name|trigger|label,
//     expansion from expansion|text|replace.
// Detection: JSON by first byte, espanso when YAML parses to matches,
// CSV otherwise. The result line names what was detected — never silent.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/branchkit/plugin-sdk-go"
	"gopkg.in/yaml.v3"
)

// Categories seeded from the manifest's collection_data. Their records
// re-contribute at every boot, so a "remove pack" on them would silently
// resurrect — the control is withheld rather than offered dishonestly.
var builtinCategories = map[string]bool{
	"fun": true, "typography": true, "dynamic": true,
}

var (
	importMu         sync.Mutex
	lastImportResult string
)

type importedSnippet struct {
	Spoken    string `json:"spoken"`
	Expansion string `json:"expansion"`
}

type importOutcome struct {
	format   string
	imported []importedSnippet
	skipped  []string // human-readable reasons, one per skipped entry
}

// unsupportedToken matches {{...}} occurrences that this plugin's
// expandTokens cannot resolve.
var tokenRe = regexp.MustCompile(`\{\{\s*([^}]*?)\s*\}\}`)

func unsupportedTokens(s string) []string {
	var out []string
	for _, m := range tokenRe.FindAllStringSubmatch(s, -1) {
		if m[1] != "date" && m[1] != "time" {
			out = append(out, m[1])
		}
	}
	return out
}

// --- espanso ---------------------------------------------------------------

type espansoMatch struct {
	Trigger  string   `yaml:"trigger"`
	Triggers []string `yaml:"triggers"`
	Label    string   `yaml:"label"`
	Replace  string   `yaml:"replace"`
}

type espansoFile struct {
	Matches []espansoMatch `yaml:"matches"`
}

// espansoName derives the spoken-ish display name: the label when present,
// else the trigger stripped of the punctuation espanso triggers ride on
// (":sig", ";date") — names are labels under selection, so no further
// curation is needed or attempted.
func espansoName(m *espansoMatch) string {
	if strings.TrimSpace(m.Label) != "" {
		return strings.TrimSpace(m.Label)
	}
	t := m.Trigger
	if t == "" && len(m.Triggers) > 0 {
		t = m.Triggers[0]
	}
	return strings.Trim(t, ":;/\\ \t")
}

func parseEspanso(text string) ([]espansoMatch, bool) {
	var file espansoFile
	if err := yaml.Unmarshal([]byte(text), &file); err == nil && len(file.Matches) > 0 {
		return file.Matches, true
	}
	var bare []espansoMatch
	if err := yaml.Unmarshal([]byte(text), &bare); err == nil && len(bare) > 0 {
		// A YAML list of anything decodes; require match-shaped entries.
		for _, m := range bare {
			if m.Trigger != "" || len(m.Triggers) > 0 || m.Replace != "" {
				return bare, true
			}
		}
	}
	return nil, false
}

// --- JSON ------------------------------------------------------------------

func parseJSONPack(text string) ([]importedSnippet, []string, bool) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil, nil, false
	}
	str := func(row map[string]any, keys ...string) string {
		for _, k := range keys {
			if v, ok := row[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}
	var out []importedSnippet
	var skipped []string
	for i, row := range rows {
		name := str(row, "spoken", "name", "trigger", "label")
		exp := ""
		for _, k := range []string{"expansion", "text", "replace"} {
			if v, ok := row[k].(string); ok && v != "" {
				exp = v
				break
			}
		}
		if name == "" || exp == "" {
			skipped = append(skipped, fmt.Sprintf("row %d: missing name or expansion", i+1))
			continue
		}
		out = append(out, importedSnippet{Spoken: name, Expansion: exp})
	}
	return out, skipped, true
}

// --- CSV -------------------------------------------------------------------

func parseCSVPack(text string) ([]importedSnippet, []string) {
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, []string{fmt.Sprintf("CSV parse error: %v", err)}
	}
	var out []importedSnippet
	var skipped []string
	for i, row := range rows {
		if len(row) < 2 {
			skipped = append(skipped, fmt.Sprintf("row %d: need name,expansion columns", i+1))
			continue
		}
		name, exp := strings.TrimSpace(row[0]), row[1]
		if i == 0 && isCSVHeader(name, exp) {
			continue
		}
		if name == "" || exp == "" {
			skipped = append(skipped, fmt.Sprintf("row %d: empty name or expansion", i+1))
			continue
		}
		out = append(out, importedSnippet{Spoken: name, Expansion: exp})
	}
	return out, skipped
}

func isCSVHeader(a, b string) bool {
	names := map[string]bool{"name": true, "spoken": true, "trigger": true}
	exps := map[string]bool{"expansion": true, "text": true, "replace": true}
	return names[strings.ToLower(a)] && exps[strings.ToLower(strings.TrimSpace(b))]
}

// --- dispatch --------------------------------------------------------------

func parsePack(text string) importOutcome {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		if rows, skipped, ok := parseJSONPack(trimmed); ok {
			return importOutcome{format: "JSON", imported: rows, skipped: skipped}
		}
	}
	if matches, ok := parseEspanso(text); ok {
		var out importOutcome
		out.format = "espanso YAML"
		for i, m := range matches {
			m := m
			name := espansoName(&m)
			switch {
			case name == "":
				out.skipped = append(out.skipped, fmt.Sprintf("match %d: no label or trigger", i+1))
			case m.Replace == "":
				out.skipped = append(out.skipped,
					fmt.Sprintf("match %d (%q): no replace text (forms and scripts don't import)", i+1, name))
			case len(unsupportedTokens(m.Replace)) > 0:
				out.skipped = append(out.skipped,
					fmt.Sprintf("match %d (%q): uses espanso variables (%s) this plugin can't expand",
						i+1, name, strings.Join(unsupportedTokens(m.Replace), ", ")))
			default:
				out.imported = append(out.imported, importedSnippet{Spoken: name, Expansion: m.Replace})
			}
		}
		return out
	}
	rows, skipped := parseCSVPack(text)
	return importOutcome{format: "CSV", imported: rows, skipped: skipped}
}

// --- import / remove -------------------------------------------------------

type importPackRequest struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

type removePackRequest struct {
	Name string `json:"name"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

func setResult(msg string) {
	importMu.Lock()
	lastImportResult = msg
	importMu.Unlock()
}

func handleImportPack(req *importPackRequest) (any, error) {
	pack := strings.TrimSpace(req.Name)
	if pack == "" {
		setResult("A pack needs a name — it becomes the category every imported snippet carries.")
		return okResponse{OK: false}, nil
	}
	if builtinCategories[pack] {
		setResult(fmt.Sprintf("%q is a built-in category — pick another pack name.", pack))
		return okResponse{OK: false}, nil
	}
	if strings.TrimSpace(req.Text) == "" {
		setResult("Nothing to import — paste a pack first.")
		return okResponse{OK: false}, nil
	}

	outcome := parsePack(req.Text)
	if len(outcome.imported) == 0 {
		reasons := ""
		if len(outcome.skipped) > 0 {
			reasons = " " + strings.Join(firstN(outcome.skipped, 3), "; ")
		}
		setResult(fmt.Sprintf("Detected %s, but nothing imported.%s", outcome.format, reasons))
		return okResponse{OK: false}, nil
	}

	// Replace-on-reimport: the pack's previous records go first, so a
	// re-import IS the update path and removals in the source propagate.
	existing, err := plugin.ListAll("snippets")
	if err != nil {
		return nil, err
	}
	var stale []string
	for _, rec := range existing {
		var p struct {
			Category string `json:"category"`
		}
		if json.Unmarshal(rec.Payload, &p) == nil && p.Category == pack {
			stale = append(stale, rec.ID)
		}
	}
	if len(stale) > 0 {
		if _, _, err := plugin.DeleteMany("snippets", stale); err != nil {
			return nil, err
		}
	}

	// Dedupe within the pack (last wins) and write. Records arrive
	// UNPROMOTED: names are labels, the codeword browse addresses them, and
	// the Speakable toggle in Settings is the only way one earns a grammar
	// slot.
	seen := map[string]int{}
	entries := make([]branchkit.CollectionPutEntry, 0, len(outcome.imported))
	dupes := 0
	for _, snip := range outcome.imported {
		payload, _ := json.Marshal(map[string]any{
			"spoken":    snip.Spoken,
			"expansion": snip.Expansion,
			"category":  pack,
			"speakable": false,
		})
		if i, dup := seen[snip.Spoken]; dup {
			entries[i] = branchkit.CollectionPutEntry{ID: snip.Spoken, Payload: payload}
			dupes++
			continue
		}
		seen[snip.Spoken] = len(entries)
		entries = append(entries, branchkit.CollectionPutEntry{ID: snip.Spoken, Payload: payload})
	}
	if _, err := plugin.PutMany("snippets", entries); err != nil {
		return nil, err
	}

	msg := fmt.Sprintf("Imported %d snippet(s) into pack %q (%s", len(entries), pack, outcome.format)
	if len(stale) > 0 {
		msg += fmt.Sprintf("; replaced the pack's previous %d", len(stale))
	}
	if dupes > 0 {
		msg += fmt.Sprintf("; %d duplicate name(s), last won", dupes)
	}
	msg += ")."
	if len(outcome.skipped) > 0 {
		msg += fmt.Sprintf(" Skipped %d: %s", len(outcome.skipped),
			strings.Join(firstN(outcome.skipped, 5), "; "))
	}
	setResult(msg)
	return okResponse{OK: true}, nil
}

func handleRemovePack(req *removePackRequest) (any, error) {
	pack := strings.TrimSpace(req.Name)
	if pack == "" || builtinCategories[pack] {
		return okResponse{OK: false}, nil
	}
	existing, err := plugin.ListAll("snippets")
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, rec := range existing {
		var p struct {
			Category string `json:"category"`
		}
		if json.Unmarshal(rec.Payload, &p) == nil && p.Category == pack {
			ids = append(ids, rec.ID)
		}
	}
	if len(ids) > 0 {
		if _, _, err := plugin.DeleteMany("snippets", ids); err != nil {
			return nil, err
		}
	}
	setResult(fmt.Sprintf("Removed pack %q (%d snippet(s)).", pack, len(ids)))
	return okResponse{OK: true}, nil
}

func firstN(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string{}, xs[:n]...), fmt.Sprintf("… and %d more", len(xs)-n))
}

// --- render ----------------------------------------------------------------

func renderImportSettings() string {
	type packView struct {
		Name    string
		Count   int
		Builtin bool
	}
	counts := map[string]int{}
	if records, err := plugin.ListAll("snippets"); err == nil {
		for _, rec := range records {
			var p struct {
				Category string `json:"category"`
			}
			if json.Unmarshal(rec.Payload, &p) == nil && p.Category != "" {
				counts[p.Category]++
			}
		}
	}
	packs := make([]packView, 0, len(counts))
	for name, n := range counts {
		packs = append(packs, packView{Name: name, Count: n, Builtin: builtinCategories[name]})
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].Name < packs[j].Name })

	importMu.Lock()
	result := lastImportResult
	importMu.Unlock()

	var b strings.Builder
	b.WriteString(`<div style="max-width: 720px;" data-signals:packname="''" data-signals:packtext="''">`)
	b.WriteString(`<h2 style="font-size: 16px; margin: 0 0 6px;">Import a pack</h2>`)
	b.WriteString(`<p style="font-size: 12px; color: var(--text-secondary, #888); margin: 0 0 12px; line-height: 1.5;">` +
		`Paste an espanso match file, CSV (name,expansion), or a JSON array. ` +
		`Imported snippets are picked by letter code from the &ldquo;snippet&rdquo; browse &mdash; ` +
		`their names stay out of the recognition vocabulary until you mark one Speakable in ` +
		`Collections &rsaquo; snippets. Re-importing under the same pack name replaces the pack.</p>`)
	b.WriteString(`<input type="text" placeholder="Pack name (becomes the category)" data-bind:packname ` +
		`style="width: 100%; box-sizing: border-box; padding: 8px 10px; margin-bottom: 8px; border-radius: 6px; ` +
		`border: 1px solid var(--border, #333); background: var(--bg-input, #0a0a0a); color: var(--text, #e0e0e0); font-size: 13px;">`)
	b.WriteString(`<textarea rows="10" placeholder="Paste the pack here&hellip;" data-bind:packtext ` +
		`style="width: 100%; box-sizing: border-box; padding: 8px 10px; border-radius: 6px; font-family: monospace; ` +
		`border: 1px solid var(--border, #333); background: var(--bg-input, #0a0a0a); color: var(--text, #e0e0e0); font-size: 12px;"></textarea>`)
	b.WriteString(`<div style="margin-top: 8px;"><button style="padding: 6px 14px; border-radius: 6px; cursor: pointer; ` +
		`border: 1px solid var(--border, #333); background: var(--accent, #4a9eff); color: #fff; font-size: 13px;" ` +
		`data-on:click="` + html.EscapeString(branchkit.MethodPost("import_pack", "{name: $packname, text: $packtext}")) + `">Import</button></div>`)
	if result != "" {
		b.WriteString(`<div style="margin-top: 10px; font-size: 12px; color: var(--text-secondary, #aaa); line-height: 1.5;">` +
			html.EscapeString(result) + `</div>`)
	}

	b.WriteString(`<h2 style="font-size: 16px; margin: 24px 0 6px;">Packs</h2>`)
	if len(packs) == 0 {
		b.WriteString(`<p style="font-size: 12px; color: var(--text-secondary, #888);">No categorized snippets yet.</p>`)
	}
	for _, p := range packs {
		nameJSON, _ := json.Marshal(p.Name)
		b.WriteString(`<div style="display: flex; align-items: center; gap: 10px; padding: 6px 0; ` +
			`border-bottom: 1px solid rgba(255,255,255,0.06); font-size: 13px;">`)
		b.WriteString(`<span style="flex: 1;">` + html.EscapeString(p.Name) + `</span>`)
		b.WriteString(fmt.Sprintf(`<span style="color: var(--text-secondary, #888);">%d snippet(s)</span>`, p.Count))
		if p.Builtin {
			b.WriteString(`<span style="font-size: 11px; color: var(--text-secondary, #666);" ` +
				`title="Shipped with the plugin — its snippets reload at startup, so removing the pack here would not stick.">built-in</span>`)
		} else {
			b.WriteString(`<button style="font-size: 11px; padding: 2px 10px; border-radius: 4px; cursor: pointer; ` +
				`border: 1px solid var(--border, #333); background: transparent; color: var(--text-secondary, #aaa);" ` +
				`data-on:click="` + html.EscapeString("if (confirm('Remove pack "+html.EscapeString(p.Name)+" and its snippets?')) "+
				branchkit.MethodPost("remove_pack", "{name: "+string(nameJSON)+"}")) + `">Remove</button>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
