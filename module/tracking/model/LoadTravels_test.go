package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestAssembleHopsAlignsStopsAndLeavesGapsUnready(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	branch := &TravelBranch{
		Stops: []Stop{
			{DestinationID: a},
			{DestinationID: b},
			{DestinationID: c},
		},
	}
	travels := map[travelKey]travelRow{
		{a, b}: {FromDestinationID: a, ToDestinationID: b, Polyline: "_p~iF~ps|U_ulLnnqC_mqNvxq`@", DistanceM: 10, DurationS: 2},
	}

	assembleHops(branch, travels)

	if len(branch.Hops) != 2 {
		t.Fatalf("hops = %d", len(branch.Hops))
	}
	if !branch.Hops[0].Ready || branch.Hops[0].FromDestinationID != a || branch.Hops[0].ToDestinationID != b {
		t.Fatalf("first hop = %+v", branch.Hops[0])
	}
	if len(branch.Hops[0].Points) == 0 {
		t.Fatal("expected decoded points")
	}
	if branch.Hops[1].Ready || branch.Hops[1].FromDestinationID != b || branch.Hops[1].ToDestinationID != c {
		t.Fatalf("second hop = %+v", branch.Hops[1])
	}
}
