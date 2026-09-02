package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
)

// ErrStale 乐观锁冲突：提交时 version 已被并发请求推进
var ErrStale = errors.New("stale place version")

type PlaceRepo struct {
	db *gorm.DB
}

func NewPlaceRepo(db *gorm.DB) *PlaceRepo {
	return &PlaceRepo{db: db}
}

func (r *PlaceRepo) ByID(ctx context.Context, id int64) (*model.Place, error) {
	var p model.Place
	err := r.db.WithContext(ctx).First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询点位: %w", err)
	}
	return &p, nil
}

// Create 建点位 + 首条 create 版本 + 触碰行程 updated_at，同一事务
func (r *PlaceRepo) Create(ctx context.Context, tripID int64, p *model.Place, rev *model.PlaceRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(p).Error; err != nil {
			return fmt.Errorf("创建点位: %w", err)
		}
		rev.PlaceID = p.ID
		if err := tx.Create(rev).Error; err != nil {
			return fmt.Errorf("创建点位版本: %w", err)
		}
		return touchTrip(tx, tripID)
	})
}

// ApplyUpdate 带 version 乐观锁的部分更新：sets 内含 version/updated_by/updated_at
func (r *PlaceRepo) ApplyUpdate(ctx context.Context, tripID, placeID int64, oldVersion int, sets map[string]any, rev *model.PlaceRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Place{}).
			Where("id = ? AND version = ?", placeID, oldVersion).
			Updates(sets)
		if res.Error != nil {
			return fmt.Errorf("更新点位: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrStale
		}
		rev.PlaceID = placeID
		if err := tx.Create(rev).Error; err != nil {
			return fmt.Errorf("写入点位版本: %w", err)
		}
		return touchTrip(tx, tripID)
	})
}

// SoftDelete 软删（gorm.DeletedAt）+ delete 版本；行还在，历史可追溯
func (r *PlaceRepo) SoftDelete(ctx context.Context, tripID int64, p *model.Place, rev *model.PlaceRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(p).Error; err != nil {
			return fmt.Errorf("删除点位: %w", err)
		}
		rev.PlaceID = p.ID
		if err := tx.Create(rev).Error; err != nil {
			return fmt.Errorf("写入点位版本: %w", err)
		}
		return touchTrip(tx, tripID)
	})
}

func touchTrip(tx *gorm.DB, tripID int64) error {
	return tx.Model(&model.Trip{}).Where("id = ?", tripID).Update("updated_at", time.Now()).Error
}
