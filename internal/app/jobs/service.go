package jobs

import (
	"context"
	"errors"

	jobconsumer "github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

// Service 异步队列服务
type Service struct {
	name      string
	server    *asynq.Server
	mux       *asynq.ServeMux
	scheduler *asynq.Scheduler
	consumer  *jobconsumer.Consumer
}

// NewService 创建异步队列服务
func NewService(cfg *config.QueueConfig, consumer *jobconsumer.Consumer) (*Service, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("queue disabled")
	}
	if consumer == nil {
		return nil, errors.New("consumer is nil")
	}
	opt, serverCfg := queue.BuildServerConfig(cfg)
	server := asynq.NewServer(opt, serverCfg)
	mux := asynq.NewServeMux()
	consumer.Register(mux)

	scheduler := asynq.NewScheduler(opt, nil)
	registerPeriodicTasks(scheduler, consumer, cfg)

	return &Service{
		name:      "worker",
		server:    server,
		mux:       mux,
		scheduler: scheduler,
		consumer:  consumer,
	}, nil
}

// registerPeriodicTasks 注册所有周期性任务
func registerPeriodicTasks(scheduler *asynq.Scheduler, consumer *jobconsumer.Consumer, cfg *config.QueueConfig) {
	if scheduler == nil || consumer == nil {
		return
	}
	if consumer.AffiliateService != nil {
		task := queue.NewAffiliateConfirmCommissionsTask()
		entryID, err := scheduler.Register("@every 1m", task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_affiliate_confirm_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_affiliate_confirm_ok", "entry_id", entryID)
		}
	}
	if consumer.MemberLevelService != nil {
		// 每小时清一次即可：折扣是否生效由 MembershipActive 实时判定，
		// 这个任务只负责把过期档位从库里清掉，不影响计价正确性。
		task := queue.NewMembershipExpireTask()
		entryID, err := scheduler.Register("@every 1h", task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_membership_expire_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_membership_expire_ok", "entry_id", entryID)
		}
	}
	if consumer.MemberLevelService != nil {
		// 到期前提醒同样每小时扫一次。窗口取满 7 天而不是 1 小时切片：
		// worker 停机数小时后，落在停机窗口里的会员仍会在下一轮被扫到，
		// 不会永久漏发；重复发送由抢占式幂等挡住。
		task := queue.NewMembershipExpiryReminderTask()
		entryID, err := scheduler.Register("@every 1h", task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_membership_reminder_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_membership_reminder_ok", "entry_id", entryID)
		}
	}
	if consumer.ResellerAccountingLedger != nil {
		task := queue.NewResellerConfirmLedgerTask()
		entryID, err := scheduler.Register("@every 1m", task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_reseller_confirm_ledger_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_reseller_confirm_ledger_ok", "entry_id", entryID)
		}
	}
	if consumer.ProductMappingService != nil {
		fallbackInterval := "5m"
		if cfg != nil && cfg.UpstreamSyncInterval != "" {
			fallbackInterval = cfg.UpstreamSyncInterval
		}
		// 优先读取后台动态设置；失败则回落到 config.yml 的兜底值
		syncInterval := fallbackInterval
		if consumer.SettingService != nil {
			d, err := consumer.SettingService.GetUpstreamSyncInterval(fallbackInterval)
			if err != nil {
				logger.Warnw("scheduler_load_upstream_sync_interval_failed", "error", err, "fallback", fallbackInterval)
			} else if d > 0 {
				syncInterval = settingsintegration.FormatUpstreamSyncIntervalForScheduler(d)
			}
		}
		task := queue.NewUpstreamSyncStockTask()
		entryID, err := scheduler.Register("@every "+syncInterval, task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_upstream_sync_stock_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_upstream_sync_stock_ok", "entry_id", entryID, "interval", syncInterval)
		}
	}
	if consumer.NotificationService != nil {
		task, err := queue.NewNotificationInventoryAlertCheckTask()
		if err != nil {
			logger.Warnw("scheduler_register_inventory_alert_check_failed", "error", err)
		} else {
			entryID, registerErr := scheduler.Register("@every 1m", task, asynq.Queue(queue.DefaultQueue))
			if registerErr != nil {
				logger.Warnw("scheduler_register_inventory_alert_check_failed", "error", registerErr)
			} else {
				logger.Infow("scheduler_register_inventory_alert_check_ok", "entry_id", entryID)
			}
		}
	}
	if consumer.ProcurementOrderService != nil {
		task := queue.NewProcurementSyncAcceptedTask()
		entryID, err := scheduler.Register("@every 30m", task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_procurement_sync_accepted_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_procurement_sync_accepted_ok", "entry_id", entryID)
		}
	}
}

// Name 服务名称
func (s *Service) Name() string {
	if s == nil || s.name == "" {
		return "worker"
	}
	return s.name
}

// Start 启动服务
func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.server == nil || s.mux == nil {
		return errors.New("worker not initialized")
	}
	if s.scheduler != nil {
		if err := s.scheduler.Start(); err != nil {
			logger.Warnw("scheduler_start_failed", "error", err)
		}
	}
	return s.server.Run(s.mux)
}

// Stop 停止服务
func (s *Service) Stop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	_ = ctx
	if s.scheduler != nil {
		s.scheduler.Shutdown()
	}
	s.server.Shutdown()
	return nil
}
