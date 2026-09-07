package application

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// SubmitManualFormInput 支付成功后补填人工交付表单的入参。
// Submissions 以 ItemKey(productID, skuID) 为键，与下单时的 manual_form_data 保持同一契约。
type SubmitManualFormInput struct {
	UserID      uint
	OrderNo     string
	Submissions map[string]jsonmap.JSON
}

// ManualFormPendingItem 描述一条待补填的订单条目。
type ManualFormPendingItem struct {
	OrderNo   string       `json:"order_no"`
	ItemID    uint         `json:"item_id"`
	ProductID uint         `json:"product_id"`
	SKUID     uint         `json:"sku_id"`
	ItemKey   string       `json:"item_key"`
	Schema    jsonmap.JSON `json:"schema"`
}

// SubmitManualForm 在订单支付成功后写入人工交付表单提交值。
// 只接受"已支付/交付中"的订单，且每个条目只能提交一次；提交后运营侧在 SLA 内处理。
func (s *OrderService) SubmitManualForm(input SubmitManualFormInput) (*orderdomain.Order, error) {
	orderNo := strings.TrimSpace(input.OrderNo)
	if input.UserID == 0 || orderNo == "" {
		return nil, ErrOrderNotFound
	}

	order, err := s.orderStore.GetAnyByOrderNoAndUser(orderNo, input.UserID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if !isManualFormCollectable(order) {
		return nil, ErrOrderStatusInvalid
	}

	pending := collectManualFormPendingItems(order)
	if len(pending) == 0 {
		return nil, ErrManualFormNotPending
	}

	now := time.Now()
	applied := 0
	for _, item := range pending {
		submission, ok := input.Submissions[item.ItemKey]
		if !ok {
			continue
		}
		_, normalizedSubmission, validateErr := manualform.ValidateAndNormalize(item.Schema, submission)
		if validateErr != nil {
			return nil, validateErr
		}
		affected, updateErr := s.orderStore.UpdateOrderItemManualSubmission(
			itemOrderID(order, item), item.ItemID, normalizedSubmission, now,
		)
		if updateErr != nil {
			return nil, ErrOrderUpdateFailed
		}
		if affected > 0 {
			applied++
		}
	}
	if applied == 0 {
		return nil, ErrManualFormNotPending
	}

	refreshed, err := s.orderStore.GetAnyByOrderNoAndUser(orderNo, input.UserID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	return refreshed, nil
}

// ListManualFormPendingItems 返回订单中仍待补填的人工交付表单条目。
func (s *OrderService) ListManualFormPendingItems(userID uint, orderNo string) ([]ManualFormPendingItem, error) {
	orderNo = strings.TrimSpace(orderNo)
	if userID == 0 || orderNo == "" {
		return nil, ErrOrderNotFound
	}
	order, err := s.orderStore.GetAnyByOrderNoAndUser(orderNo, userID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if !isManualFormCollectable(order) {
		return []ManualFormPendingItem{}, nil
	}
	return collectManualFormPendingItems(order), nil
}

// isManualFormCollectable 只有支付完成后的订单才允许补填表单。
func isManualFormCollectable(order *orderdomain.Order) bool {
	if order == nil {
		return false
	}
	switch order.Status {
	case constants.OrderStatusPaid, constants.OrderStatusFulfilling:
		return true
	default:
		// 父订单本身可能已按子订单聚合成其它状态，此时以子订单为准。
		for i := range order.Children {
			if isManualFormCollectable(&order.Children[i]) {
				return true
			}
		}
		return false
	}
}

// collectManualFormPendingItems 收集所有"有表单结构且尚未提交"的条目，含子订单。
func collectManualFormPendingItems(order *orderdomain.Order) []ManualFormPendingItem {
	if order == nil {
		return nil
	}
	pending := make([]ManualFormPendingItem, 0, len(order.Items))
	appendFrom := func(target *orderdomain.Order) {
		if target == nil || !isManualFormCollectable(target) {
			return
		}
		for i := range target.Items {
			item := &target.Items[i]
			if strings.TrimSpace(item.FulfillmentType) != constants.FulfillmentTypeManual {
				continue
			}
			if len(item.ManualFormSchemaSnapshotJSON) == 0 {
				continue
			}
			if len(item.ManualFormSubmissionJSON) > 0 {
				continue
			}
			pending = append(pending, ManualFormPendingItem{
				OrderNo:   target.OrderNo,
				ItemID:    item.ID,
				ProductID: item.ProductID,
				SKUID:     item.SKUID,
				ItemKey:   orderdomain.ItemKey(item.ProductID, item.SKUID),
				Schema:    item.ManualFormSchemaSnapshotJSON,
			})
		}
	}

	if len(order.Children) > 0 {
		for i := range order.Children {
			appendFrom(&order.Children[i])
		}
		return pending
	}
	appendFrom(order)
	return pending
}

// itemOrderID 找到条目所属订单的主键：父订单下的条目挂在对应子订单上。
func itemOrderID(order *orderdomain.Order, item ManualFormPendingItem) uint {
	if order == nil {
		return 0
	}
	if len(order.Children) > 0 {
		for i := range order.Children {
			if order.Children[i].OrderNo == item.OrderNo {
				return order.Children[i].ID
			}
		}
		return 0
	}
	return order.ID
}

// ManualFormDeadline 返回人工交付的承诺处理截止时间（支付时间 + SLA 小时数）。
func ManualFormDeadline(paidAt *time.Time) *time.Time {
	if paidAt == nil {
		return nil
	}
	deadline := paidAt.Add(time.Duration(constants.ManualFormSLAHours) * time.Hour)
	return &deadline
}
