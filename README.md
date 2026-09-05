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
`check mark`.
