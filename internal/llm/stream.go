// This file defines LLM stream event payloads.
package llm

type StreamEvent struct {
	SessionID int64
	Type      string
	Data      string
}
