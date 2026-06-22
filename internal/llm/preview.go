// This file defines LLM action previews that require human confirmation later.
package llm

type ActionPreview struct {
	Title       string
	Description string
	Risk        string
}
