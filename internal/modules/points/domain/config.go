package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// ConfigSingletonID 积分配置为单行记录，固定主键。
const ConfigSingletonID uint = 1

// Config 积分规则配置，全部由后台可改，不写死在代码里。
type Config struct {
	ID                       uint  `gorm:"primarykey" json:"id"`
	CheckinEnabled           bool  `gorm:"not null;default:true" json:"checkin_enabled"`
	CheckinPoints            int64 `gorm:"not null;default:5" json:"checkin_points"`
	ReferralRegisterPoints   int64 `gorm:"not null;default:0" json:"referral_register_points"`
	ReferralFirstOrderPoints int64 `gorm:"not null;default:0" json:"referral_first_order_points"`
	RechargePointsPerUnit    int64 `gorm:"not null;default:1" json:"recharge_points_per_unit"`
	// ExchangeEnabled 积分兑换商品总开关。DefaultConfig 默认开启；列默认仍为 false，已有库靠配置页或显式更新。
	ExchangeEnabled bool `gorm:"not null;default:false" json:"exchange_enabled"`
	// ExchangeDailyLimit 单用户每日可兑换积分上限，0 表示不限。
	ExchangeDailyLimit int64 `gorm:"not null;default:0" json:"exchange_daily_limit"`
	// ExchangeSKUDailyLimit 单 SKU 每日可被兑换件数上限，0 表示不限。
	// content 交付商品完全不占库存，只有用户维度上限挡不住热门教程被全站薅光。
	ExchangeSKUDailyLimit int64 `gorm:"not null;default:0" json:"exchange_sku_daily_limit"`
	// RechargeEnabled 积分充值总开关。DefaultConfig 默认开启；列默认仍为 false，已有库靠配置页或显式更新。
	RechargeEnabled bool `gorm:"not null;default:false" json:"recharge_enabled"`
	// ReferralEnabled 推荐奖励总开关。独立于 affiliate 模块的开关：
	// 邀请码虽复用 affiliate_code，但站长要能"只要积分推荐、不要现金分销"。
	ReferralEnabled bool `gorm:"not null;default:false" json:"referral_enabled"`
	// ReferralDailyLimit 单个推荐人每日可获得的推荐积分上限，0 表示不限。
	//
	// 这是本功能最重要的风控项，不是可选项：注册接口本身既无限流也无验证码
	// （captcha 场景表里没有 register），且 normalizeEmail 不剥离 +tag，
	// 因此"零成本量产被邀请人"在底座层面是成立的。reference 唯一索引只能保证
	// 同一个被邀请人不重复发奖，挡不住换一个新号再来一次。
	// 开启推荐奖励却把这个值留成 0，等于开放无上限刷分。
	ReferralDailyLimit int64 `gorm:"not null;default:0" json:"referral_daily_limit"`
	// ReferralFirstOrderMinAmount 触发首单奖励所需的最低订单金额，0 表示不限。
	// 余额全额支付会走合成支付分支，绕过支付渠道的最小金额校验，
	// 因此 0.01 元订单也能成交——没有这个门槛，首单奖励必然是净赚的。
	ReferralFirstOrderMinAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"referral_first_order_min_amount"`
	CreatedAt                   time.Time    `json:"created_at"`
	UpdatedAt                   time.Time    `json:"updated_at"`
}

func (Config) TableName() string {
	return "points_configs"
}

// DefaultConfig 返回内置默认值：每日签到 +5，充值 1:1，兑换与充值开启，
// 推荐注册 +10 / 首单 +20，每日上限 100（开启推荐必须带上限，否则等于开放刷分）。
func DefaultConfig(now time.Time) *Config {
	return &Config{
		ID:                       ConfigSingletonID,
		CheckinEnabled:           true,
		CheckinPoints:            5,
		ReferralRegisterPoints:   10,
		ReferralFirstOrderPoints: 20,
		RechargePointsPerUnit:    1,
		ExchangeEnabled:          true,
		RechargeEnabled:          true,
		ReferralEnabled:          true,
		ReferralDailyLimit:       100,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
}
