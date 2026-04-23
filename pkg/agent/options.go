package agent

import (
	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/extensions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/storage"
	acp "github.com/ironpark/go-acp"
)

// Option configures an Agent.
type Option func(*Agent)

// WithClient sets the ACP client for outbound calls and notifications.
func WithClient(c acp.Client) Option {
	return func(a *Agent) { a.client = c }
}

// WithRegistry sets the model registry for model resolution and options.
func WithRegistry(r llm.ModelRegistry) Option {
	return func(a *Agent) { a.registry = r }
}

// WithStorage sets the durable event log store.
func WithStorage(s storage.Archive) Option {
	return func(a *Agent) { a.archive = s }
}

// WithTools registers the agent's tool set.
func WithTools(tools ...fantasy.AgentTool) Option {
	return func(a *Agent) { a.tools = append(a.tools, tools...) }
}

// WithProviders registers context providers that contribute to the system prompt.
func WithProviders(providers ...extensions.Provider) Option {
	return func(a *Agent) { a.providers = append(a.providers, providers...) }
}
