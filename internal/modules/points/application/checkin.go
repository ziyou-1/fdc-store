package application

import (
	"errors"
	"fmt"
	"time"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

// checkinDateLayout 签到日期按站点本地时区的自然日计算。
const checkinDateLayout = "2006-01-02"

// Checkin 执行每日签到：同一自然日只能成功一次，积分入账与签到记录在同一事务内完成。
func (s *Service) Checkin(userID uint) (*pointscontract.CheckinResult, error) {
	if userID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	if s.transactions == nil {
		return nil, pointscontract.ErrTransactionRequired
	}

	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	if !config.CheckinEnabled {
		return nil, pointscontract.ErrCheckinDisabled
	}

	now := time.Now()
	today := now.Format(checkinDateLayout)
	yesterday := now.AddDate(0, 0, -1).Format(checkinDateLayout)

	var result *pointscontract.CheckinResult
	if err := s.transactions.WithinTransaction(func(tx pointscontract.Transaction) error {
		repository := tx.Points()
		if repository == nil {
			return pointscontract.ErrTransactionRequired
		}

		existing, err := repository.GetCheckinByDate(userID, today)
		if err != nil {
			return err
		}
		if existing != nil {
			return pointscontract.ErrAlreadyCheckedIn
		}

		streak := 1
		previous, err := repository.GetCheckinByDate(userID, yesterday)
		if err != nil {
			return err
		}
		if previous != nil {
			streak = previous.StreakDays + 1
		}

		record := &pointsdomain.Checkin{
			UserID:      userID,
			CheckinDate: today,
			Points:      config.CheckinPoints,
			StreakDays:  streak,
			CreatedAt:   now,
		}
		if err := repository.CreateCheckin(record); err != nil {
			return err
		}

		balance := int64(0)
		if config.CheckinPoints > 0 {
			account, _, creditErr := s.CreditInTransaction(tx, pointscontract.CreditInput{
				UserID:    userID,
				Amount:    config.CheckinPoints,
				Type:      pointscontract.TxnTypeCheckin,
				Reference: buildCheckinReference(userID, today),
				Remark:    "每日签到",
			})
			if creditErr != nil {
				return creditErr
			}
			balance = account.Balance
		} else {
			account, accountErr := ensureAccountForUpdate(repository, userID, now)
			if accountErr != nil {
				return accountErr
			}
			balance = account.Balance
		}

		result = &pointscontract.CheckinResult{
			Points:     config.CheckinPoints,
			StreakDays: streak,
			Balance:    balance,
			Date:       today,
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// CheckinStatus 返回今日签到状态，供前台按钮置灰与连签天数展示。
func (s *Service) CheckinStatus(userID uint) (*pointscontract.CheckinStatus, error) {
	if userID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	today := now.Format(checkinDateLayout)
	status := &pointscontract.CheckinStatus{
		Date:          today,
		CheckinPoints: config.CheckinPoints,
		Enabled:       config.CheckinEnabled,
	}

	record, err := s.points.GetCheckinByDate(userID, today)
	if err != nil {
		return nil, err
	}
	if record != nil {
		status.CheckedIn = true
		status.StreakDays = record.StreakDays
	} else if previous, prevErr := s.points.GetCheckinByDate(
		userID, now.AddDate(0, 0, -1).Format(checkinDateLayout),
	); prevErr == nil && previous != nil {
		status.StreakDays = previous.StreakDays
	}

	account, err := s.GetAccount(userID)
	if err != nil {
		return nil, err
	}
	status.Balance = account.Balance
	return status, nil
}

// GetConfig 读取积分规则，首次访问时落入默认值。
func (s *Service) GetConfig() (*pointsdomain.Config, error) {
	config, err := s.points.GetConfig()
	if err != nil {
		return nil, err
	}
	if config != nil {
		return config, nil
	}
	config = pointsdomain.DefaultConfig(time.Now())
	if err := s.points.SaveConfig(config); err != nil {
		return nil, pointscontract.ErrConfigUpdateFailed
	}
	return config, nil
}

// UpdateConfig 保存积分规则。所有数值不接受负数。
func (s *Service) UpdateConfig(input pointscontract.ConfigInput) (*pointsdomain.Config, error) {
	// 开启充值却把换算比例设为 0，会让用户付了钱拿到 0 积分。
	if input.RechargeEnabled && input.RechargePointsPerUnit <= 0 {
		return nil, pointscontract.ErrRechargeRateInvalid
	}
	// 开启推荐奖励却不设每日上限，等于开放无上限刷分（注册接口无限流无验证码）。
	if input.ReferralEnabled && input.ReferralDailyLimit <= 0 {
		return nil, pointscontract.ErrReferralLimitRequired
	}
	if input.CheckinPoints < 0 ||
		input.ReferralRegisterPoints < 0 ||
		input.ReferralFirstOrderPoints < 0 ||
		input.RechargePointsPerUnit < 0 ||
		input.ExchangeDailyLimit < 0 ||
		input.ExchangeSKUDailyLimit < 0 ||
		input.ReferralDailyLimit < 0 {
		return nil, pointscontract.ErrInvalidConfig
	}

	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	config.CheckinEnabled = input.CheckinEnabled
	config.CheckinPoints = input.CheckinPoints
	config.ReferralRegisterPoints = input.ReferralRegisterPoints
	config.ReferralFirstOrderPoints = input.ReferralFirstOrderPoints
	config.RechargePointsPerUnit = input.RechargePointsPerUnit
	config.ExchangeEnabled = input.ExchangeEnabled
	config.ExchangeDailyLimit = input.ExchangeDailyLimit
	config.ExchangeSKUDailyLimit = input.ExchangeSKUDailyLimit
	config.RechargeEnabled = input.RechargeEnabled
	config.ReferralEnabled = input.ReferralEnabled
	config.ReferralDailyLimit = input.ReferralDailyLimit
	config.ReferralFirstOrderMinAmount = input.ReferralFirstOrderMinAmount
	config.UpdatedAt = time.Now()
	if err := s.points.SaveConfig(config); err != nil {
		return nil, pointscontract.ErrConfigUpdateFailed
	}
	return config, nil
}

// IsAlreadyCheckedIn 便于上层区分"重复签到"与真实故障。
func IsAlreadyCheckedIn(err error) bool {
	return errors.Is(err, pointscontract.ErrAlreadyCheckedIn)
}

func buildCheckinReference(userID uint, date string) string {
	return fmt.Sprintf("checkin:%d:%s", userID, date)
}

func buildReference(scope string, userID uint, now time.Time) string {
	return fmt.Sprintf("%s:%d:%d", scope, userID, now.UnixNano())
}
