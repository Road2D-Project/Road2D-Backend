package engine

import (
	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"
	"log"
)

func PendingPosition(prg *TravelProgress, tripInfor *model.TripInformation) {
	branch, ok := tripInfor.Branches[prg.BranchID]
	if !ok {
		log.Printf("branch %s not found", prg.BranchID)
		return
	}
	if !loadHop(prg, branch, prg.HopIndex, 0) {
		return
	}

	for p := range prg.PendingPoint {
		if prg.ConnectionStatus == enum.DISCONNECTED || prg.Route == nil {
			continue
		}
		snap := prg.Route.Snap(p, prg.LastSegIndex)
		prg.LastSegIndex = snap.Seg
		prg.Progress = snap.Progress
		/*
			- Have Finished the route yet?
			- Have any hop in route to go?
		*/

		if !prg.Route.AtEnd(snap.Seg, snap.Progress, snap.Dist) {
			continue
		}
		if prg.HopIndex+1 >= len(branch.Hops) {

			continue
		}
		// Still go on hop
		if !loadHop(prg, branch, prg.HopIndex+1, prg.Route.TotalM()) {
			continue
		}
	}
}

func loadHop(prg *TravelProgress, branch *model.TravelBranch, hopIndex int, baseM float64) bool {
	if hopIndex < 0 || hopIndex >= len(branch.Hops) {
		log.Printf("invalid hop index %d", hopIndex)
		return false
	}
	hop := branch.Hops[hopIndex]
	if !hop.Ready || len(hop.Points) < 2 {
		log.Printf("hop %d is not ready", hopIndex)
		return false
	}
	next := geo.NewRoute(hop.Points)
	next.Shift(baseM)
	prg.HopIndex = hopIndex
	prg.LastSegIndex = 0
	prg.Route = next
	return true
}
