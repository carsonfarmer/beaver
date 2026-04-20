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

// ConfigID identifies a session config option.
type ConfigID = acp.SessionConfigID

// Session config option IDs.
const (
	SessionConfigModel        ConfigID = "model"
	SessionConfigThoughtLevel ConfigID = "thought_level"
)

// SessionOptions builds ACP session config options from the registry's
// available models plus the caller's current model and thought-level
// selections. Empty values fall back to registry defaults.
func SessionOptions(registry ModelRegistry, currentModel, currentThoughtLevel string) []acp.SessionConfigOption {
	var modelGroups []acp.SessionConfigSelectGroup
	for _, g := range registry.AvailableModelGroups() {
		var opts []acp.SessionConfigSelectOption
		for _, m := range g.Models {
			opts = append(opts, acp.SessionConfigSelectOption{
				Value: acp.SessionConfigValueID(m.ID),
				Name:  m.Name,
			})
		}
		modelGroups = append(modelGroups, acp.SessionConfigSelectGroup{
			Group:   acp.SessionConfigGroupID(g.Provider),
			Name:    g.Name,
			Options: opts,
		})
	}

	if currentModel == "" {
		currentModel = registry.Defaults().Model
	}
	if currentThoughtLevel == "" {
		currentThoughtLevel = registry.Defaults().ThoughtLevel
	}
	return []acp.SessionConfigOption{
		acp.NewSessionConfigOptionSelect(SessionConfigModel, "Model",
			acp.SessionConfigValueID(currentModel), modelGroups),
		acp.NewSessionConfigOptionSelect(SessionConfigThoughtLevel, "Thought Level",
			acp.SessionConfigValueID(currentThoughtLevel), []acp.SessionConfigSelectOption{
				{Value: ThoughtNone, Name: "None", Description: "No extended reasoning"},
				{Value: ThoughtLow, Name: "Low", Description: "Fast responses with lighter reasoning"},
				{Value: ThoughtMedium, Name: "Medium", Description: "Balances speed and reasoning depth"},
				{Value: ThoughtHigh, Name: "High", Description: "Greater reasoning depth for complex problems"},
			}),
	}
}
