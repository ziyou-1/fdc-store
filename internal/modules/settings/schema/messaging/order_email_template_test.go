package settingsmessaging

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func allOrderScenes(s OrderEmailTemplateSetting) map[string]OrderEmailSceneTemplate {
	return map[string]OrderEmailSceneTemplate{
		"default": s.Templates.Default, "paid": s.Templates.Paid,
		"delivered": s.Templates.Delivered, "delivered_with_content": s.Templates.DeliveredWithContent,
		"refunded": s.Templates.Refunded, "partially_refunded": s.Templates.PartiallyRefunded,
	}
}

// 默认模板必须覆盖「全部场景 × 全部受支持语言」。
// 这条是守卫：以后新增语言而没补默认文案，这里会直接红——
// 加印尼语时正是漏了这一步，印尼买家收到的订单邮件一直是中文。
func TestDefaultOrderEmailTemplateCoversEverySceneAndLocale(t *testing.T) {
	def := DefaultOrderEmailTemplateSetting()
	for scene, tmpl := range allOrderScenes(def) {
		for _, locale := range constants.SupportedLocales {
			lt, ok := tmpl[locale]
			if !ok || strings.TrimSpace(lt.Subject) == "" || strings.TrimSpace(lt.Body) == "" {
				t.Fatalf("场景 %s 语言 %s 的默认模板缺失或为空", scene, locale)
			}
		}
	}
	for _, locale := range constants.SupportedLocales {
		if strings.TrimSpace(def.GuestTip[locale]) == "" {
			t.Fatalf("guest_tip 缺少语言 %s", locale)
		}
		if strings.TrimSpace(def.FulfillmentAttachmentTip[locale]) == "" {
			t.Fatalf("fulfillment_attachment_tip 缺少语言 %s", locale)
		}
	}
}

// 每种语言必须真的用本语言的文案，而不是静默回落中文。
func TestResolveOrderEmailLocaleTemplateReturnsOwnLanguage(t *testing.T) {
	def := DefaultOrderEmailTemplateSetting()
	markers := map[string]string{
		constants.LocaleZhCN: "订单号：",
		constants.LocaleZhTW: "訂單號：",
		constants.LocaleEnUS: "Order No:",
		constants.LocaleIdID: "No. pesanan:",
	}
	for _, locale := range constants.SupportedLocales {
		marker, ok := markers[locale]
		if !ok {
			t.Fatalf("新增了语言 %s 但没更新本测试", locale)
		}
		got := ResolveOrderEmailLocaleTemplate(def.Templates.Paid, locale)
		if !strings.Contains(got.Body, marker) {
			t.Fatalf("%s 落到了错误的语言分支，body=%s", locale, got.Body)
		}
	}
}

// 语言缺失时回落简体，而不是返回空模板。
func TestResolveOrderEmailLocaleTemplateFallsBackToZhCN(t *testing.T) {
	def := DefaultOrderEmailTemplateSetting()
	delete(def.Templates.Paid, constants.LocaleIdID)
	got := ResolveOrderEmailLocaleTemplate(def.Templates.Paid, constants.LocaleIdID)
	if !strings.Contains(got.Body, "订单号：") {
		t.Fatalf("语言缺失应回落简体，实得 %q", got.Body)
	}
}

// 存回去再读出来必须一致，且未改动的场景不被污染。
func TestOrderEmailTemplateEncodeDecodeRoundTrip(t *testing.T) {
	cur := DefaultOrderEmailTemplateSetting()
	body := "Pesanan {{order_no}} sudah dibayar."
	next, err := ApplyOrderEmailTemplateSettingPatch(cur, OrderEmailTemplateSettingPatch{
		Templates: &OrderEmailTemplatesPatch{
			Paid: OrderEmailSceneTemplatePatch{constants.LocaleIdID: {Body: &body}},
		},
	})
	if err != nil {
		t.Fatalf("补丁失败: %v", err)
	}
	decoded := NormalizeOrderEmailTemplateSetting(
		DecodeOrderEmailTemplateSetting(EncodeOrderEmailTemplateSetting(next), DefaultOrderEmailTemplateSetting()))
	if got := decoded.Templates.Paid[constants.LocaleIdID].Body; got != body {
		t.Fatalf("往返后正文 = %q，期望 %q", got, body)
	}
	if got := decoded.Templates.Refunded[constants.LocaleZhCN].Body; !strings.Contains(got, "订单已退款") {
		t.Fatalf("未改动的场景被污染: %q", got)
	}
}

// 未知语言的补丁必须被拒，避免把垃圾键写进设置表。
func TestOrderEmailTemplatePatchRejectsUnknownLocale(t *testing.T) {
	body := "x"
	_, err := ApplyOrderEmailTemplateSettingPatch(DefaultOrderEmailTemplateSetting(), OrderEmailTemplateSettingPatch{
		Templates: &OrderEmailTemplatesPatch{
			Paid: OrderEmailSceneTemplatePatch{"xx-YY": {Body: &body}},
		},
	})
	if err == nil {
		t.Fatal("未知语言的补丁应被拒")
	}
}

// 清空字段回落默认，不会发出没有标题或没有正文的邮件。
func TestOrderEmailTemplateBlankFieldFallsBackToDefault(t *testing.T) {
	blank := "   "
	next, err := ApplyOrderEmailTemplateSettingPatch(DefaultOrderEmailTemplateSetting(), OrderEmailTemplateSettingPatch{
		Templates: &OrderEmailTemplatesPatch{
			Paid: OrderEmailSceneTemplatePatch{constants.LocaleZhCN: {Subject: &blank, Body: &blank}},
		},
	})
	if err != nil {
		t.Fatalf("清空字段不该报错: %v", err)
	}
	got := next.Templates.Paid[constants.LocaleZhCN]
	if got.Subject == "" || got.Body == "" {
		t.Fatalf("清空后未回落默认: %+v", got)
	}
}
