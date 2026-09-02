package service

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/model"
	"github.com/Litchi-drop/TheMajorTravel/backend/internal/repository"
)

// 行程域哨兵错误（文案可直接展示，见报告 §5.3）
var (
	ErrTripEmpty   = errors.New("行程数据未初始化，请联系管理员")
	ErrDayNotFound = errors.New("所选日期不存在")
	ErrPlaceGone   = errors.New("点位不存在或已删除")
	ErrStale       = errors.New("该点位已被他人修改，请刷新页面后重试")
	ErrNoChange    = errors.New("没有检测到修改内容")
	ErrPlaceName   = errors.New("请填写点位名称")
	ErrPlaceType   = errors.New("请选择点位类型")
	ErrCoords      = errors.New("经纬度超出范围")
)

const (
	versionKey   = "mt:trip:version"
	updatedAtKey = "mt:trip:updated_at"
)

type TripService struct {
	trips     *repository.TripRepo
	places    *repository.PlaceRepo
	revisions *repository.RevisionRepo
	users     *repository.UserRepo
	rdb       *redis.Client
}

func NewTripService(trips *repository.TripRepo, places *repository.PlaceRepo, revisions *repository.RevisionRepo, users *repository.UserRepo, rdb *redis.Client) *TripService {
	return &TripService{trips: trips, places: places, revisions: revisions, users: users, rdb: rdb}
}

// ---------- 视图 DTO（扁平：days 只装天，places 平铺带 dayId） ----------

type PlaceView struct {
	ID              int64   `json:"id"`
	DayID           int64   `json:"dayId"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Lat             float64 `json:"lat"`
	Lng             float64 `json:"lng"`
	Desc            string  `json:"desc"`
	Ferry           bool    `json:"ferry"`
	SortOrder       int     `json:"sortOrder"`
	Version         int     `json:"version"`
	CreatedBy       *int64  `json:"createdBy"`
	UpdatedBy       *int64  `json:"updatedBy"`
	UpdatedByName   string  `json:"updatedByName"` // 空串 = 蓝本（无归属用户）
	UpdatedByAvatar string  `json:"updatedByAvatar"`
	UpdatedAt       *string `json:"updatedAt"`
}

type DayView struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"`
	Title     string `json:"title"`
	Color     string `json:"color"`
	SortOrder int    `json:"sortOrder"`
}

type TripView struct {
	ID        int64       `json:"id"`
	Title     string      `json:"title"`
	StartDate string      `json:"startDate"`
	EndDate   string      `json:"endDate"`
	UpdatedAt string      `json:"updatedAt"`
	Days      []DayView   `json:"days"`
	Places    []PlaceView `json:"places"`
}

// ---------- 查询 ----------

func (s *TripService) GetTrip(ctx context.Context) (*TripView, error) {
	trip, days, err := s.tripWithDays(ctx)
	if err != nil {
		return nil, err
	}

	uidSet := map[int64]struct{}{}
	for _, d := range days {
		for _, p := range d.Places {
			if p.UpdatedBy != nil {
				uidSet[*p.UpdatedBy] = struct{}{}
			}
		}
	}
	ids := make([]int64, 0, len(uidSet))
	for id := range uidSet {
		ids = append(ids, id)
	}
	users, err := s.users.ByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	view := &TripView{
		ID: trip.ID, Title: trip.Title,
		StartDate: fmtDate(trip.StartDate), EndDate: fmtDate(trip.EndDate),
		UpdatedAt: trip.UpdatedAt.Format(time.RFC3339),
		Days:      make([]DayView, 0, len(days)),
		Places:    []PlaceView{},
	}
	for _, d := range days {
		view.Days = append(view.Days, DayView{ID: d.ID, Date: fmtDate(d.DayDate), Title: d.Title, Color: d.Color, SortOrder: d.SortOrder})
		for i := range d.Places {
			view.Places = append(view.Places, placeView(&d.Places[i], users))
		}
	}
	return view, nil
}

// Version 30s 轮询用：只读 Redis；种子态缓存缺失时回读 trips.updated_at 并补写缓存
func (s *TripService) Version(ctx context.Context) (int64, string, error) {
	v, _ := s.rdb.Get(ctx, versionKey).Int64()
	u, err := s.rdb.Get(ctx, updatedAtKey).Result()
	if err == nil {
		return v, u, nil
	}
	trip, _, err := s.tripWithDays(ctx)
	if err != nil {
		return 0, "", err
	}
	u = trip.UpdatedAt.Format(time.RFC3339)
	s.rdb.Set(ctx, updatedAtKey, u, 0)
	return v, u, nil
}

