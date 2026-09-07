package application

import (
	"context"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	pointscontract "github.com/dujiao-next/internal/modules/points/contract"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// CreatePaymentInput 创建支付请求
type CreatePaymentInput struct {
	OrderID    uint
	ChannelID  uint
	UseBalance bool
	UsePoints  bool
	// UseFreeClaim 非空表示这是一次每日免费领取的结算。
	UseFreeClaim     *orderapp.FreeClaimIntent
	ClientIP         string
	Context          context.Context
	ReturnBizType    string
	ReturnBusinessNo string
	ReturnGuest      bool
	// RequestScheme 当前请求的 scheme（http/https），用于分销站动态生成 return_url；空值默认 https。
	RequestScheme string
}

// CreatePaymentResult 创建支付结果
type CreatePaymentResult struct {
	Payment          *paymentdomain.Payment
	Channel          *paymentdomain.PaymentChannel
	OrderPaid        bool
	WalletPaidAmount money.Amount
	OnlinePayAmount  money.Amount
	PointsPaidAmount money.Amount
	PointsUsed       int64
}

func skipPaymentApplies(skipConfigured, usePoints, useFreeClaim bool) bool {
	return skipConfigured && !usePoints && !useFreeClaim
}

func hasProviderResult(payment *paymentdomain.Payment) bool {
	if payment == nil {
		return false
	}
	return strings.TrimSpace(payment.PayURL) != "" || strings.TrimSpace(payment.QRCode) != ""
}

// CreatePayment 创建支付单
func (s *PaymentService) CreatePayment(input CreatePaymentInput) (*CreatePaymentResult, error) {
	if input.OrderID == 0 {
		return nil, ErrPaymentInvalid
	}

	log := paymentLogger(
		"order_id", input.OrderID,
		"channel_id", input.ChannelID,
	)

	var payment *paymentdomain.Payment
	var order *orderdomain.Order
	var channel *paymentdomain.PaymentChannel
	feeRate := decimal.Zero
	reusedPending := false
	orderPaidByWallet := false
	pointsUsed := int64(0)
	now := time.Now()
	feeConfig := settingsapp.DefaultPaymentFeeConfig()
	if s.settingService != nil {
		feeConfig = s.settingService.GetPaymentFeeConfig()
	}

	// skip_payment 是现金成交捷径，不能盖掉积分兑换或免费领取：
	// 兑换必须真扣积分；领取必须写入当日记录，否则"每天一次"会失效。
	skipPayment := skipPaymentApplies(s.skipPayment, input.UsePoints, input.UseFreeClaim != nil)
	if skipPayment {
		input.ChannelID = 0
		input.UseBalance = false
	}

	// 在事务外查询设置，避免 SQLite 单连接池下自锁
	walletOnly := !skipPayment && s.settingService != nil && s.settingService.GetWalletOnlyPayment()
	if walletOnly {
		input.UseBalance = true
		if input.ChannelID != 0 {
			return nil, walletcontract.ErrOnlyPaymentRequired
		}
	}

	// 积分兑换：配置同样在事务外读取。积分与余额、在线渠道三者互斥，
	// 且必须整单全额抵扣——这条约束换来两个结果：抵扣后应付在线额恒为 0，
	// 于是一定走下方同事务的合成支付分支；以及 ChannelID 必为 0，
	// 从而绕不开也进不去上面那条"复用旧 pending 支付"的短路。
	// 免费领取与其它 tender 互斥：整单由系统结算，不接受任何用户付款方式。
	if input.UseFreeClaim != nil &&
		(input.UseBalance || input.UsePoints || input.ChannelID != 0 || walletOnly) {
		return nil, orderapp.ErrPointsTenderExclusive
	}

	var pointsPolicy orderapp.PointsExchangePolicy
	if input.UsePoints {
		if s.pointsSvc == nil {
			return nil, pointscontract.ErrExchangeDisabled
		}
		pointsConfig, cfgErr := s.pointsSvc.GetConfig()
		if cfgErr != nil {
			return nil, cfgErr
		}
		if !pointsConfig.ExchangeEnabled {
			return nil, pointscontract.ErrExchangeDisabled
		}
		if input.UseBalance || walletOnly || input.ChannelID != 0 {
			return nil, orderapp.ErrPointsTenderExclusive
		}
		pointsPolicy = orderapp.PointsExchangePolicy{
			DailyLimit:    pointsConfig.ExchangeDailyLimit,
			SKUDailyLimit: pointsConfig.ExchangeSKUDailyLimit,
		}
	}

	err := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		preloaded, err := tx.Orders().GetByIDForUpdateWithChildren(input.OrderID)
		if err != nil {
			return orderapp.ErrOrderFetchFailed
		}
		if preloaded == nil {
			return orderapp.ErrOrderNotFound
		}
		lockedOrder := *preloaded
		if lockedOrder.ParentID != nil {
			return ErrPaymentInvalid
		}
		if lockedOrder.Status != constants.OrderStatusPendingPayment {
			return orderapp.ErrOrderStatusInvalid
		}
		if lockedOrder.ExpiresAt != nil && !lockedOrder.ExpiresAt.After(time.Now()) {
			return orderapp.ErrOrderStatusInvalid
		}

		paymentRepo := tx.Payments()
		channelRepo := tx.PaymentChannels()
		if input.ChannelID != 0 {
			if channel == nil {
				// 事务内必须使用 tx 绑定仓储，避免在单连接池下发生自锁等待。
				resolvedChannel, err := channelRepo.GetByID(input.ChannelID)
				if err != nil {
					return err
				}
				if resolvedChannel == nil {
					return ErrPaymentChannelNotFound
				}
				if !resolvedChannel.IsActive {
					return ErrPaymentChannelInactive
				}
				resolvedFeeRate := resolvedChannel.FeeRate.Decimal.Round(2)
				if resolvedFeeRate.LessThan(decimal.Zero) || resolvedFeeRate.GreaterThan(decimal.NewFromInt(100)) {
					return ErrPaymentChannelConfigInvalid
				}
				channel = resolvedChannel
				feeRate = resolvedFeeRate
			}
			if err := validateOrderChannelEligibility(*channel, &lockedOrder); err != nil {
				return err
			}

			// 校验商品是否允许该支付渠道（传入 tx 避免 SQLite 自锁）
			allItems := lockedOrder.Items
			for _, child := range lockedOrder.Children {
				allItems = append(allItems, child.Items...)
			}
			if err := s.validateProductPaymentChannel(allItems, channel.ID, tx.Products()); err != nil {
				return err
			}

			existing, err := paymentRepo.GetLatestPendingByOrderChannel(lockedOrder.ID, channel.ID, time.Now())
			if err != nil {
				return ErrPaymentCreateFailed
			}
			if existing != nil && hasProviderResult(existing) {
				legacyFeePayment := existing.FeeAmount.Decimal.IsPositive() &&
					(existing.FeePolicy == "" || existing.FeePolicy == constants.PaymentFeePolicyLegacyCustomerSurcharge)
				if !legacyFeePayment || feeConfig.ReuseLegacyOrderFeePayment {
					reusedPending = true
					payment = existing
					order = &lockedOrder
					return nil
				}
			}
		}

		if s.walletSvc != nil {
			if input.UseBalance {
				if _, err := orderapp.ApplyWalletBalance(s.walletSvc, tx, &lockedOrder, true); err != nil {
					return err
				}
			} else if lockedOrder.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
				if _, err := orderapp.ReleaseWalletBalance(s.walletSvc, tx, &lockedOrder, constants.WalletTxnTypeOrderRefund, "用户改为在线支付，退回余额"); err != nil {
					return err
				}
			}
		}

		if input.UsePoints {
			used, pointsErr := orderapp.ApplyOrderPoints(s.pointsSvc, tx, &lockedOrder, pointsPolicy)
			if pointsErr != nil {
				return pointsErr
			}
			pointsUsed = used
		}

		if input.UseFreeClaim != nil {
			if err := orderapp.ApplyFreeClaim(tx, &lockedOrder, *input.UseFreeClaim); err != nil {
				return err
			}
		}

		onlineAmount := orderapp.ResolveOnlinePayableAmount(&lockedOrder)
		if skipPayment {
			onlineAmount = decimal.Zero
		}
		if onlineAmount.LessThanOrEqual(decimal.Zero) {
			walletPaidAmount := normalizeOrderAmount(lockedOrder.WalletPaidAmount.Decimal)
			// 绝不复用 wallet 的 provider/channel type：否则后台、对账与报表
			// 会把积分兑换单当成余额支付。Amount 取钱包实付额，积分单为 0，
			// 因此不会给以 payments.amount 为口径的 GMV 凭空加一笔现金。
			syntheticProviderType := constants.PaymentProviderWallet
			syntheticChannelType := constants.PaymentChannelTypeBalance
			syntheticInteraction := constants.PaymentInteractionBalance
			if lockedOrder.PointsPaidAmount.Decimal.GreaterThan(decimal.Zero) {
				syntheticProviderType = constants.PaymentProviderPoints
				syntheticChannelType = constants.PaymentChannelTypePoints
				syntheticInteraction = constants.PaymentInteractionPoints
			}
			if lockedOrder.FreeClaimPaidAmount.Decimal.GreaterThan(decimal.Zero) {
				syntheticProviderType = constants.PaymentProviderFreeClaim
				syntheticChannelType = constants.PaymentChannelTypeFreeClaim
				syntheticInteraction = constants.PaymentInteractionFreeClaim
			}
			if skipPayment {
				syntheticProviderType = constants.PaymentProviderSkipPayment
				syntheticChannelType = constants.PaymentChannelTypeSkipPayment
				syntheticInteraction = constants.PaymentInteractionSkipPayment
			}
			paidAt := time.Now()
			payment = &paymentdomain.Payment{
				OrderID:         lockedOrder.ID,
				ChannelID:       0,
				ProviderType:    syntheticProviderType,
				ChannelType:     syntheticChannelType,
				InteractionMode: syntheticInteraction,
				Amount:          money.FromDecimal(walletPaidAmount),
				FeeRate:         money.FromDecimal(decimal.Zero),
				FixedFee:        money.FromDecimal(decimal.Zero),
				FeeAmount:       money.FromDecimal(decimal.Zero),
				FeePolicy:       constants.PaymentFeePolicyNone,
				Currency:        lockedOrder.Currency,
				Status:          constants.PaymentStatusSuccess,
				CreatedAt:       paidAt,
				UpdatedAt:       paidAt,
				PaidAt:          &paidAt,
			}
			if err := paymentRepo.Create(payment); err != nil {
				return ErrPaymentCreateFailed
			}
			// 余额已覆盖全额，订单遗留的在线链接必须同时作废，避免用户再付一次。
			if _, err := paymentRepo.SupersedePendingByOrderID(lockedOrder.ID, payment.ID, paidAt); err != nil {
				return ErrPaymentCreateFailed
			}
			if err := s.markOrderPaid(tx, &lockedOrder, paidAt); err != nil {
				return err
			}
			orderPaidByWallet = true
			order = &lockedOrder
			return nil
		}
		if channel == nil {
			if walletOnly {
				return walletcontract.ErrOnlyPaymentRequired
			}
			return ErrPaymentInvalid
		}
		if err := validatePaymentCurrencyForChannel(lockedOrder.Currency, channel); err != nil {
			return err
		}
		if err := validatePaymentAmountForChannel(onlineAmount, channel); err != nil {
			return err
		}

		fixedFee := decimal.Zero
		if channel.FixedFee.Decimal.GreaterThan(decimal.Zero) {
			fixedFee = channel.FixedFee.Decimal.Round(2)
		}

		paymentAmount, feeAmount, feePolicy := calculatePaymentAmounts(onlineAmount, feeRate, fixedFee, feeConfig.CustomerFeeEnabled)
		payment = &paymentdomain.Payment{
			OrderID:         lockedOrder.ID,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          money.FromDecimal(paymentAmount),
			FeeRate:         money.FromDecimal(feeRate),
			FixedFee:        money.FromDecimal(fixedFee),
			FeeAmount:       money.FromDecimal(feeAmount),
			FeePolicy:       feePolicy,
			Currency:        lockedOrder.Currency,
			Status:          constants.PaymentStatusInitiated,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if shouldUseCNYPaymentCurrency(channel) {
			payment.Currency = "CNY"
		}

		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
		}
		if err := tx.Orders().UpdateFields(lockedOrder.ID, map[string]interface{}{
			"online_paid_amount": money.FromDecimal(onlineAmount),
			"updated_at":         time.Now(),
		}); err != nil {
			return orderapp.ErrOrderUpdateFailed
		}
		lockedOrder.OnlinePaidAmount = money.FromDecimal(onlineAmount)
		lockedOrder.UpdatedAt = time.Now()
		order = &lockedOrder
		return nil
	})
	if err != nil {
		return nil, err
	}

	if order == nil {
		return nil, orderapp.ErrOrderFetchFailed
	}

	if reusedPending {
		if _, err := s.paymentRepo.SupersedePendingByOrderID(order.ID, payment.ID, time.Now()); err != nil {
			return nil, ErrPaymentCreateFailed
		}
		log.Infow("payment_create_reuse_pending",
			"payment_id", payment.ID,
			"provider_type", payment.ProviderType,
			"channel_type", payment.ChannelType,
		)
		return &CreatePaymentResult{
			Payment:          payment,
			Channel:          channel,
			WalletPaidAmount: order.WalletPaidAmount,
			OnlinePayAmount:  order.OnlinePaidAmount,
			PointsPaidAmount: order.PointsPaidAmount,
			PointsUsed:       pointsUsed,
		}, nil
	}

	if orderPaidByWallet {
		log.Infow("payment_create_wallet_success",
			"payment_id", payment.ID,
			"provider_type", payment.ProviderType,
			"channel_type", payment.ChannelType,
			"interaction_mode", payment.InteractionMode,
			"currency", payment.Currency,
			"amount", payment.Amount.String(),
			"wallet_paid_amount", order.WalletPaidAmount.String(),
			"online_pay_amount", order.OnlinePaidAmount.String(),
		)
		s.enqueueOrderPaidAsync(order, payment, log)
		return &CreatePaymentResult{
			Payment:          nil,
			Channel:          nil,
			OrderPaid:        true,
			WalletPaidAmount: order.WalletPaidAmount,
			OnlinePayAmount:  money.FromDecimal(decimal.Zero),
		}, nil
	}

	if payment == nil {
		return nil, ErrPaymentCreateFailed
	}

	if err := s.applyProviderPayment(input, order, channel, payment); err != nil {
		rollbackErr := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
			paymentRepo := tx.Payments()
			payment.Status = constants.PaymentStatusFailed
			payment.UpdatedAt = time.Now()
			if updateErr := paymentRepo.Update(payment); updateErr != nil {
				return updateErr
			}
			if s.walletSvc == nil {
				return nil
			}
			preloaded, findErr := tx.Orders().GetByIDForUpdate(order.ID)
			if findErr != nil {
				return findErr
			}
			if preloaded == nil {
				return orderapp.ErrOrderNotFound
			}
			lockedOrder := *preloaded
			_, refundErr := orderapp.ReleaseWalletBalance(s.walletSvc, tx, &lockedOrder, constants.WalletTxnTypeOrderRefund, "在线支付创建失败，退回余额")
			return refundErr
		})
		if rollbackErr != nil {
			log.Errorw("payment_create_provider_failed_with_rollback_error",
				"payment_id", payment.ID,
				"order_id", order.ID,
				"provider_type", payment.ProviderType,
				"channel_type", payment.ChannelType,
				"provider_error", err,
				"rollback_error", rollbackErr,
			)
		} else {
			log.Errorw("payment_create_provider_failed",
				"payment_id", payment.ID,
				"provider_type", payment.ProviderType,
				"channel_type", payment.ChannelType,
				"error", err,
			)
		}
		return nil, err
	}
	if _, err := s.paymentRepo.SupersedePendingByOrderID(order.ID, payment.ID, time.Now()); err != nil {
		log.Errorw("payment_create_supersede_previous_failed",
			"payment_id", payment.ID,
			"order_id", order.ID,
			"error", err,
		)
		return nil, ErrPaymentCreateFailed
	}

	log.Infow("payment_create_success",
		"payment_id", payment.ID,
		"provider_type", payment.ProviderType,
		"channel_type", payment.ChannelType,
		"interaction_mode", payment.InteractionMode,
		"currency", payment.Currency,
		"amount", payment.Amount.String(),
		"wallet_paid_amount", order.WalletPaidAmount.String(),
		"online_pay_amount", order.OnlinePaidAmount.String(),
	)

	return &CreatePaymentResult{
		Payment:          payment,
		Channel:          channel,
		WalletPaidAmount: order.WalletPaidAmount,
		OnlinePayAmount:  order.OnlinePaidAmount,
	}, nil
}
