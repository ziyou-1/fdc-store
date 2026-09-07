package memberlevelhttp

import (
	"errors"
	"strings"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type UserPurchaseService interface {
	CreatePurchase(input memberlevelcontract.PurchasePaymentInput) (*memberlevelcontract.PurchaseResult, error)
}

type UserHandler struct {
	purchases UserPurchaseService
}

func NewUserHandler(purchases UserPurchaseService) *UserHandler {
	if purchases == nil {
		panic("memberlevel user handler: purchases is nil")
	}
	return &UserHandler{purchases: purchases}
}

type purchaseRequest struct {
	Period    string `json:"period" binding:"required"`
	ChannelID uint   `json:"channel_id" binding:"required"`
	Currency  string `json:"currency"`
	Remark    string `json:"remark" binding:"max=200"`
}

func (h *UserHandler) CreatePurchase(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req purchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	result, err := h.purchases.CreatePurchase(memberlevelcontract.PurchasePaymentInput{
		UserID:        uid,
		ChannelID:     req.ChannelID,
		Currency:      strings.TrimSpace(req.Currency),
		Remark:        req.Remark,
		ClientIP:      c.ClientIP(),
		Context:       c.Request.Context(),
		RequestScheme: membershipRequestScheme(c),
		Period:        strings.TrimSpace(req.Period),
	})
	if err != nil {
		respondMembershipPurchaseError(c, err)
		return
	}
	response.Success(c, result)
}

func respondMembershipPurchaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, memberlevelcontract.ErrInvalidPeriod):
		ginutil.RespondError(c, response.CodeBadRequest, "error.membership_period_invalid", err)
	case errors.Is(err, memberlevelcontract.ErrPlanNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.membership_plan_not_found", err)
	case errors.Is(err, memberlevelcontract.ErrPlanInactive):
		ginutil.RespondError(c, response.CodeBadRequest, "error.membership_plan_inactive", err)
	case errors.Is(err, memberlevelcontract.ErrPlanNotPriced):
		ginutil.RespondError(c, response.CodeBadRequest, "error.membership_plan_not_priced", err)
	case errors.Is(err, memberlevelcontract.ErrPurchaseDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.membership_purchase_disabled", err)
	case errors.Is(err, memberlevelcontract.ErrUserNotFound):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.unauthorized", err)
	default:
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	}
}

func membershipRequestScheme(c *gin.Context) string {
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
