package application

import (
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func (s *Service) SetPlanRepository(repo memberlevelcontract.PlanRepository) {
	s.planRepo = repo
}

func (s *Service) SetPurchaseCreator(creator memberlevelcontract.PurchasePaymentCreator) {
	s.purchaseCreator = creator
}

func (s *Service) EnsureDefaultPlans() error {
	if s.planRepo == nil {
		return memberlevelcontract.ErrPlanNotFound
	}
	for _, seed := range memberleveldomain.DefaultPlans() {
		existing, err := s.planRepo.GetByPeriod(seed.Period)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		plan := seed
		if err := s.planRepo.Create(&plan); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListPlans() ([]memberleveldomain.Plan, error) {
	if s.planRepo == nil {
		return nil, memberlevelcontract.ErrPlanNotFound
	}
	return s.planRepo.List()
}

func (s *Service) ListActivePlans() ([]memberleveldomain.Plan, error) {
	plans, err := s.ListPlans()
	if err != nil {
		return nil, err
	}
	active := make([]memberleveldomain.Plan, 0, len(plans))
	for _, plan := range plans {
		if plan.IsActive && plan.Price.Decimal.GreaterThan(decimal.Zero) {
			active = append(active, plan)
		}
	}
	return active, nil
}

func (s *Service) UpdatePlan(period string, price money.Amount, name, description jsonmap.JSON, isActive bool) (*memberleveldomain.Plan, error) {
	if s.planRepo == nil {
		return nil, memberlevelcontract.ErrPlanNotFound
	}
	if _, ok := memberleveldomain.SpecForPeriod(period); !ok {
		return nil, memberlevelcontract.ErrInvalidPeriod
	}
	plan, err := s.planRepo.GetByPeriod(period)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, memberlevelcontract.ErrPlanNotFound
	}
	if price.Decimal.LessThan(decimal.Zero) {
		return nil, memberlevelcontract.ErrPlanNotPriced
	}
	plan.Price = money.FromDecimal(price.Decimal)
	if len(name) > 0 {
		plan.NameJSON = name
	}
	if len(description) > 0 {
		plan.DescriptionJSON = description
	}
	plan.IsActive = isActive
	if err := s.planRepo.Update(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *Service) ResolveMemberPriceForUser(user *userdomain.User, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	if user != nil && memberleveldomain.MembershipActive(user, time.Now()) {
		return memberleveldomain.ApplyDiscountRate(basePrice, user.MembershipDiscountRate.Decimal)
	}
	if user == nil || user.MemberLevelID == 0 {
		return basePrice, decimal.Zero
	}
	return s.ResolveMemberPrice(user.MemberLevelID, productID, skuID, basePrice)
}

func (s *Service) ApplyPurchasedMembership(userID uint, period string, durationDays int, discountRate decimal.Decimal) error {
	if s.userRepo == nil || userID == 0 {
		return memberlevelcontract.ErrUserNotFound
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return memberlevelcontract.ErrUserNotFound
	}
	if !memberleveldomain.ApplyPurchase(user, period, durationDays, discountRate, time.Now()) {
		return memberlevelcontract.ErrInvalidPeriod
	}
	return s.userRepo.Update(user)
}

// MembershipReminderLead 提前多久提醒续费。
//
// 三档时长为 30 / 90 / 365 天（SpecForPeriod），提前 7 天对最短的月卡
// 也落在购买后第 23 天，不会出现「刚买完就被催续费」。
// 若将来引入更短的体验卡，这里要改成 min(7d, 时长/4)。
const MembershipReminderLead = 7 * 24 * time.Hour

// ClaimExpiringMemberships 扫描即将到期的会员并逐个抢占提醒资格，
// 只返回本次抢到的那些用户——调用方拿到谁就给谁发信，天然去重。
//
// 抢占失败（已提醒过 / 并发下被别的 worker 拿走 / 期间刚续费）不是错误，
// 静默跳过即可。
func (s *Service) ClaimExpiringMemberships(now time.Time, limit int) ([]userdomain.User, error) {
	if s.userRepo == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	from := now
	to := now.Add(MembershipReminderLead)
	candidates, err := s.userRepo.ListMembershipsExpiringBetween(from, to, limit)
	if err != nil {
		return nil, err
	}

	claimed := make([]userdomain.User, 0, len(candidates))
	for index := range candidates {
		user := candidates[index]
		// 先在 Go 侧过滤掉明显已提醒过的，省掉一次无谓的 UPDATE。
		// 真正的判定仍在 SQL 的 WHERE 里，这里只是减少往返。
		if user.MembershipRemindedFor != nil && user.MembershipExpiresAt != nil &&
			user.MembershipRemindedFor.Equal(*user.MembershipExpiresAt) {
			continue
		}
		ok, claimErr := s.userRepo.ClaimMembershipExpiryReminder(user.ID, from, to)
		if claimErr != nil {
			return nil, claimErr
		}
		if ok {
			claimed = append(claimed, user)
		}
	}
	return claimed, nil
}

// ReleaseExpiryReminder 撤销一次抢占，供发信失败后让下一轮重试。
func (s *Service) ReleaseExpiryReminder(userID uint) error {
	if s.userRepo == nil {
		return nil
	}
	return s.userRepo.ReleaseMembershipExpiryReminder(userID)
}

// ExpireMemberships 把已到期的付费会员清档，返回受影响人数。
// 折扣本来就靠 MembershipActive 判活，这里清档是为了让买家侧不再显示
// 一个早已过期的「月度会员 · 9 折」，也让后台数据反映真实状态。
func (s *Service) ExpireMemberships(now time.Time) (int64, error) {
	if s.userRepo == nil {
		return 0, nil
	}
	return s.userRepo.ExpireMemberships(now)
}

func (s *Service) CreatePurchase(input memberlevelcontract.PurchasePaymentInput) (*memberlevelcontract.PurchaseResult, error) {
	if s.purchaseCreator == nil {
		return nil, memberlevelcontract.ErrPurchaseDisabled
	}
	if input.UserID == 0 {
		return nil, memberlevelcontract.ErrUserNotFound
	}
	period := strings.TrimSpace(input.Period)
	spec, ok := memberleveldomain.SpecForPeriod(period)
	if !ok {
		return nil, memberlevelcontract.ErrInvalidPeriod
	}
	if s.planRepo == nil {
		return nil, memberlevelcontract.ErrPlanNotFound
	}
	plan, err := s.planRepo.GetByPeriod(period)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, memberlevelcontract.ErrPlanNotFound
	}
	if !plan.IsActive {
		return nil, memberlevelcontract.ErrPlanInactive
	}
	if plan.Price.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, memberlevelcontract.ErrPlanNotPriced
	}
	return s.purchaseCreator.CreateMembershipPurchase(memberlevelcontract.PurchasePaymentInput{
		UserID:        input.UserID,
		ChannelID:     input.ChannelID,
		Amount:        plan.Price,
		Currency:      input.Currency,
		Remark:        strings.TrimSpace(input.Remark),
		ClientIP:      input.ClientIP,
		Context:       input.Context,
		RequestScheme: input.RequestScheme,
		Period:        spec.Period,
		DurationDays:  spec.DurationDays,
		DiscountRate:  money.FromDecimal(spec.DiscountRate),
	})
}
