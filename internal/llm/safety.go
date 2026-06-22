// This file defines LLM safety review outcomes.
package llm

type SafetyDecision string

const (
	SafetyAllow  SafetyDecision = "allow"
	SafetyReview SafetyDecision = "review"
	SafetyDeny   SafetyDecision = "deny"
)
