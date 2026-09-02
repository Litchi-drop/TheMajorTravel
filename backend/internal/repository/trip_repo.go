package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
)

type TripRepo struct {
	db *gorm.DB
}

func NewTripRepo(db *gorm.DB) *TripRepo {
	return &TripRepo{db: db}
}

// First 单行程 MVP：取 id 最小的行程（报告 §4.2 Step 2 范围）
func (r *TripRepo) First(ctx context.Context) (*model.Trip, error) {
	var t model.Trip
	err := r.db.WithContext(ctx).Order("id ASC").First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询行程: %w", err)
	}
	return &t, nil
}

func (r *TripRepo) DayByID(ctx context.Context, id int64) (*model.Day, error) {
	var d model.Day
	err := r.db.WithContext(ctx).First(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询天: %w", err)
	}
	return &d, nil
}

// DaysWithPlaces 一次取出全部天与点位（含软删除过滤），点位按 sort_order 排
func (r *TripRepo) DaysWithPlaces(ctx context.Context, tripID int64) ([]model.Day, error) {
	var days []model.Day
	err := r.db.WithContext(ctx).
		Preload("Places", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, id ASC")
		}).
		Where("trip_id = ?", tripID).
		Order("sort_order ASC, id ASC").
		Find(&days).Error
	if err != nil {
		return nil, fmt.Errorf("查询天与点位: %w", err)
	}
	return days, nil
}
