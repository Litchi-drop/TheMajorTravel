// Package seed 首次启动把演示版蓝本写入数据库（幂等：trips 表非空即跳过）。
// 数据源为仓库根 index.html（冻结演示版）的 BLUEPRINT，人工转录于 Step 2。
package seed

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
)

//go:embed blueprint.json
var blueprintJSON []byte

type blueprintFile struct {
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Days      []struct {
		DayDate   string `json:"dayDate"`
		Title     string `json:"title"`
		Color     string `json:"color"`
		SortOrder int    `json:"sortOrder"`
		Places    []struct {
			Name      string  `json:"name"`
			Type      string  `json:"type"`
			Lat       float64 `json:"lat"`
			Lng       float64 `json:"lng"`
			Desc      string  `json:"desc"`
			Ferry     bool    `json:"ferry"`
			SortOrder int     `json:"sortOrder"`
		} `json:"places"`
	} `json:"days"`
}

// Load trips 表为空时写入蓝本（行程 + 8 天 + 46 点位），无任何用户归属（前端显示「蓝本」）
func Load(db *gorm.DB) error {
	var n int64
	if err := db.Model(&model.Trip{}).Count(&n).Error; err != nil {
		return fmt.Errorf("统计行程: %w", err)
	}
	if n > 0 {
		return nil // 已有数据（含用户改过的行程），绝不覆盖
	}

	var bp blueprintFile
	if err := json.Unmarshal(blueprintJSON, &bp); err != nil {
		return fmt.Errorf("解析 blueprint.json: %w", err)
	}
	sd, err := time.Parse("2006-01-02", bp.StartDate)
	if err != nil {
		return fmt.Errorf("startDate %q: %w", bp.StartDate, err)
	}
	ed, err := time.Parse("2006-01-02", bp.EndDate)
	if err != nil {
		return fmt.Errorf("endDate %q: %w", bp.EndDate, err)
	}

	var placeCount int
	err = db.Transaction(func(tx *gorm.DB) error {
		trip := model.Trip{Title: bp.Title, StartDate: &sd, EndDate: &ed, Visibility: "private"}
		if err := tx.Create(&trip).Error; err != nil {
			return fmt.Errorf("写入行程: %w", err)
		}
		for _, d := range bp.Days {
			date, err := time.Parse("2006-01-02", d.DayDate)
			if err != nil {
				return fmt.Errorf("dayDate %q: %w", d.DayDate, err)
			}
			day := model.Day{TripID: trip.ID, DayDate: &date, Title: d.Title, Color: d.Color, SortOrder: d.SortOrder}
			if err := tx.Create(&day).Error; err != nil {
				return fmt.Errorf("写入天 %s: %w", d.Title, err)
			}
			for _, p := range d.Places {
				place := model.Place{
					DayID: day.ID, Name: p.Name, Type: p.Type,
					Lat: p.Lat, Lng: p.Lng, Desc: p.Desc, Ferry: p.Ferry,
					SortOrder: p.SortOrder, Version: 1,
				}
				if err := tx.Create(&place).Error; err != nil {
					return fmt.Errorf("写入点位 %s: %w", p.Name, err)
				}
				placeCount++
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Printf("蓝本种子写入完成: %d 天 %d 点位（%s ~ %s）", len(bp.Days), placeCount, bp.StartDate, bp.EndDate)
	return nil
}
