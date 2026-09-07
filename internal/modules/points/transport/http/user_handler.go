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

// PointsService 是用户端积分接口所需的最小端口。
type PointsService interface {
	GetAccount(userID uint) (*pointsdomain.Account, error)
	ListTransactions(filter pointscontract.TransactionListFilter) ([]pointsdomain.Transaction, int64, error)
	Checkin(userID uint) (*pointscontract.CheckinResult, error)
	CheckinStatus(userID uint) (*pointscontract.CheckinStatus, error)
	CreateRecharge(input pointscontract.RechargeInput) (*pointscontract.RechargeResult, error)
	ListRecharges(userID uint, page, pageSize int, status string) ([]pointscontract.RechargeRecord, int64, error)
	GetConfig() (*pointsdomain.Config, error)
	GetInvite(userID uint) (*pointscontract.Invite, error)
}

// UserHandler 处理用户积分 HTTP 请求。
type UserHandler struct {
	points PointsService
}

// NewUserHandler 构造用户积分处理器。
func NewUserHandler(points PointsService) *UserHandler {
	if points == nil {
		panic("points user handler: required dependency is nil")
	}
	return &UserHandler{points: points}
}

// GetPoints 返回当前用户积分账户。
func (h *UserHandler) GetPoints(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	account, err := h.points.GetAccount(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, pointspresenter.NewAccountResp(account))
}

// GetTransactions 分页返回当前用户积分流水。
func (h *UserHandler) GetTransactions(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	records, total, err := h.points.ListTransactions(pointscontract.TransactionListFilter{
		Page:      page,
		PageSize:  pageSize,
		UserID:    uid,
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

// GetConfig 返回买家可见的积分规则。买家在充值前必须能看到兑换比例，
// 因此这里下发规则子集，不需要任何权限。
func (h *UserHandler) GetConfig(c *gin.Context) {
	if _, ok := ginutil.GetUserID(c); !ok {
		return
	}
	config, err := h.points.GetConfig()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, pointspresenter.NewPublicConfigResp(config))
}

// GetInvite 返回当前用户的推荐邀请码与奖励额度。
func (h *UserHandler) GetInvite(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	invite, err := h.points.GetInvite(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, pointspresenter.NewInviteResp(invite))
}

// GetCheckinStatus 返回今日签到状态。
func (h *UserHandler) GetCheckinStatus(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	status, err := h.points.CheckinStatus(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, status)
}

// Checkin 执行每日签到。重复签到与签到关闭都按 400 返回，便于前台区分提示。
func (h *UserHandler) Checkin(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	result, err := h.points.Checkin(uid)
	if err != nil {
		switch {
		case errors.Is(err, pointscontract.ErrAlreadyCheckedIn):
			ginutil.RespondError(c, response.CodeBadRequest, "points.already_checked_in", err)
		case errors.Is(err, pointscontract.ErrCheckinDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "points.checkin_disabled", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		}
		return
	}
	response.Success(c, pointspresenter.CheckinResp{
		Points:     result.Points,
		StreakDays: result.StreakDays,
		Balance:    result.Balance,
		Date:       result.Date,
	})
}

// rechargeRequest 用户发起积分充值的请求体。
type rechargeRequest struct {
	ChannelID uint   `json:"channel_id" binding:"required"`
	Amount    string `json:"amount" binding:"required"`
	Currency  string `json:"currency"`
	Remark    string `json:"remark" binding:"max=200"`
}

// CreateRecharge 发起积分充值。渠道白名单与金额校验完全复用钱包充值链路，
// 因此前台可以直接用 POST /wallet/payment-channels 拉取可用渠道。
func (h *UserHandler) CreateRecharge(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var request rechargeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(request.Amount))
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	result, err := h.points.CreateRecharge(pointscontract.RechargeInput{
		UserID:        uid,
		ChannelID:     request.ChannelID,
		Amount:        money.FromDecimal(amount.Round(2)),
		Currency:      strings.TrimSpace(request.Currency),
		Remark:        request.Remark,
		ClientIP:      c.ClientIP(),
		Context:       c.Request.Context(),
		RequestScheme: requestScheme(c),
	})
	if err != nil {
		respondRechargeError(c, err)
		return
	}
	response.Success(c, result)
}

// ListRecharges 分页返回当前用户的积分充值记录。
func (h *UserHandler) ListRecharges(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	records, total, err := h.points.ListRecharges(uid, page, pageSize, c.Query("status"))
	if err != nil {
		respondRechargeError(c, err)
		return
	}
	response.SuccessWithPage(c, records, response.BuildPagination(page, pageSize, total))
}

func respondRechargeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pointscontract.ErrRechargeDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_recharge_disabled", err)
	case errors.Is(err, pointscontract.ErrRechargeChannelInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_recharge_channel_invalid", err)
	case errors.Is(err, pointscontract.ErrRechargeRateInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_recharge_rate_invalid", err)
	case errors.Is(err, pointscontract.ErrInvalidAmount):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	case errors.Is(err, pointscontract.ErrExchangeGuestForbidden):
		ginutil.RespondError(c, response.CodeBadRequest, "error.points_exchange_not_supported", err)
	default:
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	}
}

// requestScheme 解析当前请求的 scheme，供支付网关生成回跳地址。
func requestScheme(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return "https"
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}
