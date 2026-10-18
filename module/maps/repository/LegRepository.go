package repository

import (
	"context"
	"errors"
	"time"

	"Road-To-Destination-BE/module/share"
	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LegRepository is the durable half of the leg cache. Redis expires on its own;
// here a row lives until a fresher computation overwrites it.
type LegRepository struct {
	db  *gorm.DB
	ttl time.Duration
}

func NewLegRepository(db *gorm.DB) *LegRepository {
	seconds := share.GetEnvIntDefault("LEG_CACHE_TTL_SECONDS", defaultLegCacheTTLSeconds)
	return &LegRepository{
		db:  db,
		ttl: time.Duration(seconds) * time.Second,
	}
}

// Find returns nil without an error on a miss, so callers treat a stale row and
// an absent row the same way: recompute and upsert over it.
func (r *LegRepository) Find(ctx context.Context, fromLat, fromLng, toLat, toLng float64, vehicle enum.Vehicle) (*tripmodel.Leg, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	var leg tripmodel.Leg
	err := r.db.WithContext(ctx).
		Where("from_lat = ? AND from_lng = ? AND to_lat = ? AND to_lng = ? AND vehicle = ?",
			fromLat, fromLng, toLat, toLng, vehicle).
		First(&leg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if leg.Expired(time.Now().UTC(), r.ttl) {
		return nil, nil
	}
	return &leg, nil
}

// Upsert writes the leg on the idx_leg_cache key.
// On conflict Postgres keeps the row that is already there. BeforeCreate may
// have stamped a new id that was never inserted, so the stored id is copied
// back before a travel or the Redis cache points at this leg.
func (r *LegRepository) Upsert(ctx context.Context, leg *tripmodel.Leg) error {
	if r == nil || r.db == nil || leg == nil {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Omit("FromLocation", "ToLocation").
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "from_lat"}, {Name: "from_lng"},
				{Name: "to_lat"}, {Name: "to_lng"},
				{Name: "vehicle"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"polyline", "distance_m", "duration_s", "steps",
				"source", "last_computed_at", "ttl_seconds", "updated_at",
			}),
		}).
		Create(leg).Error; err != nil {
		return err
	}
	var stored tripmodel.Leg
	if err := r.db.WithContext(ctx).
		Select("id").
		Where("from_lat = ? AND from_lng = ? AND to_lat = ? AND to_lng = ? AND vehicle = ?",
			leg.FromLat, leg.FromLng, leg.ToLat, leg.ToLng, leg.Vehicle).
		Take(&stored).Error; err != nil {
		return err
	}
	leg.ID = stored.ID
	return nil
}

// Exists reports whether this id is a legs row. A Redis hit can outlive the row.
func (r *LegRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	if r == nil || r.db == nil || id == uuid.Nil {
		return false, nil
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&tripmodel.Leg{}).Where("id = ?", id).Count(&n).Error
	return n > 0, err
}

var _ LegStore = (*LegRepository)(nil)
