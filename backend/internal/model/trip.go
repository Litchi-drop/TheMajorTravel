package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// JSONB place_revisions.changes 的存储类型（字段级 diff，报告 §5.2）
type JSONB map[string]any

func (j JSONB) Value() (driver.Value, error) { return json.Marshal(j) }

func (j *JSONB) Scan(v any) error {
	b, ok := v.([]byte)
	if !ok {
		return errors.New("jsonb: 非 []byte 数据")
	}
	return json.Unmarshal(b, j)
}

type Trip struct {
	ID         int64      `gorm:"primaryKey"`
	Title      string     `gorm:"size:255;not null"`
	StartDate  *time.Time `gorm:"type:date"`
	EndDate    *time.Time `gorm:"type:date"`
	Visibility string     `gorm:"size:16;not null;default:private"` // 三期发布开源启用，一期恒 private
	CreatedBy  *int64     // 蓝本种子无归属人
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Day struct {
	ID        int64      `gorm:"primaryKey"`
	TripID    int64      `gorm:"not null;index"`
	DayDate   *time.Time `gorm:"type:date"`
	Title     string     `gorm:"size:255;not null"`
	Color     string     `gorm:"size:16"`
	SortOrder int        `gorm:"not null"`
	Places    []Place    `gorm:"foreignKey:DayID"`
}

type Place struct {
	ID        int64   `gorm:"primaryKey"`
	DayID     int64   `gorm:"not null"`
	Name      string  `gorm:"size:255;not null"`
	Type      string  `gorm:"size:32;not null"`
	Lat       float64 `gorm:"not null"`
	Lng       float64 `gorm:"not null"`
	Desc      string  `gorm:"column:desc;type:text"`
	Ferry     bool    `gorm:"not null;default:false"` // 到达该站的段为轮渡（沿用演示版语义）
	SortOrder int     `gorm:"not null;default:0"`
	Version   int     `gorm:"not null;default:1"` // 乐观锁：每次更新 +1
	CreatedBy *int64
	UpdatedBy *int64
	UpdatedAt *time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // 软删除，历史可追溯
}

type PlaceRevision struct {
	ID        int64     `gorm:"primaryKey"`
	PlaceID   int64     `gorm:"not null;index:idx_revisions_place,priority:1"`
	UserID    int64     `gorm:"not null"`
	Action    string    `gorm:"size:16;not null"` // create / update / delete
	Changes   JSONB     `gorm:"type:jsonb"`
	CreatedAt time.Time `gorm:"index:idx_revisions_place,priority:2,sort:desc"`
}
