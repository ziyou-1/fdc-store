package integrationtest

import (
	"testing"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestSpecForPeriodFixedDiscounts(t *testing.T) {
	cases := []struct {
		period string
		rate   string
		days   int
		ok     bool
	}{
		{memberleveldomain.PeriodMonth, "90", 30, true},
		{memberleveldomain.PeriodQuarter, "85", 90, true},
		{memberleveldomain.PeriodYear, "80", 365, true},
		{"", "0", 0, false},
		{"week", "0", 0, false},
	}
	for _, tc := range cases {
		spec, ok := memberleveldomain.SpecForPeriod(tc.period)
		if ok != tc.ok {
			t.Fatalf("period %q ok=%v want %v", tc.period, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if spec.DiscountRate.String() != tc.rate || spec.DurationDays != tc.days {
			t.Fatalf("period %q spec=%v/%d want %s/%d", tc.period, spec.DiscountRate, spec.DurationDays, tc.rate, tc.days)
		}
	}
}

func TestApplyDiscountRateExamples(t *testing.T) {
	price, discount := memberleveldomain.ApplyDiscountRate(decimal.RequireFromString("100"), decimal.NewFromInt(90))
	if !price.Equal(decimal.RequireFromString("90")) || !discount.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("9折: got %s / %s", price, discount)
	}
	price, discount = memberleveldomain.ApplyDiscountRate(decimal.RequireFromString("100"), decimal.NewFromInt(85))
	if !price.Equal(decimal.RequireFromString("85")) || !discount.Equal(decimal.RequireFromString("15")) {
		t.Fatalf("8.5折: got %s / %s", price, discount)
	}
	price, discount = memberleveldomain.ApplyDiscountRate(decimal.RequireFromString("10.00"), decimal.NewFromInt(85))
	if !price.Equal(decimal.RequireFromString("8.50")) || !discount.Equal(decimal.RequireFromString("1.50")) {
		t.Fatalf("10 * 8.5折: got %s / %s", price, discount)
	}
	price, discount = memberleveldomain.ApplyDiscountRate(decimal.RequireFromString("100"), decimal.NewFromInt(100))
	if !price.Equal(decimal.RequireFromString("100")) || !discount.IsZero() {
		t.Fatalf("无折扣: got %s / %s", price, discount)
	}
}

func TestApplyPurchaseAndActive(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	user := &userdomain.User{ID: 1}
	if memberleveldomain.MembershipActive(user, now) {
		t.Fatal("empty user should not be active")
	}
	if !memberleveldomain.ApplyPurchase(user, memberleveldomain.PeriodMonth, 30, decimal.NewFromInt(90), now) {
		t.Fatal("apply month")
	}
	if user.MembershipPeriod != memberleveldomain.PeriodMonth {
		t.Fatalf("period=%s", user.MembershipPeriod)
	}
	if !user.MembershipDiscountRate.Decimal.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("rate=%s", user.MembershipDiscountRate.Decimal)
	}
	wantExpiry := now.Add(30 * 24 * time.Hour)
	if user.MembershipExpiresAt == nil || !user.MembershipExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expires=%v want %v", user.MembershipExpiresAt, wantExpiry)
	}
	if !memberleveldomain.MembershipActive(user, now) {
		t.Fatal("just purchased should be active")
	}
	if memberleveldomain.MembershipActive(user, wantExpiry) {
		t.Fatal("expiry instant is not active")
	}
	if memberleveldomain.ApplyPurchase(user, "week", 7, decimal.NewFromInt(90), now) {
		t.Fatal("invalid period must fail")
	}
}

