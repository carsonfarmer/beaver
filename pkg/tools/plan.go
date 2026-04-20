package tools

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

const (
	PlanName        = "plan"
	PlanDescription = `Submit or update the execution plan for the current task.

Provide the complete list of entries on every call — the submission replaces any prior plan.

Entries render as a markdown checklist:
- [x] (high) Set up project scaffolding
- [ ] (medium) Add authentication middleware
- [ ] (low) Write integration tests`
)

type PlanEntryInput struct {
	Content  string                `json:"content" description:"Task description"`
	Priority acp.PlanEntryPriority `json:"priority" description:"Priority: high, medium, or low"`
	Status   acp.PlanEntryStatus   `json:"status" description:"Status: pending, in_progress, or completed"`
}

type PlanInput struct {
	Entries []PlanEntryInput `json:"entries" description:"Complete plan, replaces any prior plan"`
}

func NewPlanTool(client acp.Client) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(PlanName, PlanDescription,
		func(ctx context.Context, in PlanInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			entries := make([]acp.PlanEntry, len(in.Entries))
			for i, e := range in.Entries {
				entries[i] = acp.PlanEntry{Content: e.Content, Priority: e.Priority, Status: e.Status}
			}
			if err := acp.NewSessionStream(client, SessionIDFrom(ctx)).SendPlan(ctx, entries); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			var sb strings.Builder
			for _, e := range entries {
				box := "[ ]"
				if e.Status == acp.PlanEntryStatusCompleted {
					box = "[x]"
				}
				fmt.Fprintf(&sb, "- %s (%s) %s\n", box, e.Priority, e.Content)
			}
			return fantasy.NewTextResponse(sb.String()), nil
		})
}
