package model

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

// destinations builds n destinations named d0..dn so tests can refer to stops
// by their position.
func destinations(n int) []Destination {
	out := make([]Destination, n)
	for i := range out {
		out[i].ID = uuid.New()
		out[i].Name = fmt.Sprintf("d%d", i)
	}
	return out
}

func closedTail(n int) []bool {
	return make([]bool, n)
}

func TestBuildTripBranchesMainHasNoSplitOrMerge(t *testing.T) {
	d := destinations(6)
	graph := GraphBranch{{d[0], d[1], d[2], d[4], d[5]}}

	got, err := BuildTripBranches(&Trip{}, graph, closedTail(len(graph)))
	if err != nil {
		t.Fatal(err)
	}
	main := got[0]
	if main.SplitFrom != nil || main.MergeTo != nil {
		t.Fatalf("main branch should have no endpoints, got split=%v merge=%v", main.SplitFrom, main.MergeTo)
	}
	if len(main.Stops) != len(graph[0]) {
		t.Fatalf("stops = %d, want %d", len(main.Stops), len(graph[0]))
	}
	for i, stop := range main.Stops {
		if stop.OrderInBranch != i {
			t.Fatalf("stop %d has order %d", i, stop.OrderInBranch)
		}
	}
}

func TestBuildTripBranchesSubSplitsAndMerges(t *testing.T) {
	d := destinations(10)
	graph := GraphBranch{
		{d[0], d[1], d[2], d[4], d[5], d[9]},
		{d[1], d[3], d[9]},
		{d[0], d[4], d[6], d[7], d[8], d[9]},
	}

	got, err := BuildTripBranches(&Trip{}, graph, closedTail(len(graph)))
	if err != nil {
		t.Fatal(err)
	}

	// The middle branch overlaps the main branch at d4 and that stays a plain stop.
	if got[2].SplitFromDestinationID == nil || *got[2].SplitFromDestinationID != d[0].ID {
		t.Fatal("branch 2 should split at d0")
	}
	if got[2].MergeToDestinationID == nil || *got[2].MergeToDestinationID != d[9].ID {
		t.Fatal("branch 2 should merge at d9")
	}
	for i, stop := range got[2].Stops {
		if stop.OrderInBranch != i || stop.DestinationID != graph[2][i].ID {
			t.Fatalf("branch 2 stop %d = order %d dest %s", i, stop.OrderInBranch, stop.DestinationID)
		}
	}
}

func TestBuildTripBranchesOpenTailSkipsMerge(t *testing.T) {
	d := destinations(4)
	graph := GraphBranch{
		{d[0], d[1], d[2]},
		{d[1], d[3]},
	}

	got, err := BuildTripBranches(&Trip{}, graph, []bool{false, true})
	if err != nil {
		t.Fatal(err)
	}
	if got[1].SplitFromDestinationID == nil || *got[1].SplitFromDestinationID != d[1].ID {
		t.Fatal("open branch should still split")
	}
	if got[1].MergeTo != nil || got[1].MergeToDestinationID != nil {
		t.Fatal("open branch should have no merge")
	}
	if len(got[1].Stops) != 2 || got[1].Stops[1].DestinationID != d[3].ID {
		t.Fatal("the open tail stop should still be kept")
	}
}

func TestBuildTripBranchesRejectsUnsharedEndpoint(t *testing.T) {
	d := destinations(3)
	graph := GraphBranch{
		{d[0], d[1]},
		{d[1], d[2]},
	}

	_, err := BuildTripBranches(&Trip{}, graph, closedTail(len(graph)))
	if err == nil {
		t.Fatal("expected an error for a merge that no other branch shares")
	}
}

func TestBuildTripBranchesRejectsDuplicateInBranch(t *testing.T) {
	d := destinations(3)
	graph := GraphBranch{{d[0], d[1], d[0]}}

	_, err := BuildTripBranches(&Trip{}, graph, closedTail(len(graph)))
	if err == nil {
		t.Fatal("expected an error for a destination repeated inside one branch")
	}
}

func TestBuildTripBranchesRejectsNilTripAndEmptyGraph(t *testing.T) {
	if _, err := BuildTripBranches(nil, GraphBranch{{}}, closedTail(1)); !errors.Is(err, ErrNilTrip) {
		t.Fatalf("nil trip: got %v", err)
	}
	if _, err := BuildTripBranches(&Trip{}, nil, nil); !errors.Is(err, ErrEmptyGraphBranch) {
		t.Fatalf("empty graph: got %v", err)
	}
}
