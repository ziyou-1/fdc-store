package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	fulfillmentcontract "github.com/dujiao-next/internal/modules/fulfillment/contract"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	externalidentitycontract "github.com/dujiao-next/internal/modules/identity/externalidentity/contract"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Service 编排人工交付与自动交付。
type Service struct {
	orderStore            ordercontract.Store
	fulfillmentRepo       fulfillmentcontract.Store
	orderQueue            ordercontract.Queue
	botNotifier           BotNotifier
	settingService        *settingsapp.Service
	defaultEmailConfig    config.EmailConfig
	downstreamCallbackSvc DownstreamCallbackEnqueuer
	userOAuthIdentityRepo externalidentitycontract.Store
	agisoPicker           AgisoCardPicker
}

// AgisoCardPicker 是「从 Agiso 卡券库提卡」的最小端口。
//
// 用端口而不是直接依赖 catalog / siteconnection，是为了避免模块间编译循环——
// 与 SetDownstreamCallbackService 同一套做法。
type AgisoCardPicker interface {
	// IsAgisoItem 报告该订单项（商品+SKU）的卡密是否来自 Agiso 卡券库。
	// 来源按 SKU 生效，因此同一商品下不同 SKU 可以分属本地 / 卡券库。
	IsAgisoItem(productID, skuID uint) (bool, error)
	// PickForOrder 按订单项对应的 SKU 向 Agiso 提卡并返回渲染好的卡密文本，条数应等于 quantity。
	// orderNo 作为幂等键传给 Agiso，重复调用返回同一批卡、不重复扣库存。
	PickForOrder(ctx context.Context, productID, skuID uint, quantity int, orderNo string) ([]string, error)
}

// SetAgisoCardPicker 注入 Agiso 提卡端口。未注入时所有商品都按本地卡密库处理。
func (s *Service) SetAgisoCardPicker(p AgisoCardPicker) { s.agisoPicker = p }

type BotNotifier interface {
	EnqueueOrderFulfilled(telegramUserID string, orderID uint) error
}

type DownstreamCallbackEnqueuer interface {
	EnqueueCallback(orderID uint)
}

// SetDownstreamCallbackService 设置下游回调服务（解决循环依赖）
func (s *Service) SetDownstreamCallbackService(svc DownstreamCallbackEnqueuer) {
	s.downstreamCallbackSvc = svc
}

// Options 汇总交付用例依赖。
type Options struct {
	OrderStore            ordercontract.Store
	FulfillmentStore      fulfillmentcontract.Store
	OrderQueue            ordercontract.Queue
	BotNotifier           BotNotifier
	SettingService        *settingsapp.Service
	DefaultEmailConfig    config.EmailConfig
	ExternalIdentityStore externalidentitycontract.Store
}

// New 创建交付服务。
func New(
	opts Options,
) *Service {
	return &Service{
		orderStore:            opts.OrderStore,
		fulfillmentRepo:       opts.FulfillmentStore,
		orderQueue:            opts.OrderQueue,
		botNotifier:           opts.BotNotifier,
		settingService:        opts.SettingService,
		defaultEmailConfig:    opts.DefaultEmailConfig,
		userOAuthIdentityRepo: opts.ExternalIdentityStore,
	}
}

// CreateManualInput 创建人工交付输入
type CreateManualInput struct {
	OrderID      uint
	AdminID      uint
	Payload      string
	DeliveryData jsonmap.JSON
	DeliveredAt  *time.Time
}

