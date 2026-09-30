package trip

import (
	"errors"
	"fmt"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ComposeMainBranch is one route branch, in the order of the destination ids.
// Those ids are the pins just forked from location ids, not a later name lookup.
func ComposeMainBranch(destinationIDs []uuid.UUID) (request.SetTripGraphRequest, error) {
	if len(destinationIDs) == 0 {
		return request.SetTripGraphRequest{}, errors.New("no destinations to place on the main branch")
	}
	return request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{destinationIDs},
		OpenTail: []bool{false},
	}, nil
}

// ComposeSetTripGraph builds one main branch from destination names.
// names[i] must be the name stored when that pin was forked. The lookup uses the name,
// not the id returned by fork, and the slice index is the stop order.
// When several rows share a name, the newest one is the pin from this run.
func ComposeSetTripGraph(db *gorm.DB, names []string) (request.SetTripGraphRequest, error) {
	if len(names) == 0 {
		return request.SetTripGraphRequest{}, errors.New("no destination names to place on the graph")
	}
	ids := make([]uuid.UUID, 0, len(names))
	for idx, name := range names {
		id, err := destinationIDByForkName(db, name)
		if err != nil {
			return request.SetTripGraphRequest{}, fmt.Errorf("destination %d %q: %w", idx, name, err)
		}
		ids = append(ids, id)
	}
	return request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{ids},
		OpenTail: []bool{false},
	}, nil
}

func destinationIDByForkName(db *gorm.DB, name string) (uuid.UUID, error) {
	var row struct {
		ID uuid.UUID
	}
	err := db.Table(model.Destination{}.TableName()).
		Select("id").
		Where("name = ?", name).
		Order("created_at DESC").
		Limit(1).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, fmt.Errorf("no destination named %q", name)
	}
	if err != nil {
		return uuid.Nil, err
	}
	if row.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("no destination named %q", name)
	}
	return row.ID, nil
}
