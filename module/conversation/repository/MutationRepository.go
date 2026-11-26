package repository

import (
	"context"
	"errors"
	"time"

	"Road-To-Destination-BE/module/conversation/model"
	tripModel "Road-To-Destination-BE/module/trip/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MutationRepository struct {
	db           *gorm.DB
	Conversation *model.Conversation
	Member       *tripModel.TripMember
	Now          time.Time
}

func NewMutationRepository(db *gorm.DB) *MutationRepository { return &MutationRepository{db: db} }

// Lock order matches planning. The member SHARE lock also guards against roster
// updates that do not yet acquire a parent-trip lock. Callbacks perform DB IO only.
func (r *MutationRepository) Transaction(ctx context.Context, actor, id uuid.UUID, run func(*MutationRepository) error) error {
	if r.db == nil {
		return ErrUnavailable
	}
	for attempt := 0; attempt < 3; attempt++ {
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var scope struct{ TripID uuid.UUID }
			result := tx.Model(&model.Conversation{}).Select("trip_id").Where("id = ?", id).Scan(&scope)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrConversationNotFound
			}
			var trip tripModel.Trip
			if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&trip, "id = ?", scope.TripID).Error; err != nil {
				return chatNotFound(err)
			}
			var member tripModel.TripMember
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("trip_id = ? AND user_id = ? AND status = 'active'", scope.TripID, actor).First(&member).Error; err != nil {
				return chatNotFound(err)
			}
			var conversation model.Conversation
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&conversation, "id = ?", id).Error; err != nil {
				return chatNotFound(err)
			}
			var now time.Time
			if err := tx.Raw("SELECT clock_timestamp()").Scan(&now).Error; err != nil {
				return err
			}
			return run(&MutationRepository{db: tx, Conversation: &conversation, Member: &member, Now: now})
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40001" && pgErr.Code != "40P01") {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return ErrUnavailable
}

func chatNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrConversationNotFound
	}
	return err
}
