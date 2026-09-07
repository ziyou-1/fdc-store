package pointshttp

import (
	"errors"
	"strings"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
	pointspresenter "github.com/dujiao-next/internal/modules/points/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminPointsService 是后台积分接口所需的最小端口。
type AdminPointsService interface {
	GetAccount(userID uint) (*pointsdomain.Account, error)
	ListTransactions(filter pointscontract.TransactionListFilter) ([]pointsdomain.Transaction, int64, error)
	AdminAdjust(input pointscontract.AdjustInput) (*pointsdomain.Account, *pointsdomain.Transaction, error)
	GetConfig() (*pointsdomain.Config, error)
	UpdateConfig(input pointscontract.ConfigInput) (*pointsdomain.Config, error)
}

// AdminHandler 处理后台积分 HTTP 请求。
type AdminHandler struct {
	points AdminPointsService
}

// NewAdminHandler 构造后台积分处理器。
func NewAdminHandler(points AdminPointsService) *AdminHandler {
	if points == nil {
		panic("points admin handler: required dependency is nil")
	}
	return &AdminHandler{points: points}
}

// adjustPointsRequest 后台人工增减积分请求体。
type adjustPointsRequest struct {
	Delta  int64  `json:"delta" binding:"required"`
	Remark string `json:"remark" binding:"max=200"`
}

// updateConfigRequest 后台修改积分规则请求体。
type updateConfigRequest struct {
	CheckinEnabled              bool   `json:"checkin_enabled"`
	CheckinPoints               int64  `json:"checkin_points" binding:"min=0"`
	ReferralRegisterPoints      int64  `json:"referral_register_points" binding:"min=0"`
	ReferralFirstOrderPoints    int64  `json:"referral_first_order_points" binding:"min=0"`
	RechargePointsPerUnit       int64  `json:"recharge_points_per_unit" binding:"min=0"`
	ExchangeEnabled             bool   `json:"exchange_enabled"`
	ExchangeDailyLimit          int64  `json:"exchange_daily_limit" binding:"min=0"`
	ExchangeSKUDailyLimit       int64  `json:"exchange_sku_daily_limit" binding:"min=0"`
	RechargeEnabled             bool   `json:"recharge_enabled"`
	ReferralEnabled             bool   `json:"referral_enabled"`
	ReferralDailyLimit          int64  `json:"referral_daily_limit" binding:"min=0"`
	ReferralFirstOrderMinAmount string `json:"referral_first_order_min_amount"`
}

// GetUserPoints 返回指定用户的积分账户。
func (h *AdminHandler) GetUserPoints(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	account, err := h.points.GetAccount(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, pointspresenter.NewAccountResp(account))
}

// GetUserTransactions 分页返回指定用户的积分流水。
func (h *AdminHandler) GetUserTransactions(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	records, total, err := h.points.ListTransactions(pointscontract.TransactionListFilter{
		Page:      page,
		PageSize:  pageSize,
		UserID:    userID,
		Type:      c.Query("type"),
		Direction: c.Query("direction"),
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(
		c,
		pointspresenter.NewTransactionRespList(records),
		response.BuildPagination(page, pageSize, total),
	)
}

// AdjustUserPoints 后台人工增减积分，Delta 为负表示扣减。
func (h *AdminHandler) AdjustUserPoints(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var request adjustPointsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	var operator *uint
	if adminID, ok := ginutil.GetAdminID(c); ok {
		operator = &adminID
	}

	account, _, err := h.points.AdminAdjust(pointscontract.AdjustInput{
		UserID:          userID,
		OperatorAdminID: operator,
		Delta:           request.Delta,
		Remark:          request.Remark,
	})
	if err != nil {
		switch {
		case errors.Is(err, pointscontract.ErrInsufficientPoints):
			ginutil.RespondError(c, response.CodeBadRequest, "points.insufficient", err)
		case errors.Is(err, pointscontract.ErrInvalidAmount):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}
	response.Success(c, pointspresenter.NewAccountResp(account))
}

// GetConfig 返回积分规则。
func (h *AdminHandler) GetConfig(c *gin.Context) {
	config, err := h.points.GetConfig()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, pointspresenter.NewConfigResp(config))
}

// UpdateConfig 保存积分规则。
func (h *AdminHandler) UpdateConfig(c *gin.Context) {
	var request updateConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	minAmount := money.FromDecimal(decimal.Zero)
	if raw := strings.TrimSpace(request.ReferralFirstOrderMinAmount); raw != "" {
		parsed, parseErr := decimal.NewFromString(raw)
		if parseErr != nil || parsed.LessThan(decimal.Zero) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", parseErr)
			return
		}
		minAmount = money.FromDecimal(parsed.Round(2))
	}

	config, err := h.points.UpdateConfig(pointscontract.ConfigInput{
		CheckinEnabled:              request.CheckinEnabled,
		CheckinPoints:               request.CheckinPoints,
		ReferralRegisterPoints:      request.ReferralRegisterPoints,
		ReferralFirstOrderPoints:    request.ReferralFirstOrderPoints,
		RechargePointsPerUnit:       request.RechargePointsPerUnit,
		ExchangeEnabled:             request.ExchangeEnabled,
		ExchangeDailyLimit:          request.ExchangeDailyLimit,
		ExchangeSKUDailyLimit:       request.ExchangeSKUDailyLimit,
		RechargeEnabled:             request.RechargeEnabled,
		ReferralEnabled:             request.ReferralEnabled,
		ReferralDailyLimit:          request.ReferralDailyLimit,
		ReferralFirstOrderMinAmount: minAmount,
	})
	if err != nil {
		if errors.Is(err, pointscontract.ErrInvalidConfig) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		if errors.Is(err, pointscontract.ErrReferralLimitRequired) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_referral_limit_required", err)
			return
		}
		if errors.Is(err, pointscontract.ErrRechargeRateInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.points_recharge_rate_invalid", err)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		return
	}
	response.Success(c, pointspresenter.NewConfigResp(config))
}
