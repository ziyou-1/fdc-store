package orderhttp

import (
	"errors"

	"github.com/dujiao-next/internal/constants"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// ManualFormService 是支付后补填人工交付表单所需的最小端口。
type ManualFormService interface {
	ListManualFormPendingItems(userID uint, orderNo string) ([]orderapp.ManualFormPendingItem, error)
	SubmitManualForm(input orderapp.SubmitManualFormInput) (*orderdomain.Order, error)
}

// ManualFormHandler 处理"支付成功后补填交付表单"的用户端请求。
type ManualFormHandler struct {
	orders ManualFormService
}

// NewManualFormHandler 构造补填表单处理器。
func NewManualFormHandler(orders ManualFormService) *ManualFormHandler {
	if orders == nil {
		panic("order manual form handler: orders is nil")
	}
	return &ManualFormHandler{orders: orders}
}

// SubmitManualFormRequest 补填请求体。
// Submissions 以 "productID:skuID" 为键；仅有一个待填条目时可只传 Data。
type SubmitManualFormRequest struct {
	Submissions map[string]jsonmap.JSON `json:"submissions"`
	Data        jsonmap.JSON            `json:"data"`
}

// GetPendingManualForms 返回该订单仍待补填的表单结构，供前台弹窗渲染。
func (h *ManualFormHandler) GetPendingManualForms(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	items, err := h.orders.ListManualFormPendingItems(uid, c.Param("order_no"))
	if err != nil {
		respondManualFormError(c, err)
		return
	}
	response.Success(c, gin.H{
		"items":     items,
		"sla_hours": constants.ManualFormSLAHours,
	})
}

// SubmitManualForm 写入补填的表单值。每个条目只能提交一次。
func (h *ManualFormHandler) SubmitManualForm(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var request SubmitManualFormRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	orderNo := c.Param("order_no")
	submissions := request.Submissions
	if len(submissions) == 0 && len(request.Data) > 0 {
		// 单条目场景：前台只有一个输入框时不必关心 item key。
		pending, err := h.orders.ListManualFormPendingItems(uid, orderNo)
		if err != nil {
			respondManualFormError(c, err)
			return
		}
		if len(pending) != 1 {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", orderapp.ErrManualFormNotPending)
			return
		}
		submissions = map[string]jsonmap.JSON{pending[0].ItemKey: request.Data}
	}
	if len(submissions) == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", orderapp.ErrManualFormNotPending)
		return
	}

	order, err := h.orders.SubmitManualForm(orderapp.SubmitManualFormInput{
		UserID:      uid,
		OrderNo:     orderNo,
		Submissions: submissions,
	})
	if err != nil {
		respondManualFormError(c, err)
		return
	}
	response.Success(c, orderpresenter.NewOrderDetail(order))
}

func respondManualFormError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, orderapp.ErrOrderNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", err)
	case errors.Is(err, orderapp.ErrOrderStatusInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", err)
	case errors.Is(err, orderapp.ErrManualFormNotPending):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	case errors.Is(err, orderapp.ErrOrderFetchFailed):
		ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
	case errors.Is(err, orderapp.ErrOrderUpdateFailed):
		ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
	default:
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
	}
}