func TestEnsureDefaultPlansCreatesThreeOnce(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	svc.SetPlanRepository(gormstore.NewPlanStore(db))
	if err := db.AutoMigrate(&memberleveldomain.Plan{}); err != nil {
		t.Fatalf("migrate plans: %v", err)
	}
	if err := svc.EnsureDefaultPlans(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := svc.EnsureDefaultPlans(); err != nil {
		t.Fatalf("ensure again: %v", err)
	}
	plans, err := svc.ListPlans()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(plans) != 3 {
		t.Fatalf("plans=%d want 3", len(plans))
	}
}

func TestResolveMemberPriceForUserPrefersActivePurchase(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	level := createMemberLevelFixture(t, db, "gold", 20, "0", true)
	if err := db.Model(&level).Update("discount_rate", money.FromDecimal(decimal.NewFromInt(95))).Error; err != nil {
		t.Fatalf("set level rate: %v", err)
	}
	user := createUserFixture(t, db, "member-buy@example.com", level.ID)
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	user.MembershipPeriod = memberleveldomain.PeriodYear
	user.MembershipDiscountRate = money.FromDecimal(decimal.NewFromInt(80))
	user.MembershipExpiresAt = &expires
	if err := db.Save(&user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	price, discount := svc.ResolveMemberPriceForUser(&user, 1, 0, decimal.RequireFromString("100"))
	if !price.Equal(decimal.RequireFromString("80")) || !discount.Equal(decimal.RequireFromString("20")) {
		t.Fatalf("active purchase: got %s / %s", price, discount)
	}

	past := now.Add(-time.Hour)
	user.MembershipExpiresAt = &past
	price, discount = svc.ResolveMemberPriceForUser(&user, 1, 0, decimal.RequireFromString("100"))
	if !price.Equal(decimal.RequireFromString("95")) || !discount.Equal(decimal.RequireFromString("5")) {
		t.Fatalf("expired purchase fallback: got %s / %s", price, discount)
	}
}

// 续费必须在剩余时长之上叠加，而不是从今天重新起算——
// 否则「还剩 300 天时续一个月」会让买家凭空损失 300 天。
func TestApplyPurchaseStacksOntoRemainingTime(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	remaining := now.AddDate(0, 0, 300)
	user := &userdomain.User{
		MembershipPeriod:       memberleveldomain.PeriodYear,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(80)),
		MembershipExpiresAt:    &remaining,
	}

	if !memberleveldomain.ApplyPurchase(user, memberleveldomain.PeriodMonth, 30, decimal.NewFromInt(90), now) {
		t.Fatal("renewal must succeed")
	}

	want := remaining.AddDate(0, 0, 30)
	if !user.MembershipExpiresAt.Equal(want) {
		t.Fatalf("续费应叠加到 %s，实际 %s", want, user.MembershipExpiresAt)
	}
	// 档位以最新一次购买为准
	if user.MembershipPeriod != memberleveldomain.PeriodMonth {
		t.Fatalf("period 应更新为最新购买档位，实际 %q", user.MembershipPeriod)
	}
	if !user.MembershipDiscountRate.Decimal.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("折扣应更新为最新档位，实际 %s", user.MembershipDiscountRate.Decimal)
	}
}

// 已过期的会员再购买，从当下重新起算，不能把过去的时间也叠上去。
func TestApplyPurchaseRestartsFromNowWhenExpired(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	expired := now.AddDate(0, 0, -10)
	user := &userdomain.User{
		MembershipPeriod:       memberleveldomain.PeriodMonth,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(90)),
		MembershipExpiresAt:    &expired,
	}

	if !memberleveldomain.ApplyPurchase(user, memberleveldomain.PeriodMonth, 30, decimal.NewFromInt(90), now) {
		t.Fatal("purchase must succeed")
	}

	want := now.AddDate(0, 0, 30)
	if !user.MembershipExpiresAt.Equal(want) {
		t.Fatalf("过期后应从当下起算 %s，实际 %s", want, user.MembershipExpiresAt)
	}
}

// 首次购买没有旧到期时间，同样从当下起算。
func TestApplyPurchaseStartsFromNowForNewMember(t *testing.T) {
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	user := &userdomain.User{}

	if !memberleveldomain.ApplyPurchase(user, memberleveldomain.PeriodQuarter, 90, decimal.NewFromInt(85), now) {
		t.Fatal("first purchase must succeed")
	}
	want := now.AddDate(0, 0, 90)
	if !user.MembershipExpiresAt.Equal(want) {
		t.Fatalf("首购应为 %s，实际 %s", want, user.MembershipExpiresAt)
	}
}

