package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
)

// RevisionView 版本列表视图：join users 带出修改人（用户被物理删除时 LEFT JOIN 置空）
type RevisionView struct {
	ID         int64       `json:"id"`
	PlaceID    int64       `json:"placeId"`
	Action     string      `json:"action"`
	Changes    model.JSONB `json:"changes"`
	CreatedAt  time.Time   `json:"createdAt"`
	UserName   string      `json:"userName"`
	UserAvatar string      `json:"userAvatar"`
}

type RevisionRepo struct {
	db *gorm.DB
}

func NewRevisionRepo(db *gorm.DB) *RevisionRepo {
	return &RevisionRepo{db: db}
}

func (r *RevisionRepo) ListByPlace(ctx context.Context, placeID int64, limit int) ([]RevisionView, error) {
	var views []RevisionView
	err := r.db.WithContext(ctx).
		Table("place_revisions").
		Select(`place_revisions.id, place_revisions.place_id, place_revisions.action,
			place_revisions.changes, place_revisions.created_at,
			COALESCE(users.nickname, '') AS user_name, COALESCE(users.avatar_emoji, '') AS user_avatar`).
		Joins("LEFT JOIN users ON users.id = place_revisions.user_id").
		Where("place_revisions.place_id = ?", placeID).
		Order("place_revisions.created_at DESC, place_revisions.id DESC").
		Limit(limit).
		Scan(&views).Error
	if err != nil {
		return nil, fmt.Errorf("查询点位历史: %w", err)
	}
	return views, nil
}
