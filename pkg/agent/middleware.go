package agent

import (
	"context"
	"encoding/json"

	acp "github.com/ironpark/go-acp"
)

// SubscriptionMiddleware intercepts session lifecycle methods and
// subscribes/unsubscribes the calling client in the SessionRegistry.
func SubscriptionMiddleware(registry *SessionRegistry) func(next acp.MethodHandler) acp.MethodHandler {
	return func(next acp.MethodHandler) acp.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			result, err := next(ctx, method, params)
			if err != nil {
				return result, err
			}

			client := ClientFrom(ctx)
			if client == nil {
				return result, nil
			}

			switch method {
			case acp.AgentMethods.SessionNew:
				if r, ok := result.(*acp.NewSessionResponse); ok {
					registry.Subscribe(r.SessionID, client)
				}
			case acp.AgentMethods.SessionLoad:
				var req acp.LoadSessionRequest
				if json.Unmarshal(params, &req) == nil {
					registry.Subscribe(req.SessionID, client)
				}
			case acp.AgentMethodsUnstable.SessionResume:
				var req acp.ResumeSessionRequest
				if json.Unmarshal(params, &req) == nil {
					registry.Subscribe(req.SessionID, client)
				}
			case acp.AgentMethodsUnstable.SessionClose:
				var req acp.CloseSessionRequest
				if json.Unmarshal(params, &req) == nil {
					registry.Unsubscribe(req.SessionID, client)
				}
			}

			return result, nil
		}
	}
}
