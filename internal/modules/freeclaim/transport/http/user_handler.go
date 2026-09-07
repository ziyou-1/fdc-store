package freeclaimhttp

import (
	"errors"

	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// FreeClaimService 是用户端免费领取所需的最小端口。
type FreeClaimService interface {
	TodayItem(userID uint) (*freeclaimcontract.TodayItem, error)
	Claim(userID uint) (*freeclaimcontract.ClaimResult, error)
	ListClaims(userID uint, page, pageSize int) ([]freeclaimdomain.Claim, int64, error)
}

// UserHandler 处理用户免费领取请求。
type UserHandler struct {
	claims FreeClaimService
}

// NewUserHandler 构造用户免费领取处理器。
func NewUserHandler(claims FreeClaimService) *UserHandler {
	if claims == nil {
		panic("free claim user handler: required dependency is nil")
	}
	return &UserHandler{claims: claims}
}

// GetToday 返回今日可领取的商品与当前用户的领取状态。
// 未登录也可访问：首页要展示"今日免费商品"，登录态只影响 claimed 字段。
func (h *UserHandler) GetToday(c *gin.Context) {
	// 这里刻意不用 ginutil.GetUserID：它在缺少 user_id 时会自行写出 401 响应，
	// 而本端点在未登录时也必须正常返回（首页要展示今日免费商品）。
	uid := optionalUserID(c)
	item, err := h.claims.TodayItem(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.Success(c, item)
}

// Claim 领取今日免费商品。
func (h *UserHandler) Claim(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	result, err := h.claims.Claim(uid)
	if err != nil {
		respondClaimError(c, err)
		return
	}
	response.Success(c, result)
}

// ListClaims 返回当前用户的领取历史。
func (h *UserHandler) ListClaims(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	records, total, err := h.claims.ListClaims(uid, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.SuccessWithPage(c, records, response.BuildPagination(page, pageSize, total))
}

func respondClaimError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, freeclaimcontract.ErrDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.free_claim_disabled", err)
	case errors.Is(err, freeclaimcontract.ErrMemberOnly),
		errors.Is(err, freeclaimcontract.ErrGuestForbidden):
		ginutil.RespondError(c, response.CodeBadRequest, "error.free_claim_member_only", err)
	case errors.Is(err, freeclaimcontract.ErrAlreadyClaimed):
		ginutil.RespondError(c, response.CodeBadRequest, "error.free_claim_already_claimed", err)
	case errors.Is(err, freeclaimcontract.ErrPoolEmpty):
		ginutil.RespondError(c, response.CodeBadRequest, "error.free_claim_pool_empty", err)
	case errors.Is(err, freeclaimcontract.ErrItemMismatch):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	default:
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	}
}

// optionalUserID 读取可选的登录用户 ID，未登录返回 0 且不写任何响应。
func optionalUserID(c *gin.Context) uint {
	if c == nil {
		return 0
	}
	raw, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	switch value := raw.(type) {
	case uint:
		return value
	case int:
		if value > 0 {
			return uint(value)
		}
	case float64:
		if value > 0 {
			return uint(value)
		}
	}
	return 0
}
