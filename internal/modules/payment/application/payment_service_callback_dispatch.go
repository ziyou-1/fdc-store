package application

import (
	"errors"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	"github.com/dujiao-next/internal/constants"
	notificationformat "github.com/dujiao-next/internal/modules/notification/application/format"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	"go.uber.org/zap"
)

func (s *PaymentService) enqueueOrderPaidAsync(order *orderdomain.Order, payment *paymentdomain.Payment, log *zap.SugaredLogger) {
	if order == nil {
		return
	}
	// 积分兑换单不产生分销佣金：佣金基数取自商品条目金额（calculateCommissionBaseAmount），
	// 而积分单的条目金额是名义现金价，不门控就是真金白银的佣金流出。
	if s.affiliateSvc != nil && order.PointsPaidAmount.Decimal.IsZero() {
		if err := s.affiliateSvc.HandleOrderPaid(order.ID); err != nil {
			log.Warnw("affiliate_handle_order_paid_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
	if s.queue != nil && s.queue.Enabled() && !isOrderFullyAutoFulfill(order) {
		// 完全自动交付的订单会紧接着发送含卡密内容的"已完成"邮件，跳过"已支付"邮件避免重复打扰
		if _, err := orderapp.EnqueueStatusEmailTaskIfEligible(s.orderRepo, s.queue, s.settingService, s.defaultEmailConfig, order.ID, constants.OrderStatusPaid); err != nil {
			log.Warnw("payment_enqueue_status_email_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"status", constants.OrderStatusPaid,
				"error", err,
			)
		}
	}
	s.enqueueOrderPaidNotificationAsync(order, payment, log)
	s.enqueueOrderPaidBotNotifyAsync(order, log)

	// 订单支付成功后触发会员等级升级检查
	// 积分兑换单不累加会员成长值，否则可以用积分免费刷会员等级。
	if s.memberLevelSvc != nil && order.UserID > 0 && order.PointsPaidAmount.Decimal.IsZero() {
		if err := s.memberLevelSvc.OnOrderPaid(order.UserID, order.TotalAmount.Decimal); err != nil {
			log.Warnw("member_level_order_paid_failed",
				"order_id", order.ID,
				"user_id", order.UserID,
				"amount", order.TotalAmount.Decimal.String(),
				"error", err,
			)
		}
	}

	// 推荐首单奖励：不判断"是不是第一单"，每次支付成功都尝试入账，
	// 由 points 侧 referral_first_order:<被邀请人ID> 这个稳定 reference 的唯一索引
	// 保证只有第一次真正落账。积分兑换单排除在外，防止用积分刷积分。
	// 门控逐条对应红队指出的失败模式：
	// 积分兑换单排除（防止用积分刷积分）、分销订单排除（分销链路刻意隔离在积分经济外）、
	// 已有现金佣金归因的订单排除（同一单不同时发现金佣金与积分）。
	if s.pointsSvc != nil && s.userRepo != nil && order.UserID > 0 &&
		order.PointsPaidAmount.Decimal.IsZero() &&
		order.ResellerID == nil &&
		order.AffiliateProfileID == nil {
		buyer, buyerErr := s.userRepo.GetByID(order.UserID)
		inviterActive := false
		if buyerErr == nil && buyer != nil && buyer.InviterUserID != nil {
			// 推荐人被封禁即止付：用户状态与推广档案状态是两张表，
			// 后台封号不会联动改档案，只查档案会漏。
			// 这里内联而不是抽 helper —— 本文件的函数名清单被架构守卫钉死。
			if inviter, inviterErr := s.userRepo.GetByID(*buyer.InviterUserID); inviterErr == nil &&
				inviter != nil && inviter.Status == constants.UserStatusActive {
				inviterActive = true
			}
		}
		if inviterActive {
			if err := s.pointsSvc.AwardReferralFirstOrder(*buyer.InviterUserID, buyer.ID, order.TotalAmount.Decimal); err != nil {
				log.Warnw("points_referral_first_order_failed",
					"order_id", order.ID,
					"user_id", order.UserID,
					"inviter_user_id", *buyer.InviterUserID,
					"error", err,
				)
			}
		}
	}

	// 纯内容交付同步完成：不入队、不依赖 Redis，支付成功即交付教程内容。
	s.fulfillContentOrders(order, log)
	// skip_payment 时 API 进程通常不跑 worker，自动发卡也必须同步完成。
	if s.skipPayment {
		s.fulfillAutoOrders(order, log)
	}

	if s.queue == nil || !s.queue.Enabled() {
		return
	}
	if len(order.Children) > 0 {
		for _, child := range order.Children {
			if child.Status == constants.OrderStatusFulfilling && hasManualFulfillmentItems(&child) {
				s.enqueueManualFulfillmentPendingAsync(&child, order, log)
			}
			if !s.skipPayment && shouldAutoFulfill(&child) {
				if err := s.queue.EnqueueOrderAutoFulfill(child.ID); err != nil {
					log.Warnw("payment_enqueue_auto_fulfill_failed",
						"order_id", order.ID,
						"child_order_id", child.ID,
						"order_no", order.OrderNo,
						"error", err,
					)
				}
			}
		}
		// 上游采购：为包含上游交付类型的订单创建采购单
		s.enqueueProcurementAsync(order, log)
		// B 侧：订单支付成功后检查是否需要回调下游
		s.enqueueDownstreamCallbackAsync(order, log)
		return
	}
	if order.Status == constants.OrderStatusFulfilling && hasManualFulfillmentItems(order) {
		s.enqueueManualFulfillmentPendingAsync(order, nil, log)
	}
	if !s.skipPayment && shouldAutoFulfill(order) {
		if err := s.queue.EnqueueOrderAutoFulfill(order.ID); err != nil {
			log.Warnw("payment_enqueue_auto_fulfill_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
	// 上游采购：为包含上游交付类型的订单创建采购单
	s.enqueueProcurementAsync(order, log)
	// B 侧：订单支付成功后检查是否需要回调下游
	s.enqueueDownstreamCallbackAsync(order, log)
}

// fulfillAutoOrders 对自动发卡订单（含子订单）同步执行交付。
func (s *PaymentService) fulfillAutoOrders(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.autoFulfillSvc == nil || order == nil {
		return
	}
	targets := make([]*orderdomain.Order, 0, len(order.Children)+1)
	if len(order.Children) > 0 {
		for i := range order.Children {
			targets = append(targets, &order.Children[i])
		}
	} else {
		targets = append(targets, order)
	}
	for _, target := range targets {
		if !shouldAutoFulfill(target) {
			continue
		}
		if err := s.autoFulfillSvc.CreateAuto(target.ID); err != nil {
			log.Warnw("payment_auto_fulfill_failed",
				"order_id", target.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
}

// fulfillContentOrders 对纯内容交付的订单（含子订单）同步执行交付。
// 重复投递的支付回调会命中交付记录唯一约束，由交付用例返回"已存在"并被忽略。
func (s *PaymentService) fulfillContentOrders(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.contentFulfillSvc == nil || order == nil {
		return
	}
	targets := make([]*orderdomain.Order, 0, len(order.Children)+1)
	if len(order.Children) > 0 {
		for i := range order.Children {
			targets = append(targets, &order.Children[i])
		}
	} else {
		targets = append(targets, order)
	}
	for _, target := range targets {
		if !shouldContentFulfill(target) {
			continue
		}
		if err := s.contentFulfillSvc.CreateContent(target.ID); err != nil {
			log.Warnw("payment_content_fulfill_failed",
				"order_id", target.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
}

// enqueueProcurementAsync 如果订单包含上游交付类型商品，创建采购单
func (s *PaymentService) enqueueProcurementAsync(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.procurementSvc == nil || order == nil {
		return
	}
	if err := s.procurementSvc.CreateForOrder(order.ID); err != nil {
		if !errors.Is(err, procurementcontract.ErrExists) {
			log.Warnw("payment_enqueue_procurement_failed",
				"order_id", order.ID,
				"order_no", order.OrderNo,
				"error", err,
			)
		}
	}
}

// enqueueDownstreamCallbackAsync B 侧：通知下游 A 站点订单已支付
func (s *PaymentService) enqueueDownstreamCallbackAsync(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.downstreamCallbackSvc == nil || order == nil {
		return
	}
	s.downstreamCallbackSvc.EnqueueCallback(order.ID)
}

func (s *PaymentService) enqueueOrderPaidNotificationAsync(order *orderdomain.Order, payment *paymentdomain.Payment, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || order == nil {
		return
	}
	payload := s.buildOrderNotificationPayload(order, payment)
	if err := s.notificationSvc.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventOrderPaidSuccess,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     order.ID,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_order_paid_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueWalletRechargeSuccessAsync(recharge *walletdomain.RechargeOrder, payment *paymentdomain.Payment, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || recharge == nil {
		return
	}
	payload := s.buildWalletRechargeNotificationPayload(recharge, payment)
	if err := s.notificationSvc.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventWalletRechargeSuccess,
		BizType:   constants.NotificationBizTypeWalletRecharge,
		BizID:     recharge.ID,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_wallet_recharge_failed",
			"recharge_id", recharge.ID,
			"recharge_no", recharge.RechargeNo,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueOrderPaidBotNotifyAsync(order *orderdomain.Order, log *zap.SugaredLogger) {
	if s.queue == nil || !s.queue.Enabled() || order == nil || order.UserID == 0 || s.userOAuthIdentityRepo == nil {
		return
	}

	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(order.UserID, constants.UserOAuthProviderTelegram)
	if err != nil {
		log.Warnw("order_paid_notify_bot_fetch_identity_failed",
			"order_id", order.ID,
			"user_id", order.UserID,
			"error", err,
		)
		return
	}
	if identity == nil || strings.TrimSpace(identity.ProviderUserID) == "" {
		return
	}

	if err := s.queue.EnqueueBotNotification(paymentcontract.BotNotification{
		EventType:      paymentcontract.BotNotificationOrderPaid,
		OrderID:        order.ID,
		TelegramUserID: strings.TrimSpace(identity.ProviderUserID),
	}); err != nil {
		log.Warnw("order_paid_notify_bot_enqueue_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"user_id", order.UserID,
			"error", err,
		)
	}
}

func (s *PaymentService) enqueueWalletRechargeBotNotifyAsync(recharge *walletdomain.RechargeOrder, log *zap.SugaredLogger) {
	if s.queue == nil || !s.queue.Enabled() || recharge == nil || recharge.UserID == 0 || s.userOAuthIdentityRepo == nil {
		return
	}

	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(recharge.UserID, constants.UserOAuthProviderTelegram)
	if err != nil {
		log.Warnw("wallet_recharge_notify_bot_fetch_identity_failed",
			"recharge_id", recharge.ID,
			"user_id", recharge.UserID,
			"error", err,
		)
		return
	}
	if identity == nil || strings.TrimSpace(identity.ProviderUserID) == "" {
		return
	}

	if err := s.queue.EnqueueBotNotification(paymentcontract.BotNotification{
		EventType:      paymentcontract.BotNotificationWalletRechargeSucceeded,
		TelegramUserID: strings.TrimSpace(identity.ProviderUserID),
		RechargeNo:     strings.TrimSpace(recharge.RechargeNo),
		Amount:         recharge.Amount.String(),
		Currency:       strings.ToUpper(strings.TrimSpace(recharge.Currency)),
	}); err != nil {
		log.Warnw("wallet_recharge_notify_bot_enqueue_failed",
			"recharge_id", recharge.ID,
			"recharge_no", recharge.RechargeNo,
			"user_id", recharge.UserID,
			"error", err,
		)
	}
}

// hasManualFulfillmentItems 判断订单是否包含需要人工交付的商品项。
// upstream 类型由采购流程自动交付，不触发待人工交付提醒。
func hasManualFulfillmentItems(order *orderdomain.Order) bool {
	if order == nil {
		return false
	}
	for _, item := range order.Items {
		if notificationformat.NormalizeFulfillmentType(item.FulfillmentType) == constants.FulfillmentTypeManual {
			return true
		}
	}
	return false
}

func (s *PaymentService) enqueueManualFulfillmentPendingAsync(order *orderdomain.Order, parent *orderdomain.Order, log *zap.SugaredLogger) {
	if s.notificationSvc == nil || order == nil {
		return
	}
	payload := s.buildManualFulfillmentNotificationPayload(order, parent)
	if err := s.notificationSvc.Enqueue(notificationcontract.EnqueueInput{
		EventType: constants.NotificationEventManualFulfillmentPending,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     order.ID,
		Data:      payload,
	}); err != nil {
		log.Warnw("notification_enqueue_manual_pending_failed",
			"order_id", order.ID,
			"order_no", order.OrderNo,
			"error", err,
		)
	}
}
