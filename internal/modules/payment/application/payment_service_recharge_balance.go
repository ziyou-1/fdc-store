package application

import (
	"fmt"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// CreateWalletBalanceRechargeInput 用钱包余额支付的充值请求。
//
// 与 CreateWalletRechargePayment 的区别：不走任何外部支付渠道，余额即时扣款、即时到账，
// 因此没有 ChannelID / ClientIP / RequestScheme / Context 这些网关相关字段。
type CreateWalletBalanceRechargeInput struct {
	UserID        uint
	Amount        money.Amount
	Currency      string
	Remark        string
	CreditTarget  string
	PointsAmount  int64
	PointsPerUnit int64
}

// CreateWalletBalanceRechargeResult 余额支付充值结果。
type CreateWalletBalanceRechargeResult struct {
	Recharge *walletdomain.RechargeOrder
	Payment  *paymentdomain.Payment
}

// CreateWalletBalanceRecharge 用钱包余额同步完成一笔充值到账（当前仅支持积分）。
//
// 一个事务内闭环：合成一张"余额支付、即时成功"的支付单 → 建成功充值单 →
// 全额从余额扣款 → 按到账目标入账。扣款与到账复用 wallet / points 既有逻辑，
// 余额不足则整笔回滚，绝不会出现"扣了钱没到账"或"到账没扣钱"的中间态。
func (s *PaymentService) CreateWalletBalanceRecharge(input CreateWalletBalanceRechargeInput) (*CreateWalletBalanceRechargeResult, error) {
	creditTarget := strings.TrimSpace(input.CreditTarget)
	// 目前仅积分充值支持余额支付：余额充值余额是自我循环、无意义；会员购买另有入口。
	if creditTarget != constants.RechargeCreditTargetPoints {
		return nil, ErrPaymentInvalid
	}
	// 到账快照必须齐全，口径与在线充值一致：绝不在到账时重读配置。
	if input.PointsAmount <= 0 || input.PointsPerUnit <= 0 {
		return nil, ErrPaymentInvalid
	}
	if input.UserID == 0 {
		return nil, ErrPaymentInvalid
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, walletcontract.ErrInvalidAmount
	}
	if s.walletSvc == nil || s.pointsSvc == nil || s.paymentRepo == nil {
		return nil, ErrPaymentCreateFailed
	}

	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "CNY"
	}
	remark := strings.TrimSpace(input.Remark)
	if remark == "" {
		remark = "积分充值"
	}

	now := time.Now()
	var payment *paymentdomain.Payment
	var recharge *walletdomain.RechargeOrder
	err := s.paymentRepo.WithinTransaction(func(tx paymentcontract.Transaction) error {
		paymentRepo := tx.Payments()
		walletTx := tx.Wallets()
		rechargeRepo := walletTx.Wallets()

		// 合成"余额支付、即时成功"的支付单，口径与订单侧全额余额支付一致：
		// provider=wallet / channel=balance / 无手续费 / ChannelID=0。
		payment = &paymentdomain.Payment{
			OrderID:         0,
			ChannelID:       0,
			ProviderType:    constants.PaymentProviderWallet,
			ChannelType:     constants.PaymentChannelTypeBalance,
			InteractionMode: constants.PaymentInteractionBalance,
			Amount:          money.FromDecimal(amount),
			FeeRate:         money.FromDecimal(decimal.Zero),
			FixedFee:        money.FromDecimal(decimal.Zero),
			FeeAmount:       money.FromDecimal(decimal.Zero),
			FeePolicy:       constants.PaymentFeePolicyNone,
			Currency:        currency,
			Status:          constants.PaymentStatusSuccess,
			CreatedAt:       now,
			UpdatedAt:       now,
			PaidAt:          &now,
		}
		if err := paymentRepo.Create(payment); err != nil {
			return ErrPaymentCreateFailed
		}

		recharge = &walletdomain.RechargeOrder{
			RechargeNo:      generateWalletRechargeNo(),
			UserID:          input.UserID,
			PaymentID:       payment.ID,
			ChannelID:       0,
			ProviderType:    constants.PaymentProviderWallet,
			ChannelType:     constants.PaymentChannelTypeBalance,
			InteractionMode: constants.PaymentInteractionBalance,
			Amount:          money.FromDecimal(amount),
			PayableAmount:   money.FromDecimal(amount),
			FeeRate:         money.FromDecimal(decimal.Zero),
			FeeAmount:       money.FromDecimal(decimal.Zero),
			Currency:        currency,
			Status:          constants.WalletRechargeStatusSuccess,
			Remark:          remark,
			CreditTarget:    creditTarget,
			PointsAmount:    input.PointsAmount,
			PointsPerUnit:   input.PointsPerUnit,
			PaidAt:          &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := rechargeRepo.CreateRechargeOrder(recharge); err != nil {
			return ErrPaymentCreateFailed
		}

		// 全额从余额扣款；余额不足直接失败，整笔回滚。
		if _, _, err := s.walletSvc.SpendInTransaction(walletTx, walletcontract.CreditInput{
			UserID:    input.UserID,
			Amount:    money.FromDecimal(amount),
			Currency:  currency,
			Type:      constants.WalletTxnTypePointsPurchase,
			Reference: fmt.Sprintf("points_recharge_spend:%d", recharge.ID),
			Remark:    "购买积分",
		}); err != nil {
			return err
		}

		// 积分到账：复用在线充值成功的同一入账逻辑（含 reference 幂等）。
		if _, err := s.pointsSvc.ApplyRechargePayment(tx.Points(), recharge); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if payment == nil || recharge == nil {
		return nil, ErrPaymentCreateFailed
	}
	return &CreateWalletBalanceRechargeResult{Recharge: recharge, Payment: payment}, nil
}