// CreateManual 创建人工交付
func (s *Service) CreateManual(input CreateManualInput) (*fulfillmentdomain.Fulfillment, error) {
	if input.OrderID == 0 || input.AdminID == 0 {
		return nil, ErrFulfillmentInvalid
	}
	payload := strings.TrimSpace(input.Payload)
	deliveryData := normalizeManualDeliveryData(input.DeliveryData)
	if payload == "" && len(deliveryData) == 0 {
		return nil, ErrFulfillmentInvalid
	}
	if payload == "" {
		payload = buildManualDeliveryPayload(deliveryData)
	}
	ftype := constants.FulfillmentTypeManual

	order, err := s.orderStore.GetByID(input.OrderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.ParentID == nil && len(order.Children) > 0 {
		return nil, ErrFulfillmentInvalid
	}
	if order.Status != constants.OrderStatusPaid && order.Status != constants.OrderStatusFulfilling {
		return nil, ErrOrderStatusInvalid
	}

	now := time.Now()
	deliveredAt := input.DeliveredAt
	if deliveredAt == nil {
		deliveredAt = &now
	}

	var created *fulfillmentdomain.Fulfillment
	err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		if _, found, err := tx.Fulfillments().FindByOrderIDForUpdate(input.OrderID); err != nil {
			return err
		} else if found {
			return ErrFulfillmentExists
		}

		fulfillment := &fulfillmentdomain.Fulfillment{
			OrderID:       input.OrderID,
			Type:          ftype,
			Status:        constants.FulfillmentStatusDelivered,
			Payload:       payload,
			LogisticsJSON: deliveryData,
			DeliveredBy:   &input.AdminID,
			DeliveredAt:   deliveredAt,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if err := tx.Fulfillments().Create(fulfillment); err != nil {
			return ErrFulfillmentCreateFailed
		}
		if err := tx.Orders().UpdateFields(order.ID, map[string]interface{}{
			"status":     constants.OrderStatusDelivered,
			"updated_at": now,
		}); err != nil {
			return ErrOrderUpdateFailed
		}
		created = fulfillment
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrFulfillmentExists) {
			return nil, ErrFulfillmentExists
		}
		if errors.Is(err, ErrOrderUpdateFailed) {
			return nil, ErrOrderUpdateFailed
		}
		return nil, ErrFulfillmentCreateFailed
	}
	if s.orderQueue != nil {
		if order.ParentID != nil {
			status, syncErr := orderapp.SyncParentStatus(s.orderStore, *order.ParentID, now)
			if syncErr != nil {
				logger.Warnw("fulfillment_sync_parent_status_failed",
					"order_id", order.ID,
					"parent_order_id", *order.ParentID,
					"target_status", constants.OrderStatusDelivered,
					"error", syncErr,
				)
			} else {
				if status == "" {
					status = constants.OrderStatusDelivered
				}
				if status != constants.OrderStatusCanceled {
					if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(s.orderStore, s.orderQueue, s.settingService, s.defaultEmailConfig, *order.ParentID, status); err != nil {
						logger.Warnw("fulfillment_enqueue_status_email_failed",
							"order_id", order.ID,
							"target_order_id", *order.ParentID,
							"status", status,
							"error", err,
						)
					}
				}
			}
		} else {
			if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(s.orderStore, s.orderQueue, s.settingService, s.defaultEmailConfig, input.OrderID, constants.OrderStatusDelivered); err != nil {
				logger.Warnw("fulfillment_enqueue_status_email_failed",
					"order_id", order.ID,
					"target_order_id", input.OrderID,
					"status", constants.OrderStatusDelivered,
					"error", err,
				)
			}
		}
	}
	// Telegram 通知：交付完成后推送给用户
	notifyOrderID := input.OrderID
	if order.ParentID != nil {
		notifyOrderID = *order.ParentID
	}
	go s.NotifyBotOrderFulfilled(order.UserID, notifyOrderID)
	// B 侧：人工交付完成后触发下游回调
	if s.downstreamCallbackSvc != nil {
		s.downstreamCallbackSvc.EnqueueCallback(input.OrderID)
	}
	return created, nil
}

