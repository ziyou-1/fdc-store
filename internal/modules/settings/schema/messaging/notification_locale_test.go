package settingsmessaging

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func notificationAllScenes(s NotificationCenterSetting) map[string]NotificationSceneTemplate {
	return map[string]NotificationSceneTemplate{
		"wallet_recharge_success":    s.Templates.WalletRechargeSuccess,
		"order_paid_success":         s.Templates.OrderPaidSuccess,
		"manual_fulfillment_pending": s.Templates.ManualFulfillmentPending,
		"exception_alert":            s.Templates.ExceptionAlert,
	}
}

// 默认模板必须覆盖「全部场景 × 全部受支持语言」。
// 这条是守卫：以后新增语言而没补默认文案，这里会直接红。
func TestNotificationDefaultCoversEverySceneAndLocale(t *testing.T) {
	def := NotificationCenterDefaultSetting()
	for scene, tmpl := range notificationAllScenes(def) {
		for _, locale := range constants.SupportedLocales {
			lt, ok := tmpl[locale]
			if !ok || strings.TrimSpace(lt.Title) == "" || strings.TrimSpace(lt.Body) == "" {
				t.Fatalf("场景 %s 语言 %s 的默认模板缺失或为空", scene, locale)
			}
		}
	}
}

// 每种语言必须真的用本语言文案，而不是静默回落中文。
func TestNotificationResolveReturnsOwnLanguage(t *testing.T) {
	def := NotificationCenterDefaultSetting()
	markers := map[string]string{
		constants.LocaleZhCN: "购买人：",
		constants.LocaleZhTW: "購買人：",
		constants.LocaleEnUS: "Customer:",
		constants.LocaleIdID: "Pelanggan:",
	}
	for _, locale := range constants.SupportedLocales {
		marker, ok := markers[locale]
		if !ok {
			t.Fatalf("新增了语言 %s 但没更新本测试", locale)
		}
		got := def.Templates.OrderPaidSuccess.ResolveLocaleTemplate(locale)
		if !strings.Contains(got.Body, marker) {
			t.Fatalf("%s 落到了错误的语言分支，body=%s", locale, got.Body)
		}
	}
}

// 印尼语必须能被设为管理员通知语言（notificationSupportedLocales 白名单）。
func TestNotificationAcceptsIndonesianAsDefaultLocale(t *testing.T) {
	setting := NotificationCenterDefaultSetting()
	setting.DefaultLocale = constants.LocaleIdID
	if err := ValidateNotificationCenterSetting(setting); err != nil {
		t.Fatalf("印尼语应可作为管理员通知语言: %v", err)
	}
}

// 语言缺失时回落简体，而不是返回空模板。
func TestNotificationResolveFallsBackToZhCN(t *testing.T) {
	def := NotificationCenterDefaultSetting()
	delete(def.Templates.OrderPaidSuccess, constants.LocaleIdID)
	got := def.Templates.OrderPaidSuccess.ResolveLocaleTemplate(constants.LocaleIdID)
	if !strings.Contains(got.Body, "购买人：") {
		t.Fatalf("语言缺失应回落简体，实得 %q", got.Body)
	}
}

// 未知语言的补丁必须被拒。
func TestNotificationPatchRejectsUnknownLocale(t *testing.T) {
	title := "x"
	_, err := ApplyNotificationCenterSettingPatch(NotificationCenterDefaultSetting(), NotificationCenterSettingPatch{
		Templates: &NotificationTemplatesPatch{
			OrderPaidSuccess: NotificationSceneTemplatePatch{"xx-YY": {Title: &title}},
		},
	})
	if err == nil {
		t.Fatal("未知语言的补丁应被拒")
	}
}

// 编解码往返一致，且未改动的场景不被污染。
func TestNotificationEncodeDecodeRoundTrip(t *testing.T) {
	body := "Pesanan {{order_no}} sudah dibayar."
	next, err := ApplyNotificationCenterSettingPatch(NotificationCenterDefaultSetting(), NotificationCenterSettingPatch{
		Templates: &NotificationTemplatesPatch{
			OrderPaidSuccess: NotificationSceneTemplatePatch{constants.LocaleIdID: {Body: &body}},
		},
	})
	if err != nil {
		t.Fatalf("补丁失败: %v", err)
	}
	decoded := DecodeNotificationCenterSetting(
		NotificationCenterSettingToMap(next), NotificationCenterDefaultSetting())
	if got := decoded.Templates.OrderPaidSuccess[constants.LocaleIdID].Body; got != body {
		t.Fatalf("往返后正文 = %q，期望 %q", got, body)
	}
	if got := decoded.Templates.ExceptionAlert[constants.LocaleZhCN].Title; got != "系统异常告警" {
		t.Fatalf("未改动的场景被污染: %q", got)
	}
}