func TestExpireMembershipsClearsOnlyExpiredRows(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	now := time.Now()

	past := now.Add(-time.Hour)
	future := now.Add(72 * time.Hour)
	expiredUser := &userdomain.User{
		Email:                  "expired@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipPeriod:       memberleveldomain.PeriodMonth,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(90)),
		MembershipExpiresAt:    &past,
	}
	activeUser := &userdomain.User{
		Email:                  "active@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipPeriod:       memberleveldomain.PeriodYear,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(80)),
		MembershipExpiresAt:    &future,
	}
	// 从未买过会员的普通用户不该被这条 UPDATE 扫到
	plainUser := &userdomain.User{
		Email:                  "plain@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(100)),
	}
	for _, u := range []*userdomain.User{expiredUser, activeUser, plainUser} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create %s: %v", u.Email, err)
		}
	}

	affected, err := svc.ExpireMemberships(now)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if affected != 1 {
		t.Fatalf("只应清掉 1 个过期会员，实际 %d", affected)
	}

	var reloadedExpired userdomain.User
	if err := db.First(&reloadedExpired, expiredUser.ID).Error; err != nil {
		t.Fatalf("reload expired: %v", err)
	}
	if reloadedExpired.MembershipPeriod != "" || reloadedExpired.MembershipExpiresAt != nil {
		t.Fatalf("过期会员应清档，实际 period=%q expires=%v",
			reloadedExpired.MembershipPeriod, reloadedExpired.MembershipExpiresAt)
	}
	if !reloadedExpired.MembershipDiscountRate.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("折扣率应回到 100，实际 %s", reloadedExpired.MembershipDiscountRate.Decimal)
	}

	var reloadedActive userdomain.User
	if err := db.First(&reloadedActive, activeUser.ID).Error; err != nil {
		t.Fatalf("reload active: %v", err)
	}
	if reloadedActive.MembershipPeriod != memberleveldomain.PeriodYear {
		t.Fatalf("未到期会员不得被清档，实际 %q", reloadedActive.MembershipPeriod)
	}

	// 再跑一次应为幂等：没有新的过期行
	again, err := svc.ExpireMemberships(now)
	if err != nil {
		t.Fatalf("expire again: %v", err)
	}
	if again != 0 {
		t.Fatalf("重复执行应无影响，实际 %d", again)
	}
}

// 到期提醒的核心保证：同一个会员在同一个到期周期内只能被抢占一次。
func TestClaimExpiringMembershipsOnlyClaimsOncePerTerm(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	now := time.Now()

	// 3 天后到期，落在 7 天提醒窗口内
	soon := now.Add(72 * time.Hour)
	user := &userdomain.User{
		Email:                  "expiring@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		Locale:                 "zh-CN",
		MembershipPeriod:       memberleveldomain.PeriodMonth,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(90)),
		MembershipExpiresAt:    &soon,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	first, err := svc.ClaimExpiringMemberships(now, 100)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(first) != 1 || first[0].ID != user.ID {
		t.Fatalf("首轮应抢到该会员，实际 %#v", first)
	}

	// 第二轮（模拟下一个小时的定时任务）必须一个都抢不到
	second, err := svc.ClaimExpiringMemberships(now, 100)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("同一到期周期不得重复提醒，第二轮却抢到 %d 个", len(second))
	}
}

// 窗口外的会员不该被扫到：刚买完年卡的人不能立刻收到催续费。
func TestClaimExpiringMembershipsIgnoresOutsideWindow(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	now := time.Now()

	far := now.Add(30 * 24 * time.Hour) // 30 天后到期，远在 7 天窗口外
	if err := db.Create(&userdomain.User{
		Email:                  "far@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipPeriod:       memberleveldomain.PeriodYear,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(80)),
		MembershipExpiresAt:    &far,
	}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	// 从未买过会员的人也不该被扫到
	if err := db.Create(&userdomain.User{
		Email:                  "plain@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(100)),
	}).Error; err != nil {
		t.Fatalf("create plain: %v", err)
	}

	claimed, err := svc.ClaimExpiringMemberships(now, 100)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("窗口外与非会员都不该被抢占，实际抢到 %d 个", len(claimed))
	}
}

