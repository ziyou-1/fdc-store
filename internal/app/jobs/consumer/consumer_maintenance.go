package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/mailbrand"
	"github.com/dujiao-next/internal/telegramidentity"

	"github.com/hibiken/asynq"
)

// handleNotificationDispatch 处理通知中心异步分发任务。
func (c *Consumer) handleNotificationDispatch(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_notification_dispatch_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	if c.NotificationService == nil {
		logger.Warnw("worker_notification_dispatch_skip_service_nil")
		return nil
	}
	var payload queue.NotificationDispatchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_notification_dispatch_unmarshal_failed", "error", err)
		return err
	}
	if strings.TrimSpace(payload.EventType) == "" {
		logger.Debugw("worker_notification_dispatch_skip_empty_event")
		return nil
	}
	if err := c.NotificationService.Dispatch(ctx, payload); err != nil {
		logger.Warnw("worker_notification_dispatch_failed",
			"event_type", payload.EventType,
			"biz_type", payload.BizType,
			"biz_id", payload.BizID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleAffiliateConfirmCommissions 处理分销佣金确认任务。
func (c *Consumer) handleAffiliateConfirmCommissions(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.AffiliateService == nil {
		logger.Debugw("worker_affiliate_confirm_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	if err := c.AffiliateService.ConfirmDueCommissions(time.Now()); err != nil {
		logger.Warnw("worker_affiliate_confirm_due_failed", "error", err)
		return err
	}
	return nil
}

func (c *Consumer) handleMembershipExpire(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.MemberLevelService == nil {
		logger.Debugw("worker_membership_expire_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	affected, err := c.MemberLevelService.ExpireMemberships(time.Now())
	if err != nil {
		logger.Warnw("worker_membership_expire_failed", "error", err)
		return err
	}
	if affected > 0 {
		logger.Infow("worker_membership_expire_ok", "affected", affected)
	}
	return nil
}

// membershipExpiryEmailCopy 是到期提醒的三语文案。
// 会员权益文案在后台可配，但这封提醒是系统行为通知而非营销素材，
// 因此与订单状态邮件一样内置，避免为一封信引入一套模板配置。
var membershipExpiryEmailCopy = map[string]struct {
	Subject string
	Body    string
}{
	"zh-CN": {
		Subject: "您的会员将于 %s 到期",
		Body:    "您好，\n\n您在 %s 的会员将于 %s 到期。到期后将不再享受会员折扣。\n\n续费可在「个人中心 - 会员」页面完成，续费时长会叠加在剩余时间之上，不会浪费。\n\n%s",
	},
	"zh-TW": {
		Subject: "您的會員將於 %s 到期",
		Body:    "您好，\n\n您在 %s 的會員將於 %s 到期。到期後將不再享有會員折扣。\n\n續費可在「個人中心 - 會員」頁面完成，續費時長會疊加在剩餘時間之上，不會浪費。\n\n%s",
	},
	"en-US": {
		Subject: "Your membership expires on %s",
		Body:    "Hello,\n\nYour membership at %s expires on %s. Member pricing stops applying after that date.\n\nYou can renew from Personal Center - Membership. Renewing adds time on top of what is left, so nothing is wasted.\n\n%s",
	},
}

// buildMembershipExpiryEmail 按买家语言渲染到期提醒邮件。
// 未知语言回落简体中文，与站点默认 locale 一致。
func buildMembershipExpiryEmail(locale, siteName, siteURL string, expiresAt time.Time) (string, string) {
	copyText, ok := membershipExpiryEmailCopy[strings.TrimSpace(locale)]
	if !ok {
		copyText = membershipExpiryEmailCopy["zh-CN"]
	}
	date := expiresAt.Format("2006-01-02")
	return fmt.Sprintf(copyText.Subject, date),
		fmt.Sprintf(copyText.Body, siteName, date, siteURL)
}

// handleMembershipExpiryReminder 扫描 7 天内到期的付费会员并发送续费提醒。
//
// 幂等由 ClaimExpiringMemberships 的条件 UPDATE 保证：抢到的才发，
// 因此本任务重复执行、worker 重启或 asynq 重试都不会重复打扰同一个人。
func (c *Consumer) handleMembershipExpiryReminder(ctx context.Context, _ *asynq.Task) error {
	if c == nil || c.MemberLevelService == nil {
		logger.Debugw("worker_membership_reminder_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	if c.EmailSender == nil {
		logger.Debugw("worker_membership_reminder_skip_email_nil")
		return nil
	}
	// 与订单邮件一致地做消费侧开关校验：SMTP 关闭时不发，也不重试。
	if c.SettingService != nil && c.Config != nil {
		smtpSetting, smtpErr := c.SettingService.GetSMTPSetting(c.Config.Email)
		if smtpErr != nil {
			logger.Warnw("worker_membership_reminder_load_smtp_failed", "error", smtpErr)
			return nil
		}
		if !smtpSetting.Enabled {
			logger.Debugw("worker_membership_reminder_skip_smtp_disabled")
			return nil
		}
	}

	claimed, err := c.MemberLevelService.ClaimExpiringMemberships(time.Now(), 200)
	if err != nil {
		logger.Warnw("worker_membership_reminder_claim_failed", "error", err)
		return err
	}
	if len(claimed) == 0 {
		return nil
	}

	// 会员是平台级权益，不属于任何分销商，因此固定用主站品牌。
	brand := mailbrand.Brand{}
	if c.EmailBrandResolver != nil {
		resolved, brandErr := c.EmailBrandResolver.ResolveEmailBrand(ctx, mailbrand.Scope{})
		if brandErr != nil {
			logger.Warnw("worker_membership_reminder_load_brand_failed", "error", brandErr)
			return brandErr
		}
		brand = resolved
	}
	siteName, siteURL := brand.SiteName, brand.SiteURL
	sent := 0
	for index := range claimed {
		user := claimed[index]
		email := strings.TrimSpace(user.Email)
		// Telegram 注册用户是占位邮箱，发出去只会硬退信。
		if email == "" || telegramidentity.IsPlaceholderEmail(email) {
			continue
		}
		if user.MembershipExpiresAt == nil {
			continue
		}
		subject, body := buildMembershipExpiryEmail(user.Locale, siteName, siteURL, *user.MembershipExpiresAt)
		if sendErr := c.EmailSender.SendCustomEmail(email, subject, body); sendErr != nil {
			// 发失败就把抢占撤销，让下一轮重试；不返回 error，
			// 否则 asynq 会重跑整个任务，已发成功的那些人不会重复
			// （抢占还在），但没必要为一个人重扫全窗口。
			logger.Warnw("worker_membership_reminder_send_failed", "user_id", user.ID, "error", sendErr)
			if releaseErr := c.MemberLevelService.ReleaseExpiryReminder(user.ID); releaseErr != nil {
				logger.Warnw("worker_membership_reminder_release_failed", "user_id", user.ID, "error", releaseErr)
			}
			continue
		}
		sent++
	}
	if sent > 0 {
		logger.Infow("worker_membership_reminder_ok", "claimed", len(claimed), "sent", sent)
	}
	return nil
}

func (c *Consumer) handleResellerConfirmLedger(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ResellerAccountingLedger == nil {
		logger.Debugw("worker_reseller_confirm_ledger_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	affected, err := c.ResellerAccountingLedger.ConfirmDueLedgerEntries(time.Now())
	if err != nil {
		logger.Warnw("worker_reseller_confirm_ledger_failed", "error", err)
		return err
	}
	logger.Debugw("worker_reseller_confirm_ledger_ok", "affected", affected)
	return nil
}

// handleReconciliationRun 处理对账任务执行。
func (c *Consumer) handleReconciliationRun(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ReconciliationService == nil {
		logger.Debugw("worker_reconciliation_run_skip_nil")
		return nil
	}
	var payload queue.ReconciliationRunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_reconciliation_run_unmarshal_failed", "error", err)
		return err
	}
	if payload.JobID == 0 {
		return nil
	}
	if err := c.ReconciliationService.Execute(ctx, payload.JobID); err != nil {
		logger.Warnw("worker_reconciliation_run_failed",
			"job_id", payload.JobID,
			"error", err,
		)
		return err
	}
	return nil
}
