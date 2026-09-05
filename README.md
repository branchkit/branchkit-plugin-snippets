# Snippets

Speak a snippet's name to type its expansion at the cursor: say
**"snippet shrug"** and `¯\_(ツ)_/¯` appears wherever you're typing. Add
your own snippets in Settings — new names become speakable immediately,
because the recognition grammar is built from the collection.

This plugin is deliberately small. It is the companion to the tutorial
[From template to a plugin worth keeping](https://branchkit.dev/guide/getting-started/your-first-plugin),
and demonstrates the platform's signature loop in ~20 lines of handler:

- the `snippets` collection feeds the matcher (`feeds_matching`), so the
  closed grammar can only hear names that exist;
- the `<snippets>` capture resolves the spoken name to its expansion
  through `value_field`, so the handler receives the text to type and
  never looks anything up;
- the collection is user-editable in the Settings UI, so customizing the
  plugin needs no code.

## Build

```bash
branchkit-cli dev build
```

## Test

```bash
cd src && go test ./...
```

The tests run the shipped test-harness: seed → grammar → capture →
params, plus the closed-grammar negative.

## Install

```bash
branchkit-cli plugin install . --build
```

Default snippets: `shrug`, `table flip`, `disapproval`, `long dash`,
`check mark`, `today`, `time now`.

## Beyond the tutorial

`main` has grown past the teaching version (tag `v0.2.0` is what the
tutorial builds):

- **Dynamic tokens** — `{{date}}` and `{{time}}` resolve when typed, in
  the seeded snippets and in any snippet you write ("Standup {{date}}").
- **Long and multi-line snippets paste** instead of typing keystroke by
  keystroke — signatures, boilerplate, and prompt-library entries arrive
  instantly and intact. The `signal_clipboard_in_use` effect announces
  the clipboard dance; granting the **optional `clipboard` privilege**
  lets the plugin restore what your clipboard held afterward (without it,
  the paste still works and the clipboard keeps the expansion).
- **Open collection** — `writers: anyone_who_declares` means any plugin
  may ship a *pack*: a manifest plus `collection_data`, no code, records
  landing here subject to your write grant. The `category` field keeps
  large collections organized in Settings.
