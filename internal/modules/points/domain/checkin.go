package domain

import "time"

// Checkin 每日签到记录。(user_id, checkin_date) 唯一索引挡住同日重复签到，
// 并发下由数据库唯一约束兜底，不依赖应用层判断。
type Checkin struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex:uk_points_checkin_user_date;not null" json:"user_id"`
	CheckinDate string    `gorm:"type:varchar(10);uniqueIndex:uk_points_checkin_user_date;not null" json:"checkin_date"`
	Points      int64     `gorm:"not null;default:0" json:"points"`
	StreakDays  int       `gorm:"not null;default:1" json:"streak_days"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}

func (Checkin) TableName() string {
	return "points_checkins"
}
