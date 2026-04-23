package storage

import (
	"encoding/json"
	"testing"
)

func TestEventIDString(t *testing.T) {
	if (EventID{}).String() != "" {
		t.Fatal("zero EventID should stringify to empty")
	}
	id := EventID{Session: "abc", N: 7}
	if got := id.String(); got != "abc#7" {
		t.Fatalf("got %q, want %q", got, "abc#7")
	}
}

func TestParseEventID(t *testing.T) {
	cases := []struct {
		in   string
		want EventID
		err  bool
	}{
		{"", EventID{}, false},
		{"abc#42", EventID{Session: "abc", N: 42}, false},
		{"5", EventID{}, true},
		{"#5", EventID{}, true},
		{"bad", EventID{}, true},
		{"abc#nope", EventID{}, true},
	}
	for _, tc := range cases {
		got, err := ParseEventID(tc.in)
		if (err != nil) != tc.err {
			t.Fatalf("ParseEventID(%q) err=%v, want err=%v", tc.in, err, tc.err)
		}
		if got != tc.want {
			t.Fatalf("ParseEventID(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestEventIDJSONRoundTrip(t *testing.T) {
	id := EventID{Session: "sess-1", N: 3}
	b, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"sess-1#3"` {
		t.Fatalf("marshal = %s", b)
	}
	var back EventID
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back != id {
		t.Fatalf("roundtrip: got %v, want %v", back, id)
	}
}
