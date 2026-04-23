package storage

import (
	"testing"

	acp "github.com/ironpark/go-acp"
)

func TestLineageEmptyTip(t *testing.T) {
	a := NewMemArchive()
	chain, err := Lineage(a, EventID{})
	if err != nil {
		t.Fatal(err)
	}
	if chain != nil {
		t.Fatalf("zero tip should return nil chain, got %v", chain)
	}
}

func TestLineageLinearChain(t *testing.T) {
	a := NewMemArchive()
	a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"})
	e1, _ := a.Append("s1", EventID{}, textUpdate("a"))
	e2, _ := a.Append("s1", EventID{}, textUpdate("b"))
	chain, err := Lineage(a, e2)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 3 {
		t.Fatalf("chain len = %d, want 3", len(chain))
	}
	if chain[0].Info == nil {
		t.Fatal("chain[0] should be header")
	}
	if chain[1].ID != e1 || chain[2].ID != e2 {
		t.Fatalf("chain ordering wrong: %v,%v,%v", chain[0].ID, chain[1].ID, chain[2].ID)
	}
}

func TestLineageRewind(t *testing.T) {
	a := NewMemArchive()
	a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"})
	e1, _ := a.Append("s1", EventID{}, textUpdate("a"))
	e2, _ := a.Append("s1", EventID{}, textUpdate("b"))
	// Rewind: new event whose parent is e1, not e2.
	e3, _ := a.Append("s1", e1, textUpdate("c"))
	chain, err := Lineage(a, e3)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 3 {
		t.Fatalf("rewound chain len = %d, want 3 (header, e1, e3)", len(chain))
	}
	if chain[2].ID != e3 || chain[1].ID != e1 {
		t.Fatalf("rewind walk wrong: %v,%v", chain[1].ID, chain[2].ID)
	}
	if chain[1].ID == e2 {
		t.Fatal("rewind should have skipped e2")
	}
	_ = e2
}

func TestLineageCrossSessionFork(t *testing.T) {
	a := NewMemArchive()
	a.Create("parent", EventID{}, acp.SessionInfo{SessionID: "parent"})
	p1, _ := a.Append("parent", EventID{}, textUpdate("p1"))
	p2, _ := a.Append("parent", EventID{}, textUpdate("p2"))

	// Fork from parent at p1 (zero-copy: child's header points at p1).
	a.Create("child", p1, acp.SessionInfo{SessionID: "child"})
	c1, _ := a.Append("child", EventID{}, textUpdate("c1"))

	chain, err := Lineage(a, c1)
	if err != nil {
		t.Fatal(err)
	}
	// Expected: parent header, p1, child header, c1.
	if len(chain) != 4 {
		t.Fatalf("cross-session chain len = %d, want 4", len(chain))
	}
	if chain[0].ID.Session != "parent" || chain[1].ID != p1 {
		t.Fatalf("parent lineage wrong: %v,%v", chain[0].ID, chain[1].ID)
	}
	if chain[2].ID.Session != "child" || chain[3].ID != c1 {
		t.Fatalf("child lineage wrong: %v,%v", chain[2].ID, chain[3].ID)
	}
	// p2 is not in the chain.
	for _, ev := range chain {
		if ev.ID == p2 {
			t.Fatal("p2 should not appear in forked lineage")
		}
	}
}

func TestLineageMissingEventErrors(t *testing.T) {
	a := NewMemArchive()
	a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"})
	_, err := Lineage(a, EventID{Session: "s1", N: 99})
	if err == nil {
		t.Fatal("expected out-of-range event to error")
	}
}
