package model

import "github.com/google/uuid"

type TravelBranch struct {
	ID             uuid.UUID
	Label          string
	SplitFrom      *uuid.UUID
	MergeTo        *uuid.UUID
	Stops          []Stop // order_in_branch tăng dần
	Hops           []Hop  // len(Hops) == len(Stops)-1; nhánh 1 điểm thì Hops rỗng
	TotalDistanceM float64
}
