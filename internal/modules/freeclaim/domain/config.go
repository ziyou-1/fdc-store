package domain

import "time"

// ConfigSingletonID 免费领取配置为单行记录，固定主键。
const ConfigSingletonID uint = 1

// Config 免费领取规则，全部后台可改。
type Config struct {
	ID uint `gorm:"primarykey" json:"id"`
	// Enabled 总开关，默认关闭以便灰度。
	Enabled bool `gorm:"not null;default:false" json:"enabled"`
	// MemberOnly 是否会员专享。需求要求"会员专享免费领取"，默认开启。
	MemberOnly bool `gorm:"not null;default:true" json:"member_only"`
	// MinMemberLevelID 允许领取的最低会员等级，0 表示任意会员等级均可。
	MinMemberLevelID uint      `gorm:"not null;default:0" json:"min_member_level_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (Config) TableName() string {
	return "free_claim_configs"
}

// DefaultConfig 返回内置默认值：功能关闭、会员专享。
func DefaultConfig(now time.Time) *Config {
	return &Config{
		ID:         ConfigSingletonID,
		Enabled:    false,
		MemberOnly: true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}
