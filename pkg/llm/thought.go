package llm

import acp "github.com/ironpark/go-acp"

// ThoughtLevel identifies a reasoning depth setting.
type ThoughtLevel = acp.SessionConfigValueID

// Thought levels.
const (
	ThoughtNone   ThoughtLevel = "none"
	ThoughtLow    ThoughtLevel = "low"
	ThoughtMedium ThoughtLevel = "medium"
	ThoughtHigh   ThoughtLevel = "high"
)

// ValidThoughtLevel reports whether t is a recognized thought level.
func ValidThoughtLevel(t ThoughtLevel) bool {
	switch t {
	case ThoughtNone, ThoughtLow, ThoughtMedium, ThoughtHigh:
		return true
	}
	return false
}

// thoughtSelectOptions are the ACP select options for the thought-level config.
var thoughtSelectOptions = []acp.SessionConfigSelectOption{
	{Value: ThoughtNone, Name: "None", Description: "No extended reasoning"},
	{Value: ThoughtLow, Name: "Low", Description: "Fast responses with lighter reasoning"},
	{Value: ThoughtMedium, Name: "Medium", Description: "Balances speed and reasoning depth"},
	{Value: ThoughtHigh, Name: "High", Description: "Greater reasoning depth for complex problems"},
}