// 续费后到期日变大，必须能为新周期重新提醒一次——
// 这是「幂等 key 绑定到期时刻」而不是「绑定用户」的关键收益。
func TestClaimExpiringMembershipsRearmsAfterRenewal(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	now := time.Now()

	soon := now.Add(48 * time.Hour)
	user := &userdomain.User{
		Email:                  "renew@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipPeriod:       memberleveldomain.PeriodMonth,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(90)),
		MembershipExpiresAt:    &soon,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if claimed, err := svc.ClaimExpiringMemberships(now, 100); err != nil || len(claimed) != 1 {
		t.Fatalf("首轮应抢到，err=%v n=%d", err, len(claimed))
	}

	// 买家续费：到期日从 +48h 变成 +48h+30d
	var reloaded userdomain.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !memberleveldomain.ApplyPurchase(
		&reloaded, memberleveldomain.PeriodMonth, 30, decimal.NewFromInt(90), now,
	) {
		t.Fatal("renew must succeed")
	}
	if err := db.Save(&reloaded).Error; err != nil {
		t.Fatalf("save renewal: %v", err)
	}

	// 新到期日在 7 天窗口外，此刻不该被抢
	if claimed, err := svc.ClaimExpiringMemberships(now, 100); err != nil || len(claimed) != 0 {
		t.Fatalf("续费后不该立刻再提醒，err=%v n=%d", err, len(claimed))
	}

	// 快进到新到期日前 3 天：应当重新武装并再抢到一次
	later := reloaded.MembershipExpiresAt.Add(-72 * time.Hour)
	claimed, err := svc.ClaimExpiringMemberships(later, 100)
	if err != nil {
		t.Fatalf("claim after renewal: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("续费后的新周期应能再提醒一次，实际抢到 %d 个", len(claimed))
	}
}

// 发信失败后撤销抢占，下一轮必须能重试。
func TestReleaseExpiryReminderAllowsRetry(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	now := time.Now()

	soon := now.Add(24 * time.Hour)
	user := &userdomain.User{
		Email:                  "retry@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipPeriod:       memberleveldomain.PeriodMonth,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(90)),
		MembershipExpiresAt:    &soon,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if claimed, err := svc.ClaimExpiringMemberships(now, 100); err != nil || len(claimed) != 1 {
		t.Fatalf("首轮应抢到，err=%v n=%d", err, len(claimed))
	}
	// 模拟 SMTP 失败后的撤销
	if err := svc.ReleaseExpiryReminder(user.ID); err != nil {
		t.Fatalf("release: %v", err)
	}
	claimed, err := svc.ClaimExpiringMemberships(now, 100)
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("撤销后下一轮应能重试，实际抢到 %d 个", len(claimed))
	}
}

// 到期清档必须一并清掉提醒标记，避免下个会员周期漏发提醒。
func TestExpireMembershipsAlsoClearsReminderMark(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	now := time.Now()

	past := now.Add(-time.Hour)
	user := &userdomain.User{
		Email:                  "cleared@example.com",
		PasswordHash:           "x",
		Status:                 "active",
		MembershipPeriod:       memberleveldomain.PeriodMonth,
		MembershipDiscountRate: money.FromDecimal(decimal.NewFromInt(90)),
		MembershipExpiresAt:    &past,
		MembershipRemindedFor:  &past,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.ExpireMemberships(now); err != nil {
		t.Fatalf("expire: %v", err)
	}
	var reloaded userdomain.User
	if err := db.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.MembershipRemindedFor != nil {
		t.Fatalf("清档时应一并清掉提醒标记，实际 %v", reloaded.MembershipRemindedFor)
	}
}
