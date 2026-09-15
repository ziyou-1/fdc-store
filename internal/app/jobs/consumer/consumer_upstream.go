package consumer

import (
	"context"
	"encoding/json"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/queue"

	"github.com/dujiao-next/internal/app/container"

	"github.com/hibiken/asynq"
)

// handleUpstreamSyncStock 处理上游库存同步任务。
func (c *Consumer) handleUpstreamSyncStock(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ProductMappingService == nil {
		logger.Debugw("worker_upstream_sync_stock_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	cfg, _ := c.SettingService.GetUpstreamSyncConfig("5m")
	if err := c.ProductMappingService.SyncAllStock(cfg); err != nil {
		logger.Warnw("worker_upstream_sync_stock_failed", "error", err)
		return err
	}
	return nil
}

// handleProcurementSubmit 处理采购单提交上游任务。
func (c *Consumer) handleProcurementSubmit(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ProcurementOrderService == nil {
		logger.Debugw("worker_procurement_submit_skip_nil")
		return nil
	}
	var payload queue.ProcurementSubmitPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_procurement_submit_unmarshal_failed", "error", err)
		return err
	}
	if payload.ProcurementOrderID == 0 {
		return nil
	}
	if err := c.ProcurementOrderService.SubmitToUpstream(payload.ProcurementOrderID); err != nil {
		logger.Warnw("worker_procurement_submit_failed",
			"procurement_order_id", payload.ProcurementOrderID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleProcurementPollStatus 处理采购单轮询上游状态任务。
func (c *Consumer) handleProcurementPollStatus(_ context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ProcurementOrderService == nil {
		logger.Debugw("worker_procurement_poll_skip_nil")
		return nil
	}
	var payload queue.ProcurementPollStatusPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_procurement_poll_unmarshal_failed", "error", err)
		return err
	}
	if payload.ProcurementOrderID == 0 {
		return nil
	}
	if err := c.ProcurementOrderService.PollUpstreamStatus(payload.ProcurementOrderID); err != nil {
		logger.Warnw("worker_procurement_poll_failed",
			"procurement_order_id", payload.ProcurementOrderID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleProcurementSyncAccepted 处理 accepted 采购单的定时巡检任务。
func (c *Consumer) handleProcurementSyncAccepted(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ProcurementOrderService == nil {
		logger.Debugw("worker_procurement_sync_accepted_skip_nil")
		return nil
	}
	c.ProcurementOrderService.SyncAcceptedOrders()
	return nil
}

// handleDownstreamCallback 处理下游回调发送任务。
func (c *Consumer) handleDownstreamCallback(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.DownstreamCallbackService == nil {
		logger.Debugw("worker_downstream_callback_skip_nil")
		return nil
	}
	var payload queue.DownstreamCallbackPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_downstream_callback_unmarshal_failed", "error", err)
		return err
	}
	if payload.DownstreamOrderRefID == 0 {
		return nil
	}
	if err := c.DownstreamCallbackService.SendCallback(ctx, payload.DownstreamOrderRefID); err != nil {
		logger.Warnw("worker_downstream_callback_failed",
			"ref_id", payload.DownstreamOrderRefID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleAgisoSyncStock 刷新卡券库商品的库存缓存。
//
// 与上游库存同步共用一个周期设置：两者性质相同（都是把外部库存拉进本地展示），
// 多加一个独立开关只会让运营多记一个参数。
func (c *Consumer) handleAgisoSyncStock(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.Container == nil {
		logger.Debugw("worker_agiso_sync_stock_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	if err := (container.AgisoStockSyncer{}).WithContainer(c.Container).SyncAll(); err != nil {
		logger.Warnw("worker_agiso_sync_stock_failed", "error", err)
		return err
	}
	return nil
}
