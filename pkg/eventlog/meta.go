package eventlog

import (
	"encoding/json"

	"github.com/google/uuid"
	acp "github.com/ironpark/go-acp"
)

// WithParentEventID returns a copy of update with _meta.parent_event_id set.
// Pass uuid.Nil to remove the key (used when replaying source events into a
// fork, where source event IDs don't apply in the new log).
func WithParentEventID(update acp.SessionUpdate, parent uuid.UUID) acp.SessionUpdate {
	raw, _ := json.Marshal(update)
	var m map[string]any
	json.Unmarshal(raw, &m)
	meta, _ := m["_meta"].(map[string]any)
	if parent == uuid.Nil {
		if meta != nil {
			delete(meta, "parent_event_id")
			if len(meta) == 0 {
				delete(m, "_meta")
			}
		}
	} else {
		if meta == nil {
			meta = map[string]any{}
		}
		meta["parent_event_id"] = parent.String()
		m["_meta"] = meta
	}
	data, _ := json.Marshal(m)
	var u acp.SessionUpdate
	json.Unmarshal(data, &u)
	return u
}

// FindByMessageID returns the event ID whose update carries the given
// message_id. Linear scan; only user/agent message chunks and thoughts
// carry message IDs (matches the protocol-layer addressability of messages).
func FindByMessageID(events []Event, messageID string) (uuid.UUID, bool) {
	if messageID == "" {
		return uuid.Nil, false
	}
	for _, ev := range events {
		if ev.Update == nil {
			continue
		}
		raw, err := json.Marshal(ev.Update)
		if err != nil {
			continue
		}
		var peek struct {
			MessageID string `json:"messageId"`
		}
		if err := json.Unmarshal(raw, &peek); err == nil && peek.MessageID == messageID {
			return ev.ID, true
		}
	}
	return uuid.Nil, false
}
