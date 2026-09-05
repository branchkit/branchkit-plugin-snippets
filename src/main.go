package main

import "github.com/branchkit/plugin-sdk-go"

func main() {
	plugin := branchkit.NewPlugin()

	// The matcher resolves the <snippets> capture through the collection's
	// value_field, so the param already carries the expansion — the spoken
	// key never reaches this handler.
	HandleType(plugin, func(p TypeParams, _ *branchkit.OnActionRequest) (any, error) {
		if p.Text == "" {
			return nil, nil
		}
		return nil, plugin.InputTypeText(p.Text)
	})

	plugin.Run()
}
