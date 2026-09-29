# Snippets

Speak a snippet's name to type its expansion at the cursor: say
**"snippet shrug"** and `¯\_(ツ)_/¯` appears wherever you're typing. Add
your own snippets in Settings; a snippet marked speakable answers to its
name, and every snippet is selectable by codeword ("snippet", then the
letter code the Discovery HUD shows).

This is the plugin the BranchKit tutorial builds:
[Build Snippets](https://branchkit.dev/guide/getting-started/build-snippets).
The tutorial quotes tag
[`v0.5.0`](https://github.com/branchkit/branchkit-plugin-snippets/tree/v0.5.0).

- The `snippets` collection feeds the matcher (`feeds_matching`), so the
  recognition grammar holds only names that exist, and only the ones marked
  speakable (`grammar_only_when`).
- The `<snippets>` capture resolves a spoken name to its expansion through
  `value_field`, so a voice command arrives with the text to type.
- Every other trigger (the `alt+shift+s` keybind, another plugin's dispatch)
  sends the snippet's `name`, and the handler reads the current expansion
  from the collection, so no trigger holds a stale copy.
- The collection is user-editable in Settings, and open to other plugins
  (`writers: anyone_who_declares`, `merge: collect`): a pack is a manifest
  plus `collection_data.snippets`, with no code.

## Build

```bash
branchkit-cli dev build
```

## Test

```bash
cd src && go test ./...
```

The harness tests run a real matcher against the plugin: seed, grammar,
capture, params, the by-name path, and the closed-grammar negative. They
need the `branchkit-test-harness` binary, which ships inside the BranchKit
app; set `BRANCHKIT_TEST_HARNESS` to its path if the tests report it
missing. Without it they are skipped, and only the unit tests run.

## Install

```bash
branchkit-cli plugin install . --build
```

Then turn on the `input` privilege on the Snippets card in Settings →
Plugins; the app starts the plugin once it is granted.

Default snippets: `shrug`, `table flip`, `disapproval`, `long dash`,
`check mark`, `today`, `time now`.

## Beyond the tutorial's core

- **Dynamic tokens.** `{{date}}` and `{{time}}` resolve when typed, in the
  seeded snippets and in any snippet you write ("Standup {{date}}").
- **Clipboard paste.** An expansion longer than 200 characters, or one with
  a newline, is pasted instead of typed. The plugin asserts the
  `signal_clipboard_in_use` effect around the paste. With the optional
  `clipboard` privilege granted, it restores what the clipboard held
  afterward; without it the paste still works and the clipboard keeps the
  expansion.
- **Import tab.** Paste an espanso YAML file, a CSV of `name,expansion`
  rows, or a JSON array, and name the pack. Imported snippets arrive as
  selection targets (`speakable: false`) grouped by pack; promote one by
  marking it speakable. Removing a pack deletes its records.