func (s *TripService) Revisions(ctx context.Context, placeID int64) ([]repository.RevisionView, error) {
	return s.revisions.ListByPlace(ctx, placeID, 100)
}

// ---------- 写入 ----------

type CreatePlaceInput struct {
	DayID     int64
	Name      string
	Type      string
	Lat       float64
	Lng       float64
	Desc      string
	Ferry     bool
	SortOrder int
}

type UpdatePlaceInput struct {
	Name      *string
	Type      *string
	Lat       *float64
	Lng       *float64
	Desc      *string
	Ferry     *bool
	DayID     *int64
	SortOrder *int
}

func (s *TripService) CreatePlace(ctx context.Context, uid int64, in CreatePlaceInput) (*PlaceView, error) {
	if in.Name == "" {
		return nil, ErrPlaceName
	}
	if in.Type == "" {
		return nil, ErrPlaceType
	}
	if !validCoords(in.Lat, in.Lng) {
		return nil, ErrCoords
	}
	trip, _, err := s.tripWithDays(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.trips.DayByID(ctx, in.DayID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDayNotFound
		}
		return nil, err
	}

	now := time.Now()
	p := &model.Place{
		DayID: in.DayID, Name: in.Name, Type: in.Type,
		Lat: in.Lat, Lng: in.Lng, Desc: in.Desc, Ferry: in.Ferry,
		SortOrder: in.SortOrder, Version: 1,
		CreatedBy: &uid, UpdatedBy: &uid, UpdatedAt: &now,
	}
	rev := &model.PlaceRevision{UserID: uid, Action: "create", Changes: createChanges(p)}
	if err := s.places.Create(ctx, trip.ID, p, rev); err != nil {
		return nil, err
	}
	s.bumpVersion(ctx)

	users, err := s.users.ByIDs(ctx, []int64{uid})
	if err != nil {
		return nil, err
	}
	return placeViewPtr(p, users), nil
}

func (s *TripService) UpdatePlace(ctx context.Context, uid, placeID int64, in UpdatePlaceInput) (*PlaceView, error) {
	p, err := s.places.ByID(ctx, placeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaceGone
		}
		return nil, err
	}

	changes := model.JSONB{} // 字段级 diff：{"字段":{"old":旧值,"new":新值}}
	sets := map[string]any{} // GORM Updates 列
	if in.Name != nil && *in.Name != p.Name {
		if *in.Name == "" {
			return nil, ErrPlaceName
		}
		changes["name"] = diff(p.Name, *in.Name)
		sets["name"] = *in.Name
	}
	if in.Type != nil && *in.Type != p.Type {
		if *in.Type == "" {
			return nil, ErrPlaceType
		}
		changes["type"] = diff(p.Type, *in.Type)
		sets["type"] = *in.Type
	}
	if in.Lat != nil && *in.Lat != p.Lat {
		if !validCoords(*in.Lat, p.Lng) {
			return nil, ErrCoords
		}
		changes["lat"] = diff(p.Lat, *in.Lat)
		sets["lat"] = *in.Lat
	}
	if in.Lng != nil && *in.Lng != p.Lng {
		if !validCoords(p.Lat, *in.Lng) {
			return nil, ErrCoords
		}
		changes["lng"] = diff(p.Lng, *in.Lng)
		sets["lng"] = *in.Lng
	}
	if in.Desc != nil && *in.Desc != p.Desc {
		changes["desc"] = diff(p.Desc, *in.Desc)
		sets["desc"] = *in.Desc
	}
	if in.Ferry != nil && *in.Ferry != p.Ferry {
		changes["ferry"] = diff(p.Ferry, *in.Ferry)
		sets["ferry"] = *in.Ferry
	}
	if in.DayID != nil && *in.DayID != p.DayID {
		if _, err := s.trips.DayByID(ctx, *in.DayID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrDayNotFound
			}
			return nil, err
		}
		changes["dayId"] = diff(p.DayID, *in.DayID)
		sets["day_id"] = *in.DayID
	}
	if in.SortOrder != nil && *in.SortOrder != p.SortOrder {
		changes["sortOrder"] = diff(p.SortOrder, *in.SortOrder)
		sets["sort_order"] = *in.SortOrder
	}
	if len(changes) == 0 {
		return nil, ErrNoChange
	}

	trip, _, err := s.tripWithDays(ctx)
	if err != nil {
		return nil, err
	}
	sets["version"] = p.Version + 1 // 乐观锁推进
	sets["updated_by"] = uid
	sets["updated_at"] = time.Now()
	rev := &model.PlaceRevision{UserID: uid, Action: "update", Changes: changes}
	if err := s.places.ApplyUpdate(ctx, trip.ID, p.ID, p.Version, sets, rev); err != nil {
		if errors.Is(err, repository.ErrStale) {
			return nil, ErrStale
		}
		return nil, err
	}
	s.bumpVersion(ctx)

	// 回读最新行构造视图（sets 的 key 是列名，手动映射回结构体易漏）
	fresh, err := s.places.ByID(ctx, placeID)
	if err != nil {
		return nil, err
	}
	users, err := s.users.ByIDs(ctx, []int64{uid})
	if err != nil {
		return nil, err
	}
	return placeViewPtr(fresh, users), nil
}