// CreateAuto 自动交付
func (s *Service) CreateAuto(orderID uint) (*fulfillmentdomain.Fulfillment, error) {
	if orderID == 0 {
		return nil, ErrFulfillmentInvalid
	}

	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.ParentID == nil && len(order.Children) > 0 {
		return nil, ErrFulfillmentInvalid
	}
	if order.Status != constants.OrderStatusPaid {
		return nil, ErrOrderStatusInvalid
	}
	if len(order.Items) == 0 {
		return nil, ErrFulfillmentInvalid
	}

	for _, item := range order.Items {
		if strings.TrimSpace(item.FulfillmentType) != constants.FulfillmentTypeAuto {
			return nil, ErrFulfillmentNotAuto
		}
	}

	// 先在事务之外把 Agiso 的卡提回来。
	// 提卡是 HTTP 调用且不可撤销，放进事务一旦回滚就等于白扣一张卡；
	// 而本方法由 asynq 任务驱动、可重试，配合订单号幂等键，先提后写是安全的。
	agisoLines, err := s.pickAgisoCards(order)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var fulfillment *fulfillmentdomain.Fulfillment
	err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		if _, found, err := tx.Fulfillments().FindByOrderIDForUpdate(orderID); err != nil {
			return err
		} else if found {
			return ErrFulfillmentExists
		}

		secretRepo := tx.CardSecrets()
		reservedRows, err := secretRepo.ListByOrderAndStatus(orderID, cardsecretdomain.StatusReserved)
		if err != nil {
			return err
		}
		reservedByKey := make(map[string][]cardsecretdomain.Secret)
		for _, reserved := range reservedRows {
			key := orderdomain.ItemKey(reserved.ProductID, reserved.SKUID)
			reservedByKey[key] = append(reservedByKey[key], reserved)
		}
		var secrets []cardsecretdomain.Secret
		// 按订单项顺序拼装最终交付内容：Agiso 商品用事务外提回的卡，
		// 其余商品照旧从本地卡密库取。
		orderedLines := make([]string, 0, len(order.Items))
		for _, item := range order.Items {
			if item.ProductID == 0 || item.Quantity <= 0 {
				return ErrFulfillmentInvalid
			}
			key := orderdomain.ItemKey(item.ProductID, item.SKUID)
			if lines, ok := agisoLines[key]; ok {
				orderedLines = append(orderedLines, lines...)
				continue
			}
			cachedReserved := reservedByKey[key]
			selected := make([]cardsecretdomain.Secret, 0, item.Quantity)
			if len(cachedReserved) > 0 {
				take := item.Quantity
				if len(cachedReserved) < take {
					take = len(cachedReserved)
				}
				selected = append(selected, cachedReserved[:take]...)
				reservedByKey[key] = cachedReserved[take:]
			}

			if len(selected) < item.Quantity {
				need := item.Quantity - len(selected)
				availableRows, err := secretRepo.ListAvailableByProduct(item.ProductID, item.SKUID, need)
				if err != nil {
					return err
				}
				selected = append(selected, availableRows...)
			}
			if len(selected) < item.Quantity {
				return ErrCardSecretInsufficient
			}
			secrets = append(secrets, selected...)
		}

		ids := make([]uint, 0, len(secrets))
		secretLines := make([]string, 0, len(secrets))
		for _, secret := range secrets {
			ids = append(ids, secret.ID)
			secretLines = append(secretLines, secret.Secret)
		}
		// Agiso 的卡不经过本地卡密表，单独并进交付内容
		secretLines = append(orderedLines, secretLines...)

		affected, err := secretRepo.MarkUsed(ids, orderID, now)
		if err != nil {
			return err
		}
		if int(affected) != len(ids) {
			return ErrCardSecretInsufficient
		}

		payload := strings.Join(secretLines, "\n")
		fulfillment = &fulfillmentdomain.Fulfillment{
			OrderID:     orderID,
			Type:        constants.FulfillmentTypeAuto,
			Status:      constants.FulfillmentStatusDelivered,
			Payload:     payload,
			DeliveredAt: &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Fulfillments().Create(fulfillment); err != nil {
			return ErrFulfillmentCreateFailed
		}
		if err := tx.Orders().UpdateFields(orderID, map[string]interface{}{
			"status":     constants.OrderStatusCompleted,
			"updated_at": now,
		}); err != nil {
			return ErrOrderUpdateFailed
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrFulfillmentExists):
			return nil, ErrFulfillmentExists
		case errors.Is(err, ErrCardSecretInsufficient):
			return nil, ErrCardSecretInsufficient
		case errors.Is(err, ErrOrderUpdateFailed):
			return nil, ErrOrderUpdateFailed
		case errors.Is(err, ErrFulfillmentNotAuto):
			return nil, ErrFulfillmentNotAuto
		default:
			return nil, ErrFulfillmentCreateFailed
		}
	}
	s.afterDelivered(order, orderID, now)
	return fulfillment, nil
}

// afterDelivered 执行交付完成后的公共收尾：父订单状态同步、状态邮件、
// Telegram 通知与下游回调。auto 与 content 两种交付共用。
func (s *Service) afterDelivered(order *orderdomain.Order, orderID uint, now time.Time) {
	if order == nil {
		return
	}
	if s.orderQueue != nil {
		if order.ParentID != nil {
			status, syncErr := orderapp.SyncParentStatus(s.orderStore, *order.ParentID, now)
			if syncErr != nil {
				logger.Warnw("fulfillment_sync_parent_status_failed",
					"order_id", order.ID,
					"parent_order_id", *order.ParentID,
					"target_status", constants.OrderStatusCompleted,
					"error", syncErr,
				)
			} else {
				if status == "" {
					status = constants.OrderStatusCompleted
				}
				if status != constants.OrderStatusCanceled {
					if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(s.orderStore, s.orderQueue, s.settingService, s.defaultEmailConfig, *order.ParentID, status); err != nil {
						logger.Warnw("fulfillment_enqueue_status_email_failed",
							"order_id", order.ID,
							"target_order_id", *order.ParentID,
							"status", status,
							"error", err,
						)
					}
				}
			}
		} else {
			if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(s.orderStore, s.orderQueue, s.settingService, s.defaultEmailConfig, orderID, constants.OrderStatusCompleted); err != nil {
				logger.Warnw("fulfillment_enqueue_status_email_failed",
					"order_id", order.ID,
					"target_order_id", orderID,
					"status", constants.OrderStatusCompleted,
					"error", err,
				)
			}
		}
	}
	// Telegram 通知：交付完成后推送给用户
	notifyOrderID := orderID
	if order.ParentID != nil {
		notifyOrderID = *order.ParentID
	}
	go s.NotifyBotOrderFulfilled(order.UserID, notifyOrderID)
	// B 侧：交付完成后触发下游回调
	if s.downstreamCallbackSvc != nil {
		s.downstreamCallbackSvc.EnqueueCallback(orderID)
	}
}

