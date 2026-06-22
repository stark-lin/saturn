// This file contains LLM session behavior helpers.
package llm

func (s Session) Empty() bool {
	return s.ID == 0 && s.Title == ""
}
