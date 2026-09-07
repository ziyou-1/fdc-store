package application

import (
	"context"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/shared/serial"

	"github.com/shopspring/decimal"
)

// CreateWalletRechargePaymentInput 创建钱包充值支付请求
type CreateWalletRechargePaymentInput struct {
	UserID        uint
	ChannelID     uint
	Amount        money.Amount
	Currency      string
	Remark        string
	ClientIP      string
	Context       context.Context
	RequestScheme string
	// CreditTarget 空值按 wallet 处理，保持既有调用方行为不变。
	CreditTarget           string
	PointsAmount           int64
	PointsPerUnit          int64
	MembershipPeriod       string
	MembershipDurationDays int
	MembershipDiscountRate money.Amount
}

// CreateWalletRechargePaymentResult 创建钱包充值支付结果
type CreateWalletRechargePaymentResult struct {
	Recharge *walletdomain.RechargeOrder
	Payment  *paymentdomain.Payment
}

// CreateWalletRechargePayment 创建钱包充值支付单
func (s *PaymentService) CreateWalletRechargePayment(input CreateWalletRechargePaymentInput) (*CreateWalletRechargePaymentResult, error) {
	// 到账目标归一化：空值按余额处理，保证既有调用方行为不变。
	creditTarget := strings.TrimSpace(input.CreditTarget)
	if creditTarget == "" {
		creditTarget = constants.RechargeCreditTargetWallet
	}
	if creditTarget != constants.RechargeCreditTargetWallet &&
		creditTarget != constants.RechargeCreditTargetPoints &&
		creditTarget != constants.RechargeCreditTargetMembership {
		return nil, ErrPaymentInvalid
	}
	// 积分充值必须在建单时就带上快照：回调到账时绝不重读配置，
	// 否则换算比例中途被改会让用户拿到与下单时不一致的积分。
	if creditTarget == constants.RechargeCreditTargetPoints &&
		(input.PointsAmount <= 0 || input.PointsPerUnit <= 0) {
		return nil, ErrPaymentInvalid
	}
	if creditTarget == constants.RechargeCreditTargetMembership &&
		(strings.TrimSpace(input.MembershipPeriod) == "" || input.MembershipDurationDays <= 0 ||
			input.MembershipDiscountRate.Decimal.LessThanOrEqual(decimal.Zero) ||
			input.MembershipDiscountRate.Decimal.GreaterThanOrEqual(decimal.NewFromInt(100))) {
		return nil, ErrPaymentInvalid
	}
	if input.UserID == 0 || input.ChannelID == 0 {
		return nil, ErrPaymentInvalid
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, walletcontract.ErrInvalidAmount
	}
	if s.walletRepo == nil {
		return nil, ErrPaymentCreateFailed
	}

	channel, err := s.channelRepo.GetByID(input.ChannelID)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, ErrPaymentChannelNotFound
	}
	if !channel.IsActive {
		return nil, ErrPaymentChannelInactive
	}
	if s.userRepo == nil {
		return nil, ErrPaymentInvalid
	}
	user, err := s.userRepo.GetByID(input.UserID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if err := validateWalletChannelEligibility(*channel, user); err != nil {
		return nil, err
	}

	// 校验钱包充值是否允许该支付渠道
	if err := s.validateWalletRechargeChannel(channel.ID); err != nil {
		return nil, err
	}

	feeRate := channel.FeeRate.Decimal.Round(2)
	if feeRate.LessThan(decimal.Zero) || feeRate.GreaterThan(decimal.NewFromInt(100)) {
		return nil, ErrPaymentChannelConfigInvalid
	}
	fixedFee := channel.FixedFee.Decimal.Round(2)
	if fixedFee.LessThan(decimal.Zero) || fixedFee.GreaterThanOrEqual(decimal.NewFromInt(10000)) {
		return nil, ErrPaymentChannelConfigInvalid
	}
	if err := validatePaymentAmountForChannel(amount, channel); err != nil {
		return nil, err
	}

	feeConfig := settingsapp.DefaultPaymentFeeConfig()
	if s.settingService != nil {
		feeConfig = s.settingService.GetPaymentFeeConfig()
	}
	paymentAmount, feeAmount, feePolicy := calculatePaymentAmounts(amount, feeRate, fixedFee, feeConfig.CustomerFeeEnabled)
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}
	if err := validatePaymentCurrencyForChannel(currency, channel); err != nil {
		return nil, err
	}
	if shouldUseCNYPaymentCurrency(channel) {
		currency = "CNY"
	}
	now := time.Now()

	var payment *paymentdomain.Payment
	var recharge *walletdomain.RechargeOrder
	err = s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		rechargeNo := generateWalletRechargeNo()
		paymentRepo := tx.Payments()
		payment = &paymentdomain.Payment{
			OrderID:         0,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          money.FromDecimal(paymentAmount),
			FeeRate:         money.FromDecimal(feeRate),
			FixedFee:        money.FromDecimal(fixedFee),
			FeeAmount:       money.FromDecimal(feeAmount),
			FeePolicy:       feePolicy,
			Currency:        currency,
			Status:          constants.PaymentStatusInitiated,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
		}

		rechargeRepo := tx.Wallets().Wallets()
		remark := strings.TrimSpace(input.Remark)
		if remark == "" {
			if creditTarget == constants.RechargeCreditTargetPoints {
				remark = "积分充值"
			} else if creditTarget == constants.RechargeCreditTargetMembership {
				remark = "会员购买"
			} else {
				remark = "余额充值"
			}
		}
		recharge = &walletdomain.RechargeOrder{
			RechargeNo:      rechargeNo,
			UserID:          input.UserID,
			PaymentID:       payment.ID,
			ChannelID:       channel.ID,
			ProviderType:    channel.ProviderType,
			ChannelType:     channel.ChannelType,
			InteractionMode: channel.InteractionMode,
			Amount:          money.FromDecimal(amount),
			PayableAmount:   money.FromDecimal(paymentAmount),
			FeeRate:         money.FromDecimal(feeRate),
			FeeAmount:       money.FromDecimal(feeAmount),
			Currency:        currency,
			Status:          constants.WalletRechargeStatusPending,
			Remark:          remark,
			CreditTarget:           creditTarget,
			PointsAmount:           input.PointsAmount,
			PointsPerUnit:          input.PointsPerUnit,
			MembershipPeriod:       strings.TrimSpace(input.MembershipPeriod),
			MembershipDurationDays: input.MembershipDurationDays,
			MembershipDiscountRate: input.MembershipDiscountRate,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := rechargeRepo.CreateRechargeOrder(recharge); err != nil {
			return ErrPaymentCreateFailed
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if payment == nil || recharge == nil {
		return nil, ErrPaymentCreateFailed
	}

	// 前端回跳需要区分余额充值与积分充值两条落地页。
	// 这里用局部变量而不是抽小函数：payment_service_recharge.go 的函数名清单
	// 被 payment_service_structure_test.go 精确钉死，新增函数会直接打红 CI。
	returnBizType := "recharge"
	if creditTarget == constants.RechargeCreditTargetPoints {
		returnBizType = "points_recharge"
	} else if creditTarget == constants.RechargeCreditTargetMembership {
		returnBizType = "membership"
	}

	// 复用支付网关下单逻辑，使用充值单号作为业务单号。
	virtualOrder := &orderdomain.Order{
		OrderNo: recharge.RechargeNo,
		UserID:  recharge.UserID,
	}
	if err := s.applyProviderPayment(CreatePaymentInput{
		ChannelID:        input.ChannelID,
		ClientIP:         input.ClientIP,
		Context:          input.Context,
		ReturnBizType:    returnBizType,
		ReturnBusinessNo: recharge.RechargeNo,
		RequestScheme:    input.RequestScheme,
	}, virtualOrder, channel, payment); err != nil {
		_ = s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
			rechargeRepo := tx.Wallets().Wallets()
			paymentRepo := tx.Payments()
			failedAt := time.Now()
			payment.Status = constants.PaymentStatusFailed
			payment.UpdatedAt = failedAt
			if updateErr := paymentRepo.Update(payment); updateErr != nil {
				return updateErr
			}
			lockedRecharge, getErr := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
			if getErr != nil || lockedRecharge == nil {
				return getErr
			}
			lockedRecharge.Status = constants.WalletRechargeStatusFailed
			lockedRecharge.UpdatedAt = failedAt
			return rechargeRepo.UpdateRechargeOrder(lockedRecharge)
		})
		return nil, err
	}
	if s.queue != nil && s.queue.Enabled() {
		delay := time.Duration(s.resolveExpireMinutes()) * time.Minute
		if err := s.queue.EnqueueWalletRechargeExpire(payment.ID, delay); err != nil {
			logger.Errorw("wallet_recharge_enqueue_timeout_expire_failed",
				"payment_id", payment.ID,
				"recharge_no", recharge.RechargeNo,
				"delay_minutes", int(delay/time.Minute),
				"error", err,
			)
			_ = s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
				rechargeRepo := tx.Wallets().Wallets()
				paymentRepo := tx.Payments()
				failedAt := time.Now()
				payment.Status = constants.PaymentStatusFailed
				payment.UpdatedAt = failedAt
				if updateErr := paymentRepo.Update(payment); updateErr != nil {
					return updateErr
				}
				lockedRecharge, getErr := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
				if getErr != nil || lockedRecharge == nil {
					return getErr
				}
				if lockedRecharge.Status == constants.WalletRechargeStatusSuccess {
					return nil
				}
				lockedRecharge.Status = constants.WalletRechargeStatusFailed
				lockedRecharge.UpdatedAt = failedAt
				return rechargeRepo.UpdateRechargeOrder(lockedRecharge)
			})
			return nil, ErrQueueUnavailable
		}
	}

	reloadedRecharge, err := s.walletRepo.GetRechargeOrderByPaymentID(payment.ID)
	if err != nil {
		return nil, ErrPaymentUpdateFailed
	}
	if reloadedRecharge != nil {
		recharge = reloadedRecharge
	}
	return &CreateWalletRechargePaymentResult{
		Recharge: recharge,
		Payment:  payment,
	}, nil
}

