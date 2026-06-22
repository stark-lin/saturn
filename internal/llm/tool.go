// This file defines LLM tool metadata.
package llm

type Tool struct {
	Name        string
	Description string
	ReadOnly    bool
	DraftOnly   bool
}
