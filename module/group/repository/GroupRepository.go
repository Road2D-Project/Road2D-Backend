package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/group/model"
	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActiveGroupByUser is a group plus the caller's role in that group.
type ActiveGroupByUser struct {
	Group model.Group
	Role  enum.GroupRole
}

type GroupRepository struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// Nguyên tắt
// Với create thì bọc transaction bằng các  db.WithContext(ctx).Transaction
// Với select, update, delete thì result := db.WithContext, check result có lỗi và rowEffected
func (r *GroupRepository) CreateGroupWithMembers(ctx context.Context, group *model.Group, members []model.GroupMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		for i := range members {
			members[i].GroupID = group.ID
		}
		if len(members) == 0 {
			return ErrNotEnoughGroupMember
		}
		// GroupMember.TableName() is group_members; GORM inserts that slice as new rows.
		return tx.Create(&members).Error
	})
}

func (r *GroupRepository) FindGroupByID(ctx context.Context, id uuid.UUID) (*model.Group, error) {
	var group model.Group
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGroupNotFound
	}
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *GroupRepository) UpdateGroupInfo(ctx context.Context, id uuid.UUID, name, description *string) error {
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if description != nil {
		updates["description"] = *description
	}
	if len(updates) == 0 {
		return ErrNoGroupUpdate
	}
	result := r.db.WithContext(ctx).Model(&model.Group{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *GroupRepository) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Group{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrGroupNotFound
	}
	return nil
}

func (r *GroupRepository) ListActiveGroupByUser(ctx context.Context, userID uuid.UUID) ([]ActiveGroupByUser, error) {
	var members []model.GroupMember
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, enum.MembershipActive).
		Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []ActiveGroupByUser{}, nil
	}
	ids := make([]uuid.UUID, 0, len(members))
	roleByGroup := make(map[uuid.UUID]enum.GroupRole, len(members))
	for _, m := range members {
		ids = append(ids, m.GroupID)
		roleByGroup[m.GroupID] = m.Role
	}
	var groups []model.Group
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("updated_at DESC").Find(&groups).Error; err != nil {
		return nil, err
	}
	out := make([]ActiveGroupByUser, 0, len(groups))
	for _, g := range groups {
		out = append(out, ActiveGroupByUser{Group: g, Role: roleByGroup[g.ID]})
	}
	return out, nil
}

func (r *GroupRepository) CountGroupTrips(ctx context.Context, groupID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&tripmodel.Trip{}).Where("group_id = ?", groupID).Count(&n).Error
	return n, err
}