func generateWalletRechargeNo() string {
	return serial.Generate("WR")
}

// ExpireWalletRechargePayment 将未支付的钱包充值单标记为过期（幂等）。
func (s *PaymentService) ExpireWalletRechargePayment(paymentID uint) (*paymentdomain.Payment, error) {
	if paymentID == 0 {
		return nil, ErrPaymentInvalid
	}
	if s == nil || s.paymentRepo == nil || s.walletRepo == nil {
		return nil, ErrPaymentUpdateFailed
	}

	var output *paymentdomain.Payment
	err := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		locked, err := tx.Payments().GetByIDForUpdate(paymentID)
		if err != nil {
			return ErrPaymentUpdateFailed
		}
		if locked == nil {
			return ErrPaymentNotFound
		}
		payment := *locked
		if payment.OrderID != 0 {
			output = &payment
			return nil
		}

		rechargeRepo := tx.Wallets().Wallets()
		recharge, err := rechargeRepo.GetRechargeOrderByPaymentIDForUpdate(payment.ID)
		if err != nil {
			return ErrPaymentUpdateFailed
		}
		if recharge == nil {
			return walletcontract.ErrRechargeNotFound
		}
		if !canExpireWalletRechargePayment(&payment, recharge) {
			output = &payment
			return nil
		}

		now := time.Now()
		payment.Status = constants.PaymentStatusExpired
		payment.ExpiredAt = &now
		payment.UpdatedAt = now
		if err := tx.Payments().Update(&payment); err != nil {
			return ErrPaymentUpdateFailed
		}

		recharge.Status = constants.WalletRechargeStatusExpired
		recharge.UpdatedAt = now
		if err := rechargeRepo.UpdateRechargeOrder(recharge); err != nil {
			return ErrPaymentUpdateFailed
		}
		output = &payment
		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}

func canExpireWalletRechargePayment(payment *paymentdomain.Payment, recharge *walletdomain.RechargeOrder) bool {
	if payment == nil || recharge == nil || recharge.Status != constants.WalletRechargeStatusPending {
		return false
	}
	if payment.Status == constants.PaymentStatusSuccess || recharge.Status == constants.WalletRechargeStatusSuccess {
		return false
	}
	if payment.Status == constants.PaymentStatusFailed || recharge.Status == constants.WalletRechargeStatusFailed {
		return false
	}
	if payment.Status == constants.PaymentStatusExpired || recharge.Status == constants.WalletRechargeStatusExpired {
		return false
	}
	return payment.Status == constants.PaymentStatusInitiated || payment.Status == constants.PaymentStatusPending
}
