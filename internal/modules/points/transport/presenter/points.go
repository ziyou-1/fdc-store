package presenter

import (
	"time"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// AccountResp 积分账户响应。
type AccountResp struct {
	Balance     int64 `json:"balance"`
	TotalEarned int64 `json:"total_earned"`
	TotalSpent  int64 `json:"total_spent"`
}

// NewAccountResp 从积分账户构造响应。
func NewAccountResp(account *pointsdomain.Account) AccountResp {
	if account == nil {
		return AccountResp{}
	}
	return AccountResp{
		Balance:     account.Balance,
		TotalEarned: account.TotalEarned,
		TotalSpent:  account.TotalSpent,
	}
}

// TransactionResp 积分流水响应。
type TransactionResp struct {
	ID           uint      `json:"id"`
	Type         string    `json:"type"`
	Direction    string    `json:"direction"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewTransactionResp 从积分流水构造响应。
func NewTransactionResp(record *pointsdomain.Transaction) TransactionResp {
	return TransactionResp{
		ID:           record.ID,
		Type:         record.Type,
		Direction:    record.Direction,
		Amount:       record.Amount,
		BalanceAfter: record.BalanceAfter,
		Remark:       record.Remark,
		CreatedAt:    record.CreatedAt,
	}
}

// NewTransactionRespList 批量构造积分流水响应。
func NewTransactionRespList(records []pointsdomain.Transaction) []TransactionResp {
	list := make([]TransactionResp, 0, len(records))
	for index := range records {
		list = append(list, NewTransactionResp(&records[index]))
	}
	return list
}

// ConfigResp 积分规则响应。
type ConfigResp struct {
	CheckinEnabled              bool         `json:"checkin_enabled"`
	CheckinPoints               int64        `json:"checkin_points"`
	ReferralRegisterPoints      int64        `json:"referral_register_points"`
	ReferralFirstOrderPoints    int64        `json:"referral_first_order_points"`
	RechargePointsPerUnit       int64        `json:"recharge_points_per_unit"`
	ExchangeEnabled             bool         `json:"exchange_enabled"`
	ExchangeDailyLimit          int64        `json:"exchange_daily_limit"`
	ExchangeSKUDailyLimit       int64        `json:"exchange_sku_daily_limit"`
	RechargeEnabled             bool         `json:"recharge_enabled"`
	ReferralEnabled             bool         `json:"referral_enabled"`
	ReferralDailyLimit          int64        `json:"referral_daily_limit"`
	ReferralFirstOrderMinAmount money.Amount `json:"referral_first_order_min_amount"`
}

// NewConfigResp 从积分规则构造响应。
func NewConfigResp(config *pointsdomain.Config) ConfigResp {
	if config == nil {
		return ConfigResp{}
	}
	return ConfigResp{
		CheckinEnabled:              config.CheckinEnabled,
		CheckinPoints:               config.CheckinPoints,
		ReferralRegisterPoints:      config.ReferralRegisterPoints,
		ReferralFirstOrderPoints:    config.ReferralFirstOrderPoints,
		RechargePointsPerUnit:       config.RechargePointsPerUnit,
		ExchangeEnabled:             config.ExchangeEnabled,
		ExchangeDailyLimit:          config.ExchangeDailyLimit,
		ExchangeSKUDailyLimit:       config.ExchangeSKUDailyLimit,
		RechargeEnabled:             config.RechargeEnabled,
		ReferralEnabled:             config.ReferralEnabled,
		ReferralDailyLimit:          config.ReferralDailyLimit,
		ReferralFirstOrderMinAmount: config.ReferralFirstOrderMinAmount,
	}
}

// CheckinResp 签到结果响应。
type CheckinResp struct {
	Points     int64  `json:"points"`
	StreakDays int    `json:"streak_days"`
	Balance    int64  `json:"balance"`
	Date       string `json:"date"`
}

// PublicConfigResp 买家可见的积分规则子集。
// 只暴露展示所需字段：买家在充值前必须知道兑换比例，邀请前必须知道奖励额度。
// 推广/兑换的每日上限等运营参数不下发。
type PublicConfigResp struct {
	CheckinEnabled           bool  `json:"checkin_enabled"`
	CheckinPoints            int64 `json:"checkin_points"`
	RechargeEnabled          bool  `json:"recharge_enabled"`
	RechargePointsPerUnit    int64 `json:"recharge_points_per_unit"`
	ExchangeEnabled          bool  `json:"exchange_enabled"`
	ReferralEnabled          bool  `json:"referral_enabled"`
	ReferralRegisterPoints   int64 `json:"referral_register_points"`
	ReferralFirstOrderPoints int64 `json:"referral_first_order_points"`
}

// NewPublicConfigResp 从积分规则构造买家可见响应。
func NewPublicConfigResp(config *pointsdomain.Config) PublicConfigResp {
	if config == nil {
		return PublicConfigResp{}
	}
	return PublicConfigResp{
		CheckinEnabled:           config.CheckinEnabled,
		CheckinPoints:            config.CheckinPoints,
		RechargeEnabled:          config.RechargeEnabled,
		RechargePointsPerUnit:    config.RechargePointsPerUnit,
		ExchangeEnabled:          config.ExchangeEnabled,
		ReferralEnabled:          config.ReferralEnabled,
		ReferralRegisterPoints:   config.ReferralRegisterPoints,
		ReferralFirstOrderPoints: config.ReferralFirstOrderPoints,
	}
}

// InviteResp 买家推荐入口响应。
type InviteResp struct {
	Enabled          bool   `json:"enabled"`
	Code             string `json:"code"`
	RegisterPoints   int64  `json:"register_points"`
	FirstOrderPoints int64  `json:"first_order_points"`
}

// NewInviteResp 从推荐入口构造响应。
func NewInviteResp(invite *pointscontract.Invite) InviteResp {
	if invite == nil {
		return InviteResp{}
	}
	return InviteResp{
		Enabled:          invite.Enabled,
		Code:             invite.Code,
		RegisterPoints:   invite.RegisterPoints,
		FirstOrderPoints: invite.FirstOrderPoints,
	}
}
