package tokenstationhttp

import (
	"errors"

	tokenstationapp "github.com/dujiao-next/internal/modules/tokenstation/application"
	tokenstationcontract "github.com/dujiao-next/internal/modules/tokenstation/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminHandler 处理后台别人中转站链接。
type AdminHandler struct {
	stations *tokenstationapp.Service
}

func NewAdminHandler(stations *tokenstationapp.Service) *AdminHandler {
	if stations == nil {
		panic("token station admin handler: required dependency is nil")
	}
	return &AdminHandler{stations: stations}
}

type stationRequest struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

func (h *AdminHandler) ListStations(c *gin.Context) {
	items, err := h.stations.ListStations()
	if err != nil {
		respondTokenStationError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *AdminHandler) CreateStation(c *gin.Context) {
	var request stationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	station, err := h.stations.CreateStation(tokenstationcontract.StationInput{
		Name:    request.Name,
		BaseURL: request.BaseURL,
	})
	if err != nil {
		respondTokenStationError(c, err)
		return
	}
	response.Success(c, station)
}

func (h *AdminHandler) UpdateStation(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var request stationRequest
	if bindErr := c.ShouldBindJSON(&request); bindErr != nil {
		ginutil.RespondBindError(c, bindErr)
		return
	}
	station, updateErr := h.stations.UpdateStation(id, tokenstationcontract.StationInput{
		Name:    request.Name,
		BaseURL: request.BaseURL,
	})
	if updateErr != nil {
		respondTokenStationError(c, updateErr)
		return
	}
	response.Success(c, station)
}

func (h *AdminHandler) DeleteStation(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if deleteErr := h.stations.DeleteStation(id); deleteErr != nil {
		respondTokenStationError(c, deleteErr)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func classifyTokenStationError(err error) (int, string) {
	switch {
	case errors.Is(err, tokenstationcontract.ErrStationInvalid):
		return response.CodeBadRequest, "error.token_station_invalid"
	case errors.Is(err, tokenstationcontract.ErrStationNotFound):
		return response.CodeNotFound, "error.token_station_not_found"
	default:
		return response.CodeBadRequest, "error.bad_request"
	}
}
