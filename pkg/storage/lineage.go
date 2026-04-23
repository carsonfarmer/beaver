package storage

import (
	"fmt"
	"slices"

	acp "github.com/ironpark/go-acp"
)

// Lineage walks parent pointers from tip back to a root, hopping across
// sessions as needed, and returns events in chronological order (root first,
// tip last). A zero tip yields a nil chain.
//
// Replay follows parent pointers, not file order. This is where rewind
// (parent into the middle of a session) and fork (parent into a different
// session) become an ordinary chain walk.
func Lineage(a Archive, tip EventID) ([]Event, error) {
	if tip.IsZero() {
		return nil, nil
	}
	var chain []Event
	cache := map[acp.SessionID][]Event{}
	for !tip.IsZero() {
		events, ok := cache[tip.Session]
		if !ok {
			var err error
			events, err = a.Events(tip.Session)
			if err != nil {
				return nil, err
			}
			cache[tip.Session] = events
		}
		if tip.N < 0 || tip.N >= int64(len(events)) {
			return nil, fmt.Errorf("event %s out of range [0,%d)", tip, len(events))
		}
		ev := events[tip.N]
		chain = append(chain, ev)
		tip = ev.Parent
	}
	slices.Reverse(chain)
	return chain, nil
}
