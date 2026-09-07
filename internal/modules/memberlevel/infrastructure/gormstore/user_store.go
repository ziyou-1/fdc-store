package gormstore

import (
	"errors"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// UserStore persists only the user fields owned by member-level progression.
type UserStore struct {
	db *gorm.DB
}

func NewUserStore(db *gorm.DB) *UserStore {
	return &UserStore{db: db}
}

func (r *UserStore) GetByID(id uint) (*userdomain.User, error) {
	var user userdomain.User
	if err := r.db.Where("id = ? AND deleted_at IS NULL", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserStore) Update(user *userdomain.User) error {
	if user == nil || user.ID == 0 {
		return nil
	}
	return r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", user.ID).
		Select("*").
		Omit("id", "deleted_at").
		Updates(user).Error
}

func (r *UserStore) IncrementTotalRecharged(userID uint, amount decimal.Decimal) error {
	return r.incrementMoneyColumn(userID, "total_recharged", amount)
}

func (r *UserStore) IncrementTotalSpent(userID uint, amount decimal.Decimal) error {
	return r.incrementMoneyColumn(userID, "total_spent", amount)
}

func (r *UserStore) incrementMoneyColumn(userID uint, column string, amount decimal.Decimal) error {
	if userID == 0 {
		return nil
	}
	amount = amount.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	return r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]interface{}{
			column:       gorm.Expr(column+" + ?", money.FromDecimal(amount)),
			"updated_at": time.Now(),
		}).Error
}

func (r *UserStore) UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error) {
	if userID == 0 || currentLevelID == nextLevelID {
		return 0, nil
	}
	result := r.db.Model(&userdomain.User{}).
		Where("id = ? AND member_level_id = ? AND deleted_at IS NULL", userID, currentLevelID).
		Updates(map[string]interface{}{
			"member_level_id": nextLevelID,
			"updated_at":      time.Now(),
		})
	return result.RowsAffected, result.Error
}

func (r *UserStore) AssignDefaultMemberLevel(defaultLevelID uint) (int64, error) {
	if defaultLevelID == 0 {
		return 0, nil
	}
	result := r.db.Model(&userdomain.User{}).
		Where("(member_level_id = 0 OR member_level_id IS NULL) AND deleted_at IS NULL").
		Updates(map[string]interface{}{
			"member_level_id": defaultLevelID,
			"updated_at":      time.Now(),
		})
	return result.RowsAffected, result.Error
}

// ExpireMemberships 清掉已到期的付费会员档位。
// 折扣率回到 100（不打折）而不是 0——0 在 MembershipActive 里是非法值。
func (r *UserStore) ExpireMemberships(now time.Time) (int64, error) {
	result := r.db.Model(&userdomain.User{}).
		Where("membership_expires_at IS NOT NULL AND membership_expires_at <= ? AND membership_period <> '' AND deleted_at IS NULL", now).
		Updates(map[string]interface{}{
			"membership_period":        "",
			"membership_discount_rate": 100,
			"membership_expires_at":    nil,
			// 一并清掉提醒标记：否则下次买会员时若到期时刻碰巧与旧值相同，
			// 抢占条件会误判成「已提醒过」而漏发。
			"membership_reminded_for": nil,
			"updated_at":              time.Now(),
		})
	return result.RowsAffected, result.Error
}

// ListMembershipsExpiringBetween 返回到期时间落在 [from, to) 的付费会员。
func (r *UserStore) ListMembershipsExpiringBetween(from, to time.Time, limit int) ([]userdomain.User, error) {
	if limit <= 0 {
		limit = 200
	}
	var users []userdomain.User
	err := r.db.
		Where("membership_expires_at IS NOT NULL AND membership_expires_at >= ? AND membership_expires_at < ? AND membership_period <> '' AND deleted_at IS NULL", from, to).
		Order("membership_expires_at ASC").
		Limit(limit).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// ClaimMembershipExpiryReminder 抢占「为当前到期时刻发提醒」的资格。
//
// 幂等完全靠这一条 UPDATE 的 WHERE 条件保证，不依赖 Redis——
// cache.SetNX 在 Redis 未启用时会直接返回 true，用它做跨天幂等会静默失效。
//
// membership_reminded_for 存的是到期时刻本身：续费把 membership_expires_at 改大后，
// 两列不再相等，下一个周期自动重新武装。
func (r *UserStore) ClaimMembershipExpiryReminder(userID uint, from, to time.Time) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	result := r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Where("membership_period <> '' AND membership_expires_at IS NOT NULL").
		// 再次校验到期时刻仍落在扫描窗口内：防止「扫描之后、抢占之前」用户刚续费，
		// 把新周期的提醒资格提前消耗掉。
		Where("membership_expires_at >= ? AND membership_expires_at < ?", from, to).
		Where("membership_reminded_for IS NULL OR membership_reminded_for <> membership_expires_at").
		Updates(map[string]interface{}{
			"membership_reminded_for": gorm.Expr("membership_expires_at"),
			"updated_at":              time.Now(),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ReleaseMembershipExpiryReminder 撤销抢占，让下一轮扫描重试。
func (r *UserStore) ReleaseMembershipExpiryReminder(userID uint) error {
	if userID == 0 {
		return nil
	}
	return r.db.Model(&userdomain.User{}).
		Where("id = ? AND deleted_at IS NULL", userID).
		Updates(map[string]interface{}{
			"membership_reminded_for": nil,
			"updated_at":              time.Now(),
		}).Error
}

var _ memberlevelcontract.UserRepository = (*UserStore)(nil)