// CreateContent 处理纯内容交付：订单支付成功后立即完成，交付物是商品自带的使用教程，
// 不消耗卡密库存也不占用人工库存。同一订单重复调用返回 ErrFulfillmentExists。
func (s *Service) CreateContent(orderID uint) (*fulfillmentdomain.Fulfillment, error) {
	if orderID == 0 {
		return nil, ErrFulfillmentInvalid
	}

	order, err := s.orderStore.GetByID(orderID)
	if err != nil {
		return nil, ErrOrderFetchFailed
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.ParentID == nil && len(order.Children) > 0 {
		return nil, ErrFulfillmentInvalid
	}
	if order.Status != constants.OrderStatusPaid {
		return nil, ErrOrderStatusInvalid
	}
	if len(order.Items) == 0 {
		return nil, ErrFulfillmentInvalid
	}
	for _, item := range order.Items {
		if strings.TrimSpace(item.FulfillmentType) != constants.FulfillmentTypeContent {
			return nil, ErrFulfillmentNotContent
		}
	}

	now := time.Now()
	var fulfillment *fulfillmentdomain.Fulfillment
	err = s.orderStore.WithinTransaction(func(tx ordercontract.Transaction) error {
		if _, found, err := tx.Fulfillments().FindByOrderIDForUpdate(orderID); err != nil {
			return err
		} else if found {
			return ErrFulfillmentExists
		}

		fulfillment = &fulfillmentdomain.Fulfillment{
			OrderID:     orderID,
			Type:        constants.FulfillmentTypeContent,
			Status:      constants.FulfillmentStatusDelivered,
			Payload:     "",
			DeliveredAt: &now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		// 交付内容本身由订单详情读取商品的多语言使用教程渲染，这里只标记交付形态，
		// 便于前端分支与后台统计区分三类交付。
		fulfillment.LogisticsJSON = jsonmap.JSON{"delivery_kind": constants.FulfillmentTypeContent}
		if err := tx.Fulfillments().Create(fulfillment); err != nil {
			return ErrFulfillmentCreateFailed
		}
		if err := tx.Orders().UpdateFields(orderID, map[string]interface{}{
			"status":     constants.OrderStatusCompleted,
			"updated_at": now,
		}); err != nil {
			return ErrOrderUpdateFailed
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrFulfillmentExists):
			return nil, ErrFulfillmentExists
		case errors.Is(err, ErrOrderUpdateFailed):
			return nil, ErrOrderUpdateFailed
		default:
			return nil, ErrFulfillmentCreateFailed
		}
	}

	s.afterDelivered(order, orderID, now)
	return fulfillment, nil
}

// NotifyBotOrderFulfilled 查找用户 Telegram 绑定并入队通知任务。
func (s *Service) NotifyBotOrderFulfilled(userID, orderID uint) {
	if s.botNotifier == nil || userID == 0 || s.userOAuthIdentityRepo == nil {
		return
	}

	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(userID, "telegram")
	if err != nil {
		logger.Warnw("fulfillment_notify_bot_fetch_identity_failed",
			"order_id", orderID, "user_id", userID, "error", err)
		return
	}
	if identity == nil || strings.TrimSpace(identity.ProviderUserID) == "" {
		return
	}

	if err := s.botNotifier.EnqueueOrderFulfilled(strings.TrimSpace(identity.ProviderUserID), orderID); err != nil {
		logger.Warnw("fulfillment_notify_bot_enqueue_failed",
			"order_id", orderID, "user_id", userID, "error", err)
	}
}

func normalizeManualDeliveryData(raw jsonmap.JSON) jsonmap.JSON {
	if len(raw) == 0 {
		return jsonmap.JSON{}
	}
	normalized := jsonmap.JSON{}
	note := strings.TrimSpace(toStringValue(raw["note"]))
	if note != "" {
		normalized["note"] = note
	}
	if entriesRaw, ok := raw["entries"]; ok {
		entries := normalizeManualDeliveryEntries(entriesRaw)
		if len(entries) > 0 {
			normalized["entries"] = entries
		}
	}

	keys := make([]string, 0, len(raw))
	for key := range raw {
		if key == "note" || key == "entries" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := normalizeManualDeliveryPrimitive(raw[key])
		if value == nil {
			continue
		}
		normalized[key] = value
	}

	return normalized
}

func normalizeManualDeliveryEntries(raw interface{}) []jsonmap.JSON {
	appendEntry := func(entries []jsonmap.JSON, row map[string]interface{}) []jsonmap.JSON {
		key := strings.TrimSpace(toStringValue(row["key"]))
		value := strings.TrimSpace(toStringValue(row["value"]))
		if key == "" && value == "" {
			return entries
		}
		entry := jsonmap.JSON{}
		if key != "" {
			entry["key"] = key
		}
		if value != "" {
			entry["value"] = value
		}
		return append(entries, entry)
	}

	entries := make([]jsonmap.JSON, 0)
	switch value := raw.(type) {
	case []jsonmap.JSON:
		for _, item := range value {
			entries = appendEntry(entries, item)
		}
		return entries
	case []map[string]interface{}:
		for _, item := range value {
			entries = appendEntry(entries, item)
		}
		return entries
	case []interface{}:
		for _, item := range value {
			row, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			entries = appendEntry(entries, row)
		}
		return entries
	default:
		return nil
	}
}

func normalizeManualDeliveryPrimitive(raw interface{}) interface{} {
	switch value := raw.(type) {
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return nil
		}
		return text
	case bool:
		return value
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return value
	default:
		return nil
	}
}

func buildManualDeliveryPayload(data jsonmap.JSON) string {
	if len(data) == 0 {
		return ""
	}
	lines := make([]string, 0, 8)
	if note, ok := data["note"].(string); ok && strings.TrimSpace(note) != "" {
		lines = append(lines, note)
	}
	if entries, ok := data["entries"].([]jsonmap.JSON); ok {
		for _, item := range entries {
			key := strings.TrimSpace(toStringValue(item["key"]))
			value := strings.TrimSpace(toStringValue(item["value"]))
			if key == "" && value == "" {
				continue
			}
			if key == "" {
				lines = append(lines, value)
				continue
			}
			if value == "" {
				lines = append(lines, key)
				continue
			}
			lines = append(lines, fmt.Sprintf("%s: %s", key, value))
		}
	}

	extraKeys := make([]string, 0, len(data))
	for key := range data {
		if key == "note" || key == "entries" {
			continue
		}
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		valueText := strings.TrimSpace(toStringValue(data[key]))
		if valueText == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", key, valueText))
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func toStringValue(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
		return fmt.Sprintf("%v", value)
	default:
		return ""
	}
}

// pickAgisoCards 在事务之外把订单里所有「卡密来自 Agiso」的商品提好卡。
// 返回值按订单项 key 分组；没有此类商品时返回空 map，调用方无需特判。
func (s *Service) pickAgisoCards(order *orderdomain.Order) (map[string][]string, error) {
	out := map[string][]string{}
	if s.agisoPicker == nil {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, item := range order.Items {
		if item.ProductID == 0 || item.Quantity <= 0 {
			continue
		}
		isAgiso, err := s.agisoPicker.IsAgisoItem(item.ProductID, item.SKUID)
		if err != nil {
			return nil, err
		}
		if !isAgiso {
			continue
		}
		// 幂等键用订单号加商品与 SKU：一个订单里可能有多个 Agiso 商品，
		// 只用订单号会让第二个商品命中第一个的幂等结果，拿到错误的卡。
		pickNo := fmt.Sprintf("%s-%d-%d", order.OrderNo, item.ProductID, item.SKUID)
		lines, err := s.agisoPicker.PickForOrder(ctx, item.ProductID, item.SKUID, item.Quantity, pickNo)
		if err != nil {
			return nil, err
		}
		if len(lines) != item.Quantity {
			// 数量对不上就是部分失败，宁可整单转人工，也不要发出去一半
			return nil, ErrCardSecretInsufficient
		}
		out[orderdomain.ItemKey(item.ProductID, item.SKUID)] = lines
	}
	return out, nil
}
