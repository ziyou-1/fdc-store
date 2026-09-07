package paymentcallbackhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

var errAlipayCallbackPaymentNotFound = errors.New("alipay callback payment not found")

func (h *Handler) handleAlipayCallback(c *gin.Context) bool {
	log := ginutil.RequestLog(c)
	form, err := parseCallbackForm(c)
	if err != nil {
		log.Warnw("alipay_callback_form_parse_failed", "error", err)
		return false
	}
	if !isAlipayCallbackForm(form) {
		log.Debugw("alipay_callback_not_matched")
		return false
	}
	log.Infow("alipay_callback_received",
		"client_ip", c.ClientIP(),
		"out_trade_no", strings.TrimSpace(getFirstValue(form, "out_trade_no")),
		"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")),
		"trade_status", strings.TrimSpace(getFirstValue(form, "trade_status")),
	)

	payment, channel, err := h.findAlipayCallbackPayment(form)
	if err != nil || payment == nil || channel == nil {
		log.Warnw("alipay_callback_payment_not_found",
			"out_trade_no", strings.TrimSpace(getFirstValue(form, "out_trade_no")),
			"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")),
			"error", err,
		)
		c.String(http.StatusOK, constants.AlipayCallbackFail)
		return true
	}

	updated, err := h.service.HandleSyncCallback(channel, form, nil)
	if err != nil {
		log.Warnw("alipay_callback_handle_failed", "payment_id", payment.ID, "channel_id", channel.ID, "error", err)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type": "alipay_callback_handle_failed", "alert_level": "error",
			"payment_id":   fmt.Sprintf("%d", payment.ID),
			"out_trade_no": strings.TrimSpace(getFirstValue(form, "out_trade_no")),
			"message":      strings.TrimSpace(err.Error()), "provider": constants.PaymentChannelTypeAlipay,
		})
		c.String(http.StatusOK, constants.AlipayCallbackFail)
		return true
	}
	log.Infow("alipay_callback_processed", "payment_id", payment.ID, "channel_id", channel.ID,
		"out_trade_no", strings.TrimSpace(getFirstValue(form, "out_trade_no")),
		"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")), "status", updated.Status)
	c.String(http.StatusOK, constants.AlipayCallbackSuccess)
	return true
}

func (h *Handler) handleEpayCallback(c *gin.Context) bool {
	log := ginutil.RequestLog(c)
	form, err := parseCallbackForm(c)
	if err != nil {
		log.Warnw("epay_callback_form_parse_failed", "error", err)
		return false
	}
	outTradeNo := strings.TrimSpace(getFirstValue(form, "out_trade_no"))
	pid := strings.TrimSpace(getFirstValue(form, "pid"))
	if pid == "" || outTradeNo == "" {
		log.Debugw("epay_callback_not_matched", "reason", "missing_pid_or_out_trade_no")
		return false
	}
	if strings.TrimSpace(getFirstValue(form, "trade_status")) == "" {
		log.Debugw("epay_callback_not_matched", "reason", "missing_trade_status")
		return false
	}
	log.Infow("epay_callback_received", "client_ip", c.ClientIP(), "out_trade_no", outTradeNo,
		"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")),
		"trade_status", strings.TrimSpace(getFirstValue(form, "trade_status")))

	payment, err := h.payments.GetByGatewayOrderNo(outTradeNo)
	if err != nil || payment == nil {
		log.Warnw("epay_callback_payment_not_found", "out_trade_no", outTradeNo, "error", err)
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}
	channel, err := h.channels.GetByID(payment.ChannelID)
	if err != nil || channel == nil {
		log.Warnw("epay_callback_channel_not_found", "payment_id", payment.ID, "channel_id", payment.ChannelID, "error", err)
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}
	if strings.ToLower(strings.TrimSpace(channel.ProviderType)) != constants.PaymentProviderEpay {
		log.Warnw("epay_callback_provider_invalid", "payment_id", payment.ID, "channel_id", channel.ID, "provider_type", channel.ProviderType)
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}

	updated, err := h.service.HandleSyncCallback(channel, form, nil)
	if err != nil {
		log.Warnw("epay_callback_handle_failed", "payment_id", payment.ID, "channel_id", channel.ID, "out_trade_no", outTradeNo, "error", err)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type": "epay_callback_handle_failed", "alert_level": "error",
			"payment_id": fmt.Sprintf("%d", payment.ID), "out_trade_no": outTradeNo,
			"message": strings.TrimSpace(err.Error()), "provider": constants.PaymentProviderEpay,
		})
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}
	log.Infow("epay_callback_processed", "payment_id", payment.ID, "channel_id", channel.ID, "out_trade_no", outTradeNo, "status", updated.Status)
	c.String(http.StatusOK, constants.EpayCallbackSuccess)
	return true
}

func parseCallbackForm(c *gin.Context) (map[string][]string, error) {
	normalizeCallbackRequestQuery(c.Request)
	if err := c.Request.ParseForm(); err != nil {
		return nil, err
	}
	if len(c.Request.PostForm) > 0 {
		return c.Request.PostForm, nil
	}
	return c.Request.Form, nil
}

func normalizeCallbackRequestQuery(r *http.Request) {
	if r == nil || r.URL == nil || r.URL.RawQuery == "" {
		return
	}
	raw := r.URL.RawQuery
	normalized := strings.ReplaceAll(strings.ReplaceAll(raw, "&amp;", "&"), ";", "&")
	if normalized != raw {
		r.URL.RawQuery = normalized
	}
}