func (s *TripService) DeletePlace(ctx context.Context, uid, placeID int64) error {
	p, err := s.places.ByID(ctx, placeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrPlaceGone
		}
		return err
	}
	trip, _, err := s.tripWithDays(ctx)
	if err != nil {
		return err
	}
	rev := &model.PlaceRevision{UserID: uid, Action: "delete", Changes: model.JSONB{
		"name": diff(p.Name, nil),
		"type": diff(p.Type, nil),
		"lat":  diff(p.Lat, nil),
		"lng":  diff(p.Lng, nil),
	}}
	if err := s.places.SoftDelete(ctx, trip.ID, p, rev); err != nil {
		return err
	}
	s.bumpVersion(ctx)
	return nil
}

// ---------- 内部工具 ----------

// bumpVersion 写操作成功后推进全局版本（轮询信号）；失败不影响已提交的 PG 事务，
// 下一次写操作会再次 INCR，最坏情况是轮询方晚一轮感知
func (s *TripService) bumpVersion(ctx context.Context) {
	s.rdb.Incr(ctx, versionKey)
	s.rdb.Set(ctx, updatedAtKey, time.Now().Format(time.RFC3339), 0)
}

func (s *TripService) tripWithDays(ctx context.Context) (*model.Trip, []model.Day, error) {
	trip, err := s.trips.First(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrTripEmpty
		}
		return nil, nil, err
	}
	days, err := s.trips.DaysWithPlaces(ctx, trip.ID)
	if err != nil {
		return nil, nil, err
	}
	return trip, days, nil
}

func createChanges(p *model.Place) model.JSONB {
	return model.JSONB{
		"name":  diff(nil, p.Name),
		"type":  diff(nil, p.Type),
		"lat":   diff(nil, p.Lat),
		"lng":   diff(nil, p.Lng),
		"desc":  diff(nil, p.Desc),
		"ferry": diff(nil, p.Ferry),
	}
}

func diff(old, new any) map[string]any { return map[string]any{"old": old, "new": new} }

func validCoords(lat, lng float64) bool {
	if lat == 0 && lng == 0 { // 前端漏传时的典型值
		return false
	}
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

func fmtDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func placeView(p *model.Place, users map[int64]*model.User) PlaceView {
	return PlaceView{
		ID: p.ID, DayID: p.DayID, Name: p.Name, Type: p.Type,
		Lat: p.Lat, Lng: p.Lng, Desc: p.Desc, Ferry: p.Ferry,
		SortOrder: p.SortOrder, Version: p.Version,
		CreatedBy: p.CreatedBy, UpdatedBy: p.UpdatedBy,
		UpdatedByName:   userName(p.UpdatedBy, users),
		UpdatedByAvatar: userAvatar(p.UpdatedBy, users),
		UpdatedAt:       fmtTime(p.UpdatedAt),
	}
}

func placeViewPtr(p *model.Place, users map[int64]*model.User) *PlaceView {
	v := placeView(p, users)
	return &v
}

func userName(id *int64, users map[int64]*model.User) string {
	if id == nil {
		return ""
	}
	if u, ok := users[*id]; ok {
		return u.Nickname
	}
	return ""
}

func userAvatar(id *int64, users map[int64]*model.User) string {
	if id == nil {
		return ""
	}
	if u, ok := users[*id]; ok {
		return u.AvatarEmoji
	}
	return ""
}

func fmtTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
