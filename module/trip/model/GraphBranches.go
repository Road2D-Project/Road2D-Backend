package model

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

// DTO chính
type GraphBranch [][]Destination

var (
	ErrEmptyGraphBranch = errors.New("Empty Graph Branch")
	ErrNilTrip          = errors.New("Nil Trip Branch")
)

func ErrBranchNotEnoughDestinations(name string) error {
	return fmt.Errorf("Branch %s not enough destinations", name)
}
func ErrDestinationDoesNotMatch(desName string, branchName string) error {
	return fmt.Errorf("Destination %s in branch %s does not match ", branchName, desName)
}

func ErrDuplicateDestinationInBranch(desName string, branchName string) error {
	return fmt.Errorf("Duplicate destination  %s in branch %s", desName, branchName)
}
func BuildTripBranches(trip *Trip, graphBranches GraphBranch) ([]TripBranch, error) {
	if trip == nil {

		return nil, ErrNilTrip // cần khai báo
	}
	if len(graphBranches) == 0 || len(graphBranches[0]) == 0 {
		return nil, ErrEmptyGraphBranch
	}
	// Pass 1: đếm số branch chứa mỗi destination, đồng thời bắt trùng trong 1 branch
	occur := make(map[uuid.UUID]int)
	for i, branch := range graphBranches {
		seen := make(map[uuid.UUID]bool, len(branch))
		for _, d := range branch {
			if seen[d.ID] {
				return nil, ErrDuplicateDestinationInBranch(d.Name, strconv.Itoa(i)) // cần khai báo
			}
			seen[d.ID] = true
			occur[d.ID]++
		}
	}
	result := make([]TripBranch, len(graphBranches))
	// branch[0] = main
	result[0] = *NewTripBranch(trip, nil, nil)
	for j := range graphBranches[0] {
		result[0].Stops = append(result[0].Stops, *NewBranchDestination(&result[0], &graphBranches[0][j], j))
	}
	// branch[n>0] = sub: split = branch[0], merge = branch[len-1], cả 2 phải shared
	for i := 1; i < len(graphBranches); i++ {
		branch := graphBranches[i]
		branchName := strconv.Itoa(i)
		if len(branch) < 2 {
			return nil, ErrBranchNotEnoughDestinations(branchName)
		}
		split, merge := &branch[0], &branch[len(branch)-1]
		if occur[split.ID] < 2 {
			return nil, ErrDestinationDoesNotMatch(split.Name, branchName)
		}
		if occur[merge.ID] < 2 {
			return nil, ErrDestinationDoesNotMatch(merge.Name, branchName)
		}
		b := NewTripBranch(trip, nil, nil)
		b.SplitFrom, b.SplitFromDestinationID = split, &split.ID
		b.MergeTo, b.MergeToDestinationID = merge, &merge.ID
		for j := range branch { // order = index trong branch, bao gồm split và merge
			b.Stops = append(b.Stops, *NewBranchDestination(b, &branch[j], j))
		}
		result[i] = *b
	}
	return result, nil
}