func isAlipayCallbackForm(form map[string][]string) bool {
	if strings.TrimSpace(getFirstValue(form, "sign")) == "" {
		return false
	}
	hasNotifyField := strings.TrimSpace(getFirstValue(form, "notify_id")) != "" ||
		strings.TrimSpace(getFirstValue(form, "notify_type")) != "" ||
		strings.TrimSpace(getFirstValue(form, "buyer_id")) != ""
	return hasNotifyField && (strings.TrimSpace(getFirstValue(form, "out_trade_no")) != "" ||
		strings.TrimSpace(getFirstValue(form, "trade_no")) != "")
}

func (h *Handler) findAlipayCallbackPayment(form map[string][]string) (*paymentdomain.Payment, *paymentdomain.PaymentChannel, error) {
	for _, candidate := range []struct {
		value string
		find  func(string) (*paymentdomain.Payment, error)
	}{
		{strings.TrimSpace(getFirstValue(form, "out_trade_no")), h.payments.GetByGatewayOrderNo},
		{strings.TrimSpace(getFirstValue(form, "trade_no")), h.payments.GetLatestByProviderRef},
	} {
		if candidate.value == "" {
			continue
		}
		payment, err := candidate.find(candidate.value)
		if err != nil || payment == nil {
			continue
		}
		channel, err := h.channels.GetByID(payment.ChannelID)
		if err == nil && channel != nil &&
			strings.EqualFold(strings.TrimSpace(channel.ProviderType), constants.PaymentProviderOfficial) &&
			strings.EqualFold(strings.TrimSpace(channel.ChannelType), constants.PaymentChannelTypeAlipay) {
			return payment, channel, nil
		}
	}
	return nil, nil, errAlipayCallbackPaymentNotFound
}

// handlePayssionCallback 识别并处理 Payssion 的异步通知。
//
// Payssion 既可能发 form POST 也可能发 JSON body（官方示例按 Content-Type 分支），
// 两种都要接住：只解析 form 会让 JSON 通知全部落到"无法识别"分支并返回 404，
// 网关会当成失败反复重投。
func (h *Handler) handlePayssionCallback(c *gin.Context) bool {
	log := ginutil.RequestLog(c)

	form, err := parseCallbackForm(c)
	if err != nil {
		log.Warnw("payssion_callback_form_parse_failed", "error", err)
		form = map[string][]string{}
	}
	var body []byte
	if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "json") && c.Request.Body != nil {
		body, _ = io.ReadAll(c.Request.Body)
	}

	notifySig := strings.TrimSpace(getFirstValue(form, "notify_sig"))
	pmID := strings.TrimSpace(getFirstValue(form, "pm_id"))
	businessNo := strings.TrimSpace(getFirstValue(form, "order_id"))
	if businessNo == "" {
		businessNo = strings.TrimSpace(getFirstValue(form, "track_id"))
	}
	// JSON 投递时上面几个字段都在 body 里，这里补一次识别。
	if len(body) > 0 && (notifySig == "" || pmID == "" || businessNo == "") {
		decoded := map[string]interface{}{}
		if json.Unmarshal(body, &decoded) == nil {
			if notifySig == "" {
				notifySig = strings.TrimSpace(fmt.Sprintf("%v", decoded["notify_sig"]))
			}
			if pmID == "" {
				pmID = strings.TrimSpace(fmt.Sprintf("%v", decoded["pm_id"]))
			}
			if businessNo == "" {
				if value, ok := decoded["order_id"]; ok {
					businessNo = strings.TrimSpace(fmt.Sprintf("%v", value))
				} else if value, ok := decoded["track_id"]; ok {
					businessNo = strings.TrimSpace(fmt.Sprintf("%v", value))
				}
			}
		}
	}

	// 识别条件：notify_sig + pm_id 是 Payssion 独有的组合。
	if notifySig == "" || notifySig == "<nil>" || pmID == "" || pmID == "<nil>" || businessNo == "" {
		log.Debugw("payssion_callback_not_matched", "reason", "missing_notify_sig_or_pm_id_or_order_id")
		return false
	}
	log.Infow("payssion_callback_received", "client_ip", c.ClientIP(), "order_no", businessNo, "pm_id", pmID)

	payment, err := h.payments.GetByGatewayOrderNo(businessNo)
	if err != nil || payment == nil {
		log.Warnw("payssion_callback_payment_not_found", "order_no", businessNo, "error", err)
		c.AbortWithStatus(http.StatusNotFound)
		return true
	}
	channel, err := h.channels.GetByID(payment.ChannelID)
	if err != nil || channel == nil {
		log.Warnw("payssion_callback_channel_not_found", "payment_id", payment.ID, "channel_id", payment.ChannelID, "error", err)
		c.AbortWithStatus(http.StatusNotFound)
		return true
	}
	if strings.ToLower(strings.TrimSpace(channel.ProviderType)) != constants.PaymentProviderPayssion {
		log.Warnw("payssion_callback_provider_invalid", "payment_id", payment.ID, "provider_type", channel.ProviderType)
		return false
	}

	if _, err := h.service.HandleSyncCallback(channel, form, body); err != nil {
		log.Warnw("payssion_callback_handle_failed", "payment_id", payment.ID, "order_no", businessNo, "error", err)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type": "payssion_callback_handle_failed", "alert_level": "error",
			"payment_id": fmt.Sprintf("%d", payment.ID), "out_trade_no": businessNo,
			"message": strings.TrimSpace(err.Error()), "provider": constants.PaymentProviderPayssion,
		})
		// 验签失败按官方 WHMCS 插件的做法回 406，网关据此判定"我方拒绝"，
		// 与"处理失败请重投"区分开。
		c.String(http.StatusNotAcceptable, "invalid notify_sig")
		return true
	}
	// Payssion 只要求返回 HTTP 200 表示已收妥。
	c.String(http.StatusOK, "OK")
	return true
}
