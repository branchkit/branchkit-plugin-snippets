# Snippets

Speak a snippet's name to type its expansion at the cursor: say
**"snippet shrug"** and `¯\_(ツ)_/¯` appears wherever you're typing. A plugin
for [BranchKit](https://github.com/branchkit), an accessibility plugin
platform for the desktop, and its teaching plugin: about as little code as a
real plugin takes. MIT licensed.

BranchKit is pre-launch: the app is not publicly released yet.

## What you can say and press

| Trigger | Does |
|---|---|
| `snippet <name>` | Type that snippet (`snippet shrug`, `snippet table flip`) |
| `snippet`, then the letter code the Discovery HUD shows | Type the snippet with that code; works for every snippet, spoken name or not |
| `Alt+Shift+S` | Type `shrug` (a default hotkey; change it in Settings → Keybinds) |

Default snippets: `shrug`, `table flip`, `disapproval`, `long dash`,
`check mark`, `today`, `time now`. Add your own in Settings (the `snippets`
collection); a snippet marked **speakable** answers to its name, the rest are
reached by codeword.

## The tutorial

The BranchKit guide's "Build Snippets" page builds this plugin step by step
from the `branchkit-cli dev init` scaffold. It quotes the source at tag
[`v0.5.0`](https://github.com/branchkit/branchkit-plugin-snippets/tree/v0.5.0).
The documentation site is not public yet; until it is, this README and the
source are the tutorial.

Since `v0.5.0`, `main` has two manifest changes: the default hotkey moved to
`collection_data["_platform.bindings"]` (the platform's hotkey table, where it
was a `keybinds` entry before), and `requires.reasons` says why each privilege
is asked for.

## How it works

Four files carry the core. Read them in this order.

**`plugin.json`** declares everything the platform needs to route to the
plugin, before any code runs:

- One action, `snippets.type`, with two optional fields: `text` (the text to
  type) and `name` (a snippet to look up).
- The `snippets` collection. `feeds_matching` makes its records a capture the
  matcher understands, keyed by `spoken`, resolving to `expansion`
  (`value_field`). `grammar_only_when: "speakable"` keeps names out of the
  speech recogniser's grammar unless marked speakable, so a large imported
  pack costs nothing until you promote a name. `durable: true` keeps the
  records across an app restart: imported snippets exist nowhere else, so
  they cannot be re-published at boot like the seeded ones.
- `writers: anyone_who_declares` and `merge: collect`: other plugins can
  contribute records. A snippet pack is a manifest with
  `collection_data.snippets` and no code.
- `collection_data` seeds the default snippets, points at `commands.json`,
  and contributes the default hotkey to `_platform.bindings`.
- `requires`: the `input` privilege, the optional `clipboard` privilege, and
  a plain-language reason for each, shown on the plugin's card in Settings
  where the person grants them.

**`commands.json`** is one command: `snippet <snippets>`, dispatching
`snippets.type` with `text` set to the captured expansion.
`"discovery": "select"` puts every snippet on the Discovery HUD with a letter
code.

**`src/main.go`** registers one handler. Two ways in, one source of truth: a
voice capture arrives with `text` already resolved by the matcher; every other
trigger (the hotkey, another plugin's dispatch) sends `name`, and the handler
reads the current expansion from the collection, so no trigger carries a
stale copy.

```go
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
	return nil, h.plugin.InputTypeText(branchkit.InputTypeTextRequest{Text: text})
})
```

**`src/actions_gen.go`** is generated from `plugin.json` by
[branchkit-gen](https://github.com/branchkit/branchkit-gen): `TypeParams` and
`HandleType`, so the action string is never spelled in code and the params
cannot drift from the manifest. Every platform call goes through the SDK's
typed wrappers (`h.plugin.InputTypeText(...)`), never a raw `Call`.

## Beyond the core

- **Dynamic tokens.** `{{date}}` and `{{time}}` resolve when typed, in the
  seeded snippets and in any snippet you write ("Standup {{date}}").
- **Clipboard paste.** An expansion longer than 200 characters, or one with
  a newline, is pasted instead of typed. The plugin asserts the
  `signal_clipboard_in_use` effect around the paste, so clipboard-aware
  plugins can stand off. With the optional `clipboard` privilege granted, it
  restores what the clipboard held afterward; without it the paste still
  works and the clipboard keeps the expansion.
- **Import tab** (`src/import.go`). Paste an espanso YAML file, a CSV of
  `name,expansion` rows, or a JSON array, and name the pack. Imported
  snippets arrive as selection targets (`speakable: false`) grouped by pack;
  promote one by marking it speakable. Matches using espanso variables other
  than `{{date}}` and `{{time}}` are skipped and counted. Removing a pack
  deletes its records. The tab's buttons are wired with
  `branchkit.HandleCommand`.

## Permissions

| Privilege | Why |
|---|---|
| `input` (required) | To type a snippet where the cursor is |
| `clipboard` (optional) | To put your clipboard back after pasting a long snippet |

No network: the manifest declares no hosts, so the sandbox gives it none.

## Platform support

Nothing here is OS-specific: typing, pasting and the clipboard are platform
operations that BranchKit implements on macOS, Linux and Windows.

## Build

Go 1.25, [plugin-sdk-go](https://github.com/branchkit/plugin-sdk-go).

```bash
branchkit-cli dev build
```

That runs the manifest's `dev.build` recipe. The Import tab is a
[templ](https://templ.guide) template with its generated Go committed; run
`templ generate` in `src/` after editing `src/import.templ`.

## Test

```bash
cd src && go test ./...
```

The harness tests run a real matcher against the plugin: seed, grammar,
capture, params, the by-name path, and the closed-grammar negative. They need
the `branchkit-test-harness` binary, which ships inside the BranchKit app;
set `BRANCHKIT_TEST_HARNESS` to its path if the tests report it missing.
Without it they are skipped, and only the unit tests run.

## Install

```bash
branchkit-cli plugin install . --build
```

Then turn on the `input` privilege on the Snippets card in Settings →
Plugins; the app starts the plugin once it is granted.

## License

MIT. See [LICENSE](LICENSE).
