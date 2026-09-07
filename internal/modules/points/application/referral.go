package application

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

// AwardReferralRegister 给推荐人发放"被邀请人注册"奖励。
//
// reference 用被邀请人的 userID 而不是推荐人的：一个被邀请人这辈子只能带来一次注册奖励，
// 做成稳定业务键后，重复调用（重试、并发、补偿脚本）天然只入账一次。
//
// 但要清楚这层幂等挡不住什么：它保证"同一个被邀请人不重复发奖"，
// 挡不住"换一个新号再来一次"。真正的风控是下面的推荐人每日上限。
func (s *Service) AwardReferralRegister(inviterUserID, inviteeUserID uint) error {
	return s.awardReferral(
		inviterUserID,
		inviteeUserID,
		pointscontract.TxnTypeReferralRegister,
		fmt.Sprintf("referral_register:%d", inviteeUserID),
		"推荐注册奖励",
		func(config *pointsdomain.Config) int64 { return config.ReferralRegisterPoints },
	)
}

// AwardReferralFirstOrder 给推荐人发放"被邀请人首单"奖励。
//
// 刻意不去查询"这是不是第一单"：每次支付成功都尝试入账，由 reference 唯一索引
// 保证只有第一次真正落账，之后静默 no-op。这样既避免了"第一单"定义的模糊
// （父子订单、混合支付、部分退款各有歧义），也天然免疫并发与回调重投。
// orderAmount 是订单实付口径的金额，用于最低金额门槛判断。
func (s *Service) AwardReferralFirstOrder(inviterUserID, inviteeUserID uint, orderAmount decimal.Decimal) error {
	config, err := s.GetConfig()
	if err != nil {
		return err
	}
	// 最低金额门槛：余额全额支付走合成支付分支，会绕过支付渠道的最小金额校验，
	// 0.01 元订单同样能成交。没有这道门槛，首单奖励对刷子必然是净赚的。
	minAmount := config.ReferralFirstOrderMinAmount.Decimal.Round(2)
	if minAmount.GreaterThan(decimal.Zero) && orderAmount.Round(2).LessThan(minAmount) {
		return nil
	}
	return s.awardReferral(
		inviterUserID,
		inviteeUserID,
		pointscontract.TxnTypeReferralOrder,
		fmt.Sprintf("referral_first_order:%d", inviteeUserID),
		"推荐首单奖励",
		func(config *pointsdomain.Config) int64 { return config.ReferralFirstOrderPoints },
	)
}

func (s *Service) awardReferral(
	inviterUserID, inviteeUserID uint,
	transactionType, reference, remark string,
	pick func(*pointsdomain.Config) int64,
) error {
	if inviterUserID == 0 || inviteeUserID == 0 {
		return nil
	}
	// 自己邀请自己不发奖。注册时被邀请人尚未存在，理论上撞不上，
	// 但补偿脚本或后台改数据都可能造出这种数据，这里兜住。
	if inviterUserID == inviteeUserID {
		return nil
	}

	config, err := s.GetConfig()
	if err != nil {
		return err
	}
	if !config.ReferralEnabled {
		return nil
	}
	points := pick(config)
	// 奖励额度为 0 表示该项未开启，直接跳过：不写零额流水，保持流水表干净。
	if points <= 0 {
		return nil
	}

	// 推荐人维度的每日上限：这是本功能唯一能限制"量产小号刷分"的地方。
	// 注册接口没有限流也没有验证码，邮箱 +tag 又能无限派生，
	// 所以只能在发奖侧封顶，而不是指望上游拦住注册。
	if config.ReferralDailyLimit > 0 {
		used, sumErr := s.sumReferralPointsToday(inviterUserID)
		if sumErr != nil {
			return sumErr
		}
		if used+points > config.ReferralDailyLimit {
			return nil
		}
	}

	_, _, err = s.Credit(pointscontract.CreditInput{
		UserID:    inviterUserID,
		Amount:    points,
		Type:      transactionType,
		Reference: reference,
		Remark:    remark,
	})
	return err
}

// sumReferralPointsToday 统计推荐人当日已获得的推荐类积分合计（注册 + 首单）。
func (s *Service) sumReferralPointsToday(inviterUserID uint) (int64, error) {
	since := startOfDay(time.Now())
	total := int64(0)
	for _, transactionType := range []string{
		pointscontract.TxnTypeReferralRegister,
		pointscontract.TxnTypeReferralOrder,
	} {
		amount, err := s.points.SumPointsByTypeSince(inviterUserID, transactionType, since)
		if err != nil {
			return 0, err
		}
		total += amount
	}
	return total, nil
}
