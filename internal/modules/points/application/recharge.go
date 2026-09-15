package application

import (
	"fmt"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// ResolveRechargePoints 把充值金额换算成积分。
// 乘法 + 向下取整：宁可少给也不多给；perUnit 非正时返回 0，天然规避除零
// （UpdateConfig 历史上允许 RechargePointsPerUnit 为 0）。
func ResolveRechargePoints(amount money.Amount, perUnit int64) int64 {
	if perUnit <= 0 {
		return 0
	}
	return amount.Decimal.Round(2).Mul(decimal.NewFromInt(perUnit)).Floor().IntPart()
}

// ApplyRechargePayment 把一张已支付成功的充值单入账为积分。
//
// 到账积分取建单时的快照 recharge.PointsAmount，绝不在回调里重读配置——
// 否则运营中途调整换算比例会让用户拿到与下单时不一致的积分。
// 幂等由 CreditInTransaction 的 reference 先查短路 + reference 唯一索引共同保证。
func (s *Service) ApplyRechargePayment(
	tx pointscontract.Transaction,
	recharge *walletdomain.RechargeOrder,
) (*pointsdomain.Transaction, error) {
	if tx == nil {
		return nil, pointscontract.ErrTransactionRequired
	}
	if recharge == nil || recharge.ID == 0 || recharge.UserID == 0 {
		return nil, pointscontract.ErrRechargeNotFound
	}
	if recharge.PointsAmount <= 0 {
		return nil, pointscontract.ErrInvalidAmount
	}
	_, transaction, err := s.CreditInTransaction(tx, pointscontract.CreditInput{
		UserID:    recharge.UserID,
		Amount:    recharge.PointsAmount,
		Type:      pointscontract.TxnTypeRecharge,
		Reference: fmt.Sprintf("points_recharge:%d:success", recharge.ID),
		Remark:    cleanRemark(recharge.Remark, "在线充值积分到账"),
	})
	return transaction, err
}

// CreateRecharge 编排积分充值：读配置 → 校验 → 换算快照 → 通过端口建单。
//
// 建单本身要落到支付模块（复用整条钱包充值链路），但 points/application 不能
// import payment/application —— 后者已经 import 了 pointsapp，反向引用即编译环。
// 因此这里只依赖 RechargePaymentCreator 端口，实现由装配层注入。
func (s *Service) CreateRecharge(input pointscontract.RechargeInput) (*pointscontract.RechargeResult, error) {
	if s.rechargeCreator == nil {
		return nil, pointscontract.ErrRechargeDisabled
	}
	if input.UserID == 0 {
		return nil, pointscontract.ErrExchangeGuestForbidden
	}
	if input.Amount.Decimal.Round(2).LessThanOrEqual(decimal.Zero) {
		return nil, pointscontract.ErrInvalidAmount
	}

	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	if !config.RechargeEnabled {
		return nil, pointscontract.ErrRechargeDisabled
	}
	if config.RechargePointsPerUnit <= 0 {
		return nil, pointscontract.ErrRechargeRateInvalid
	}

	points := ResolveRechargePoints(input.Amount, config.RechargePointsPerUnit)
	if points <= 0 {
		return nil, pointscontract.ErrInvalidAmount
	}

	return s.rechargeCreator.CreatePointsRecharge(pointscontract.RechargePaymentInput{
		UserID:         input.UserID,
		ChannelID:      input.ChannelID,
		Amount:         input.Amount,
		Currency:       input.Currency,
		Remark:         input.Remark,
		ClientIP:       input.ClientIP,
		Context:        input.Context,
		RequestScheme:  input.RequestScheme,
		PointsAmount:   points,
		PointsPerUnit:  config.RechargePointsPerUnit,
		PayWithBalance: input.PayWithBalance,
	})
}

// ListRecharges 分页返回用户的积分充值记录。
func (s *Service) ListRecharges(userID uint, page, pageSize int, status string) ([]pointscontract.RechargeRecord, int64, error) {
	if s.rechargeCreator == nil {
		return nil, 0, pointscontract.ErrRechargeDisabled
	}
	if userID == 0 {
		return nil, 0, pointscontract.ErrExchangeGuestForbidden
	}
	return s.rechargeCreator.ListPointsRecharges(userID, page, pageSize, status)
}
