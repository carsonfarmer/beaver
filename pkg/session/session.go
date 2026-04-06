package session

import (
	"context"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/llm"
	acp "github.com/ironpark/go-acp"
)

// Session holds per-session state.
type Session struct {
	Cwd          string             `json:"cwd,omitempty"`
	Title        string             `json:"title,omitempty"`
	UpdatedAt    string             `json:"updatedAt,omitempty"`
	Model        string             `json:"model,omitempty"`        // "provider/model"
	ThoughtLevel string             `json:"thoughtLevel,omitempty"`
	SystemPrompt string             `json:"systemPrompt,omitempty"`
	History      []fantasy.Message  `json:"history,omitempty"`
	Cancel       context.CancelFunc `json:"-"`
}

// Session config option IDs.
const (
	ConfigModel        acp.SessionConfigID = "model"
	ConfigThoughtLevel acp.SessionConfigID = "thought_level"
)

// ConfigOptions builds the config options with current values and available choices.
func ConfigOptions(registry llm.ModelRegistry, sess *Session) []acp.SessionConfigOption {
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
	currentModel := sess.Model
	if currentModel == "" {
		currentModel = registry.Defaults().Model
	}
	currentThought := sess.ThoughtLevel
	if currentThought == "" {
		currentThought = registry.Defaults().ThoughtLevel
	}

	return []acp.SessionConfigOption{
		acp.NewSessionConfigOptionSelect(ConfigModel, "Model",
			acp.SessionConfigValueID(currentModel), modelGroups),
		acp.NewSessionConfigOptionSelect(ConfigThoughtLevel, "Reasoning Effort",
			acp.SessionConfigValueID(currentThought), []acp.SessionConfigSelectOption{
				{Value: "none", Name: "None", Description: "No extended reasoning"},
				{Value: "low", Name: "Low", Description: "Fast responses with lighter reasoning"},
				{Value: "medium", Name: "Medium", Description: "Balances speed and reasoning depth"},
				{Value: "high", Name: "High", Description: "Greater reasoning depth for complex problems"},
			}),
	}
}

// Info returns an acp.SessionInfo for this session.
func (s *Session) Info(id acp.SessionID) acp.SessionInfo {
	return acp.SessionInfo{
		SessionID: id,
		Cwd:       s.Cwd,
		Title:     s.Title,
		UpdatedAt: s.UpdatedAt,
	}
}

// Fork creates a copy of this session with a new cwd and a cloned history.
func (s *Session) Fork(cwd string) *Session {
	history := make([]fantasy.Message, len(s.History))
	copy(history, s.History)
	return &Session{
		Cwd:          cwd,
		Model:        s.Model,
		ThoughtLevel: s.ThoughtLevel,
		History:      history,
	}
}
