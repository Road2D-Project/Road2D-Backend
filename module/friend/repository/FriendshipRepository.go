package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/friend/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// searchCandidateCap bounds the SQL scan before the service ranks and cuts
// the list. Replace both this cap and the service limit with a cursor.
const searchCandidateCap = 100

type FriendshipRepository struct {
	db *gorm.DB
}

func NewFriendshipRepository(db *gorm.DB) *FriendshipRepository {
	return &FriendshipRepository{db: db}
}

func (r *FriendshipRepository) FindPair(ctx context.Context, a, b uuid.UUID) (*model.Friendship, error) {
	low, high := model.OrderedPair(a, b)
	var row model.Friendship
	err := r.db.WithContext(ctx).
		Where("user_low_id = ? AND user_high_id = ?", low, high).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *FriendshipRepository) Insert(ctx context.Context, row *model.Friendship) error {
	err := r.db.WithContext(ctx).Create(row).Error
	if isUniqueViolation(err) {
		return ErrRequestAlreadySent
	}
	return err
}

func (r *FriendshipRepository) Save(ctx context.Context, row *model.Friendship) error {
	return r.db.WithContext(ctx).Save(row).Error
}

func (r *FriendshipRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Friendship{}, "id = ?", id).Error
}

func (r *FriendshipRepository) ListAccepted(ctx context.Context, userID uuid.UUID) ([]FriendRow, error) {
	return r.listByStatus(ctx, userID, enum.FriendStatusAccepted, false, false)
}

func (r *FriendshipRepository) ListPending(ctx context.Context, userID uuid.UUID, incoming bool) ([]FriendRow, error) {
	return r.listByStatus(ctx, userID, enum.FriendStatusPending, true, incoming)
}

func (r *FriendshipRepository) listByStatus(ctx context.Context, userID uuid.UUID, status enum.FriendStatus, pendingOnly bool, incoming bool) ([]FriendRow, error) {
	var scanned []friendScan
	other := "CASE WHEN f.user_low_id = ? THEN f.user_high_id ELSE f.user_low_id END"
	query := r.db.WithContext(ctx).
		Table("friendships AS f").
		Select(other+" AS user_id, u.username, f.status, f.requester_id", userID).
		Joins("JOIN users u ON u.id = "+other, userID).
		Where("f.status = ? AND (f.user_low_id = ? OR f.user_high_id = ?)", status.String(), userID, userID)
	if pendingOnly && incoming {
		query = query.Where("f.requester_id <> ?", userID)
	}
	if pendingOnly && !incoming {
		query = query.Where("f.requester_id = ?", userID)
	}
	if err := query.Scan(&scanned).Error; err != nil {
		return nil, err
	}
	rows := make([]FriendRow, 0, len(scanned))
	for _, item := range scanned {
		parsed, err := enum.FriendStatusString(item.Status)
		if err != nil {
			return nil, err
		}
		rows = append(rows, FriendRow{
			UserID:      item.UserID,
			Username:    item.Username,
			Status:      parsed,
			RequesterID: item.RequesterID,
		})
	}
	return rows, nil
}

// SearchCandidates loads users whose username contains usernameLike, plus an
// exact number_phone hit when exactPhone is set.
//
// usernameLike must already be an escaped ILIKE pattern. Phone comparison is
// equality on the full stored number. Do not change the phone predicate to
// ILIKE, prefix, or pg_trgm: a partial number must not match.
//
// Upgrade later:
//   - pg_trgm similarity() and a GIN index on username (this ILIKE does not
//     tolerate typos)
//   - a display-name column; do not add email to public search
//   - an index on users.number_phone if exact lookup gets hot
//   - cursor pagination instead of searchCandidateCap
//   - rate limiting, and skip blocked users when a block list exists
func (r *FriendshipRepository) SearchCandidates(ctx context.Context, viewer uuid.UUID, usernameLike, exactPhone string) ([]UserCandidate, error) {
	var scanned []userScan
	query := r.db.WithContext(ctx).Table("users").Where("id <> ?", viewer)
	if exactPhone == "" {
		query = query.
			Select("id AS user_id, username, false AS phone_matched").
			Where("username ILIKE ? ESCAPE '\\'", usernameLike)
	} else {
		query = query.
			Select("id AS user_id, username, (number_phone = ?) AS phone_matched", exactPhone).
			Where("username ILIKE ? ESCAPE '\\' OR number_phone = ?", usernameLike, exactPhone)
	}
	if err := query.Limit(searchCandidateCap).Scan(&scanned).Error; err != nil {
		return nil, err
	}
	out := make([]UserCandidate, 0, len(scanned))
	for _, item := range scanned {
		out = append(out, UserCandidate{
			UserID:       item.UserID,
			Username:     item.Username,
			PhoneMatched: item.PhoneMatched,
		})
	}
	return out, nil
}

func (r *FriendshipRepository) Relations(ctx context.Context, viewer uuid.UUID, others []uuid.UUID) (map[uuid.UUID]model.Friendship, error) {
	out := map[uuid.UUID]model.Friendship{}
	if len(others) == 0 {
		return out, nil
	}
	var rows []model.Friendship
	err := r.db.WithContext(ctx).
		Where("(user_low_id = ? AND user_high_id IN ?) OR (user_high_id = ? AND user_low_id IN ?)",
			viewer, others, viewer, others).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OtherUser(viewer)] = row
	}
	return out, nil
}

// MissingFriends returns ids in otherIDs that are not accepted friends of userID.
func (r *FriendshipRepository) MissingFriends(ctx context.Context, userID uuid.UUID, otherIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(otherIDs) == 0 {
		return nil, nil
	}
	var rows []model.Friendship
	err := r.db.WithContext(ctx).
		Where("status = ?", enum.FriendStatusAccepted.String()).
		Where("(user_low_id = ? AND user_high_id IN ?) OR (user_high_id = ? AND user_low_id IN ?)",
			userID, otherIDs, userID, otherIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	found := map[uuid.UUID]struct{}{}
	for _, row := range rows {
		found[row.OtherUser(userID)] = struct{}{}
	}
	missing := make([]uuid.UUID, 0)
	for _, id := range otherIDs {
		if _, ok := found[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing, nil
}

type friendScan struct {
	UserID      uuid.UUID `gorm:"column:user_id"`
	Username    string    `gorm:"column:username"`
	Status      string    `gorm:"column:status"`
	RequesterID uuid.UUID `gorm:"column:requester_id"`
}

type userScan struct {
	UserID       uuid.UUID `gorm:"column:user_id"`
	Username     string    `gorm:"column:username"`
	PhoneMatched bool      `gorm:"column:phone_matched"`
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
