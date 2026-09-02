package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Litchi-drop/TheMajorTravel/backend/internal/service"
)

// TripHandler 行程与点位接口（报告 §6：GET /api/trip、GET /api/trip/version、
// POST/PATCH/DELETE /api/places、GET /api/places/:id/revisions）
type TripHandler struct {
	trips *service.TripService
}

func NewTripHandler(trips *service.TripService) *TripHandler {
	return &TripHandler{trips: trips}
}

func (h *TripHandler) GetTrip(c *gin.Context) {
	v, err := h.trips.GetTrip(c.Request.Context())
	if err != nil {
		respondTripErr(c, err)
		return
	}
	OK(c, v)
}

func (h *TripHandler) Version(c *gin.Context) {
	v, u, err := h.trips.Version(c.Request.Context())
	if err != nil {
		respondTripErr(c, err)
		return
	}
	OK(c, gin.H{"version": v, "updatedAt": u})
}

func (h *TripHandler) CreatePlace(c *gin.Context) {
	var req struct {
		DayID     int64   `json:"dayId" binding:"required"`
		Name      string  `json:"name" binding:"required"`
		Type      string  `json:"type" binding:"required"`
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		Desc      string  `json:"desc"`
		Ferry     bool    `json:"ferry"`
		SortOrder int     `json:"sortOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Err(c, 400, "请求参数不完整，请检查后重试")
		return
	}
	v, err := h.trips.CreatePlace(c.Request.Context(), tripUID(c), service.CreatePlaceInput{
		DayID: req.DayID, Name: req.Name, Type: req.Type,
		Lat: req.Lat, Lng: req.Lng, Desc: req.Desc,
		Ferry: req.Ferry, SortOrder: req.SortOrder,
	})
	if err != nil {
		respondTripErr(c, err)
		return
	}
	OK(c, v)
}

func (h *TripHandler) UpdatePlace(c *gin.Context) {
	id, ok := placeID(c)
	if !ok {
		return
	}
	// 全部字段可选：只传想改的；服务端与库中现值做字段级 diff
	var req struct {
		Name      *string  `json:"name"`
		Type      *string  `json:"type"`
		Lat       *float64 `json:"lat"`
		Lng       *float64 `json:"lng"`
		Desc      *string  `json:"desc"`
		Ferry     *bool    `json:"ferry"`
		DayID     *int64   `json:"dayId"`
		SortOrder *int     `json:"sortOrder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Err(c, 400, "请求参数不完整，请检查后重试")
		return
	}
	v, err := h.trips.UpdatePlace(c.Request.Context(), tripUID(c), id, service.UpdatePlaceInput{
		Name: req.Name, Type: req.Type, Lat: req.Lat, Lng: req.Lng,
		Desc: req.Desc, Ferry: req.Ferry, DayID: req.DayID, SortOrder: req.SortOrder,
	})
	if err != nil {
		respondTripErr(c, err)
		return
	}
	OK(c, v)
}

func (h *TripHandler) DeletePlace(c *gin.Context) {
	id, ok := placeID(c)
	if !ok {
		return
	}
	if err := h.trips.DeletePlace(c.Request.Context(), tripUID(c), id); err != nil {
		respondTripErr(c, err)
		return
	}
	OK(c, gin.H{"message": "已删除（可在历史中追溯）"})
}

func (h *TripHandler) Revisions(c *gin.Context) {
	id, ok := placeID(c)
	if !ok {
		return
	}
	list, err := h.trips.Revisions(c.Request.Context(), id)
	if err != nil {
		respondTripErr(c, err)
		return
	}
	OK(c, list)
}

func tripUID(c *gin.Context) int64 {
	return c.MustGet("uid").(int64)
}

func placeID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		Err(c, 400, "点位 ID 不合法")
		return 0, false
	}
	return id, true
}

// 哨兵错误 → 状态码（中文文案已可直出）
func respondTripErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDayNotFound),
		errors.Is(err, service.ErrPlaceGone):
		Err(c, 404, err.Error())
	case errors.Is(err, service.ErrStale):
		Err(c, 409, err.Error())
	case errors.Is(err, service.ErrNoChange),
		errors.Is(err, service.ErrPlaceName),
		errors.Is(err, service.ErrPlaceType),
		errors.Is(err, service.ErrCoords):
		Err(c, 400, err.Error())
	default:
		c.Error(err)
		Err(c, 500, "服务器开小差了，请稍后再试")
	}
}
