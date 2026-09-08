package settingsmessaging

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrOrderEmailTemplateConfigInvalid = errors.New("order email template config invalid")

// OrderEmailLocalizedTemplate 订单邮件单语言模板
type OrderEmailLocalizedTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// OrderEmailSceneTemplate 订单邮件场景模板，按 locale 索引。
//
// 这里用 map 而不是固定的 ZHCN/ZHTW/ENUS 三个字段：固定字段意味着每加一门语言
// 都要改结构体、改 Resolve、改前端，漏掉任何一处都不会编译报错——加印尼语时就
// 正好漏了，印尼买家收到的订单邮件一直是中文。改成 map 后语言集合只由
// constants.SupportedLocales 决定。
type OrderEmailSceneTemplate map[string]OrderEmailLocalizedTemplate

// OrderEmailLocalizedText 按 locale 索引的纯文本（提示语一类）。
type OrderEmailLocalizedText map[string]string

// OrderEmailTemplatesSetting 所有订单邮件场景模板集合。
// 场景维度保持具名字段：它对应订单状态，是固定的领域概念，不会随语言增长。
type OrderEmailTemplatesSetting struct {
	Default              OrderEmailSceneTemplate `json:"default"`
	Paid                 OrderEmailSceneTemplate `json:"paid"`
	Delivered            OrderEmailSceneTemplate `json:"delivered"`
	DeliveredWithContent OrderEmailSceneTemplate `json:"delivered_with_content"`
	Refunded             OrderEmailSceneTemplate `json:"refunded"`
	PartiallyRefunded    OrderEmailSceneTemplate `json:"partially_refunded"`
}

// OrderEmailTemplateSetting 订单邮件模板配置
type OrderEmailTemplateSetting struct {
	Templates                OrderEmailTemplatesSetting `json:"templates"`
	GuestTip                 OrderEmailLocalizedText    `json:"guest_tip"`
	FulfillmentAttachmentTip OrderEmailLocalizedText    `json:"fulfillment_attachment_tip"`
}

// --- Patch 结构 ---

// OrderEmailLocalizedTemplatePatch 单语言模板补丁
type OrderEmailLocalizedTemplatePatch struct {
	Subject *string `json:"subject"`
	Body    *string `json:"body"`
}

// OrderEmailSceneTemplatePatch 场景模板补丁，按 locale 索引。
type OrderEmailSceneTemplatePatch map[string]OrderEmailLocalizedTemplatePatch

// OrderEmailLocalizedTextPatch 纯文本补丁，按 locale 索引。
type OrderEmailLocalizedTextPatch map[string]*string

// OrderEmailTemplatesPatch 模板集合补丁
type OrderEmailTemplatesPatch struct {
	Default              OrderEmailSceneTemplatePatch `json:"default"`
	Paid                 OrderEmailSceneTemplatePatch `json:"paid"`
	Delivered            OrderEmailSceneTemplatePatch `json:"delivered"`
	DeliveredWithContent OrderEmailSceneTemplatePatch `json:"delivered_with_content"`
	Refunded             OrderEmailSceneTemplatePatch `json:"refunded"`
	PartiallyRefunded    OrderEmailSceneTemplatePatch `json:"partially_refunded"`
}

// OrderEmailTemplateSettingPatch 订单邮件模板配置补丁
type OrderEmailTemplateSettingPatch struct {
	Templates                *OrderEmailTemplatesPatch    `json:"templates"`
	GuestTip                 OrderEmailLocalizedTextPatch `json:"guest_tip"`
	FulfillmentAttachmentTip OrderEmailLocalizedTextPatch `json:"fulfillment_attachment_tip"`
}

// OrderEmailScenes 返回全部场景键，顺序即后台展示顺序。
func OrderEmailScenes() []string {
	return []string{"default", "paid", "delivered", "delivered_with_content", "refunded", "partially_refunded"}
}

func isSupportedOrderEmailLocale(locale string) bool {
	for _, l := range constants.SupportedLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// --- 默认值 ---

// orderEmailCopy 一门语言的全部默认文案。
type orderEmailCopy struct {
	subject string
	bodies  map[string]string // scene -> body
	guest   string
	attach  string
}

func orderEmailDefaultCopy() map[string]orderEmailCopy {
	return map[string]orderEmailCopy{
		constants.LocaleZhCN: {
			subject: "订单状态更新：{{status}}",
			bodies: map[string]string{
				"default":                "订单号：{{order_no}}\n状态：{{status}}\n金额：{{amount}} {{currency}}\n\n感谢您的购买。\n\n{{site_name}} 的网址：{{site_url}}",
				"paid":                   "订单号：{{order_no}}\n状态：{{status}}\n金额：{{amount}} {{currency}}\n\n我们已收到您的付款，将尽快完成交付。\n\n{{site_name}} 的网址：{{site_url}}",
				"delivered":              "订单号：{{order_no}}\n状态：{{status}}\n金额：{{amount}} {{currency}}\n\n交付已完成，感谢您的购买。\n\n{{site_name}} 的网址：{{site_url}}",
				"delivered_with_content": "订单号：{{order_no}}\n状态：{{status}}\n金额：{{amount}} {{currency}}\n\n交付内容：\n{{fulfillment_info}}\n\n使用说明：\n{{instructions}}\n\n感谢您的购买。\n\n{{site_name}} 的网址：{{site_url}}",
				"refunded":               "订单号：{{order_no}}\n状态：{{status}}\n退款金额：{{refund_amount}} {{currency}}\n退款原因：{{refund_reason}}\n\n订单已退款，如有疑问请联系管理员。\n\n{{site_name}} 的网址：{{site_url}}",
				"partially_refunded":     "订单号：{{order_no}}\n状态：{{status}}\n退款金额：{{refund_amount}} {{currency}}\n退款原因：{{refund_reason}}\n\n订单已部分退款，如有疑问请联系管理员。\n\n{{site_name}} 的网址：{{site_url}}",
			},
			guest:  "游客订单可使用下单邮箱与订单密码在网站查询订单详情。",
			attach: "交付内容较多，已作为附件发送，请查看邮件附件获取完整交付内容。",
		},
		constants.LocaleZhTW: {
			subject: "訂單狀態更新：{{status}}",
			bodies: map[string]string{
				"default":                "訂單號：{{order_no}}\n狀態：{{status}}\n金額：{{amount}} {{currency}}\n\n感謝您的購買。\n\n{{site_name}} 的網址：{{site_url}}",
				"paid":                   "訂單號：{{order_no}}\n狀態：{{status}}\n金額：{{amount}} {{currency}}\n\n已收到付款，將盡快完成交付。\n\n{{site_name}} 的網址：{{site_url}}",
				"delivered":              "訂單號：{{order_no}}\n狀態：{{status}}\n金額：{{amount}} {{currency}}\n\n交付已完成，感謝您的購買。\n\n{{site_name}} 的網址：{{site_url}}",
				"delivered_with_content": "訂單號：{{order_no}}\n狀態：{{status}}\n金額：{{amount}} {{currency}}\n\n交付內容：\n{{fulfillment_info}}\n\n使用說明：\n{{instructions}}\n\n感謝您的購買。\n\n{{site_name}} 的網址：{{site_url}}",
				"refunded":               "訂單號：{{order_no}}\n狀態：{{status}}\n退款金額：{{refund_amount}} {{currency}}\n退款原因：{{refund_reason}}\n\n訂單已退款，如有疑問請聯絡管理員。\n\n{{site_name}} 的網址：{{site_url}}",
				"partially_refunded":     "訂單號：{{order_no}}\n狀態：{{status}}\n退款金額：{{refund_amount}} {{currency}}\n退款原因：{{refund_reason}}\n\n訂單已部分退款，如有疑問請聯絡管理員。\n\n{{site_name}} 的網址：{{site_url}}",
			},
			guest:  "遊客訂單可使用下單信箱與訂單密碼在網站查詢訂單詳情。",
			attach: "交付內容較多，已作為附件發送，請查看郵件附件獲取完整交付內容。",
		},
		constants.LocaleEnUS: {
			subject: "Order status updated: {{status}}",
			bodies: map[string]string{
				"default":                "Order No: {{order_no}}\nStatus: {{status}}\nAmount: {{amount}} {{currency}}\n\nThank you for your purchase.\n\n{{site_name}}'s Site URL: {{site_url}}",
				"paid":                   "Order No: {{order_no}}\nStatus: {{status}}\nAmount: {{amount}} {{currency}}\n\nWe have received your payment and will deliver soon.\n\n{{site_name}}'s Site URL: {{site_url}}",
				"delivered":              "Order No: {{order_no}}\nStatus: {{status}}\nAmount: {{amount}} {{currency}}\n\nDelivery completed. Thank you for your purchase.\n\n{{site_name}}'s Site URL: {{site_url}}",
				"delivered_with_content": "Order No: {{order_no}}\nStatus: {{status}}\nAmount: {{amount}} {{currency}}\n\nDelivery content:\n{{fulfillment_info}}\n\nUsage instructions:\n{{instructions}}\n\nThank you for your purchase.\n\n{{site_name}}'s Site URL: {{site_url}}",
				"refunded":               "Order No: {{order_no}}\nStatus: {{status}}\nRefund Amount: {{refund_amount}} {{currency}}\nReason for refund: {{refund_reason}}\n\nThe order has been refunded. Please contact admin if needed.\n\n{{site_name}}'s Site URL: {{site_url}}",
				"partially_refunded":     "Order No: {{order_no}}\nStatus: {{status}}\nRefund Amount: {{refund_amount}} {{currency}}\nReason for refund: {{refund_reason}}\n\nThe order has been partially refunded. Please contact admin if needed.\n\n{{site_name}}'s Site URL: {{site_url}}",
			},
			guest:  "Guest orders can be queried on the site using the checkout email and order password.",
			attach: "The delivery content is included as an attachment. Please check the email attachment for the full content.",
		},
		constants.LocaleIdID: {
			subject: "Status pesanan diperbarui: {{status}}",
			bodies: map[string]string{
				"default":                "No. pesanan: {{order_no}}\nStatus: {{status}}\nJumlah: {{amount}} {{currency}}\n\nTerima kasih atas pembelian kamu.\n\nSitus {{site_name}}: {{site_url}}",
				"paid":                   "No. pesanan: {{order_no}}\nStatus: {{status}}\nJumlah: {{amount}} {{currency}}\n\nPembayaran kamu sudah kami terima dan pesanan akan segera dikirim.\n\nSitus {{site_name}}: {{site_url}}",
				"delivered":              "No. pesanan: {{order_no}}\nStatus: {{status}}\nJumlah: {{amount}} {{currency}}\n\nPesanan sudah dikirim. Terima kasih atas pembelian kamu.\n\nSitus {{site_name}}: {{site_url}}",
				"delivered_with_content": "No. pesanan: {{order_no}}\nStatus: {{status}}\nJumlah: {{amount}} {{currency}}\n\nIsi pesanan:\n{{fulfillment_info}}\n\nCara pakai:\n{{instructions}}\n\nTerima kasih atas pembelian kamu.\n\nSitus {{site_name}}: {{site_url}}",
				"refunded":               "No. pesanan: {{order_no}}\nStatus: {{status}}\nJumlah pengembalian: {{refund_amount}} {{currency}}\nAlasan: {{refund_reason}}\n\nDana pesanan sudah dikembalikan. Hubungi admin bila ada pertanyaan.\n\nSitus {{site_name}}: {{site_url}}",
				"partially_refunded":     "No. pesanan: {{order_no}}\nStatus: {{status}}\nJumlah pengembalian: {{refund_amount}} {{currency}}\nAlasan: {{refund_reason}}\n\nDana pesanan dikembalikan sebagian. Hubungi admin bila ada pertanyaan.\n\nSitus {{site_name}}: {{site_url}}",
			},
			guest:  "Pesanan tanpa akun bisa dicek di situs memakai email pemesanan dan kata sandi pesanan.",
			attach: "Isi pesanan cukup panjang sehingga dikirim sebagai lampiran. Silakan buka lampiran email untuk melihat isi lengkapnya.",
		},
	}
}

func defaultSceneTemplate(copies map[string]orderEmailCopy, scene string) OrderEmailSceneTemplate {
	out := OrderEmailSceneTemplate{}
	for _, locale := range constants.SupportedLocales {
		c, ok := copies[locale]
		if !ok {
			// 新增语言但未补默认文案时回落简体，至少不会发出空邮件。
			c = copies[constants.LocaleZhCN]
		}
		out[locale] = OrderEmailLocalizedTemplate{Subject: c.subject, Body: c.bodies[scene]}
	}
	return out
}

func defaultLocalizedText(copies map[string]orderEmailCopy, pick func(orderEmailCopy) string) OrderEmailLocalizedText {
	out := OrderEmailLocalizedText{}
	for _, locale := range constants.SupportedLocales {
		c, ok := copies[locale]
		if !ok {
			c = copies[constants.LocaleZhCN]
		}
		out[locale] = pick(c)
	}
	return out
}

// DefaultOrderEmailTemplateSetting 返回默认订单邮件模板。
func DefaultOrderEmailTemplateSetting() OrderEmailTemplateSetting {
	copies := orderEmailDefaultCopy()
	return OrderEmailTemplateSetting{
		Templates: OrderEmailTemplatesSetting{
			Default:              defaultSceneTemplate(copies, "default"),
			Paid:                 defaultSceneTemplate(copies, "paid"),
			Delivered:            defaultSceneTemplate(copies, "delivered"),
			DeliveredWithContent: defaultSceneTemplate(copies, "delivered_with_content"),
			Refunded:             defaultSceneTemplate(copies, "refunded"),
			PartiallyRefunded:    defaultSceneTemplate(copies, "partially_refunded"),
		},
		GuestTip:                 defaultLocalizedText(copies, func(c orderEmailCopy) string { return c.guest }),
		FulfillmentAttachmentTip: defaultLocalizedText(copies, func(c orderEmailCopy) string { return c.attach }),
	}
}

// --- Normalize / Validate ---

func (s *OrderEmailTemplatesSetting) each(fn func(scene string, tmpl OrderEmailSceneTemplate) OrderEmailSceneTemplate) {
	s.Default = fn("default", s.Default)
	s.Paid = fn("paid", s.Paid)
	s.Delivered = fn("delivered", s.Delivered)
	s.DeliveredWithContent = fn("delivered_with_content", s.DeliveredWithContent)
	s.Refunded = fn("refunded", s.Refunded)
	s.PartiallyRefunded = fn("partially_refunded", s.PartiallyRefunded)
}

// NormalizeOrderEmailTemplateSetting 归一化：裁剪空白、补齐缺失语言为默认值、剔除未知语言。
func NormalizeOrderEmailTemplateSetting(setting OrderEmailTemplateSetting) OrderEmailTemplateSetting {
	def := DefaultOrderEmailTemplateSetting()
	defTemplates := map[string]OrderEmailSceneTemplate{
		"default": def.Templates.Default, "paid": def.Templates.Paid,
		"delivered": def.Templates.Delivered, "delivered_with_content": def.Templates.DeliveredWithContent,
		"refunded": def.Templates.Refunded, "partially_refunded": def.Templates.PartiallyRefunded,
	}
	setting.Templates.each(func(scene string, tmpl OrderEmailSceneTemplate) OrderEmailSceneTemplate {
		return normalizeOrderEmailSceneTemplate(tmpl, defTemplates[scene])
	})
	setting.GuestTip = normalizeOrderEmailText(setting.GuestTip, def.GuestTip)
	setting.FulfillmentAttachmentTip = normalizeOrderEmailText(setting.FulfillmentAttachmentTip, def.FulfillmentAttachmentTip)
	return setting
}

func normalizeOrderEmailSceneTemplate(t, fallback OrderEmailSceneTemplate) OrderEmailSceneTemplate {
	out := OrderEmailSceneTemplate{}
	for _, locale := range constants.SupportedLocales {
		lt := t[locale]
		lt.Subject = strings.TrimSpace(lt.Subject)
		lt.Body = strings.TrimSpace(lt.Body)
		// 任一字段留空即回落默认，避免发出没有标题或没有正文的邮件。
		if lt.Subject == "" {
			lt.Subject = fallback[locale].Subject
		}
		if lt.Body == "" {
			lt.Body = fallback[locale].Body
		}
		out[locale] = lt
	}
	return out
}

func normalizeOrderEmailText(t, fallback OrderEmailLocalizedText) OrderEmailLocalizedText {
	out := OrderEmailLocalizedText{}
	for _, locale := range constants.SupportedLocales {
		v := strings.TrimSpace(t[locale])
		if v == "" {
			v = fallback[locale]
		}
		out[locale] = v
	}
	return out
}

// ValidateOrderEmailTemplateSetting 校验：每个场景每种语言的标题与正文都不得为空。
func ValidateOrderEmailTemplateSetting(setting OrderEmailTemplateSetting) error {
	var invalid error
	setting.Templates.each(func(_ string, tmpl OrderEmailSceneTemplate) OrderEmailSceneTemplate {
		for _, locale := range constants.SupportedLocales {
			lt := tmpl[locale]
			if strings.TrimSpace(lt.Subject) == "" || strings.TrimSpace(lt.Body) == "" {
				invalid = ErrOrderEmailTemplateConfigInvalid
			}
		}
		return tmpl
	})
	return invalid
}

// --- Encode / Decode ---

func orderEmailSceneTemplateToMap(t OrderEmailSceneTemplate) map[string]interface{} {
	out := map[string]interface{}{}
	for locale, lt := range t {
		out[locale] = map[string]interface{}{"subject": lt.Subject, "body": lt.Body}
	}
	return out
}

func orderEmailTextToMap(t OrderEmailLocalizedText) map[string]interface{} {
	out := map[string]interface{}{}
	for locale, v := range t {
		out[locale] = v
	}
	return out
}

// EncodeOrderEmailTemplateSetting 序列化为 settings 表结构。
func EncodeOrderEmailTemplateSetting(setting OrderEmailTemplateSetting) jsonmap.JSON {
	normalized := NormalizeOrderEmailTemplateSetting(setting)
	return jsonmap.JSON{
		"templates": map[string]interface{}{
			"default":                orderEmailSceneTemplateToMap(normalized.Templates.Default),
			"paid":                   orderEmailSceneTemplateToMap(normalized.Templates.Paid),
			"delivered":              orderEmailSceneTemplateToMap(normalized.Templates.Delivered),
			"delivered_with_content": orderEmailSceneTemplateToMap(normalized.Templates.DeliveredWithContent),
			"refunded":               orderEmailSceneTemplateToMap(normalized.Templates.Refunded),
			"partially_refunded":     orderEmailSceneTemplateToMap(normalized.Templates.PartiallyRefunded),
		},
		"guest_tip":                  orderEmailTextToMap(normalized.GuestTip),
		"fulfillment_attachment_tip": orderEmailTextToMap(normalized.FulfillmentAttachmentTip),
	}
}

// MaskOrderEmailTemplateSettingForAdmin 返回管理端可用配置（无敏感字段）
func MaskOrderEmailTemplateSettingForAdmin(setting OrderEmailTemplateSetting) jsonmap.JSON {
	out := EncodeOrderEmailTemplateSetting(setting)
	// 附带语言清单，前端据此渲染语言页签，不必自己再写死一份。
	out["locales"] = append([]string(nil), constants.SupportedLocales...)
	out["scenes"] = OrderEmailScenes()
	return out
}

func decodeOrderEmailSceneTemplate(raw interface{}, fallback OrderEmailSceneTemplate) OrderEmailSceneTemplate {
	out := OrderEmailSceneTemplate{}
	for locale, lt := range fallback {
		out[locale] = lt
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return out
	}
	for locale, rawTmpl := range m {
		if !isSupportedOrderEmailLocale(locale) {
			continue
		}
		tm, ok := rawTmpl.(map[string]interface{})
		if !ok {
			continue
		}
		lt := out[locale]
		if s, ok := tm["subject"].(string); ok {
			lt.Subject = s
		}
		if b, ok := tm["body"].(string); ok {
			lt.Body = b
		}
		out[locale] = lt
	}
	return out
}

func decodeOrderEmailText(raw interface{}, fallback OrderEmailLocalizedText) OrderEmailLocalizedText {
	out := OrderEmailLocalizedText{}
	for locale, v := range fallback {
		out[locale] = v
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return out
	}
	for locale, v := range m {
		if !isSupportedOrderEmailLocale(locale) {
			continue
		}
		if s, ok := v.(string); ok {
			out[locale] = s
		}
	}
	return out
}

func DecodeOrderEmailTemplateSetting(raw jsonmap.JSON, fallback OrderEmailTemplateSetting) OrderEmailTemplateSetting {
	next := fallback
	if templates, ok := raw["templates"].(map[string]interface{}); ok {
		next.Templates.Default = decodeOrderEmailSceneTemplate(templates["default"], fallback.Templates.Default)
		next.Templates.Paid = decodeOrderEmailSceneTemplate(templates["paid"], fallback.Templates.Paid)
		next.Templates.Delivered = decodeOrderEmailSceneTemplate(templates["delivered"], fallback.Templates.Delivered)
		next.Templates.DeliveredWithContent = decodeOrderEmailSceneTemplate(templates["delivered_with_content"], fallback.Templates.DeliveredWithContent)
		next.Templates.Refunded = decodeOrderEmailSceneTemplate(templates["refunded"], fallback.Templates.Refunded)
		next.Templates.PartiallyRefunded = decodeOrderEmailSceneTemplate(templates["partially_refunded"], fallback.Templates.PartiallyRefunded)
	}
	next.GuestTip = decodeOrderEmailText(raw["guest_tip"], fallback.GuestTip)
	next.FulfillmentAttachmentTip = decodeOrderEmailText(raw["fulfillment_attachment_tip"], fallback.FulfillmentAttachmentTip)
	return next
}

// --- Patch ---

func applyOrderEmailSceneTemplatePatch(target *OrderEmailSceneTemplate, patch OrderEmailSceneTemplatePatch) error {
	if len(patch) == 0 {
		return nil
	}
	if *target == nil {
		*target = OrderEmailSceneTemplate{}
	}
	for locale, p := range patch {
		if !isSupportedOrderEmailLocale(locale) {
			return ErrOrderEmailTemplateConfigInvalid
		}
		lt := (*target)[locale]
		if p.Subject != nil {
			lt.Subject = strings.TrimSpace(*p.Subject)
		}
		if p.Body != nil {
			lt.Body = strings.TrimSpace(*p.Body)
		}
		(*target)[locale] = lt
	}
	return nil
}

func applyOrderEmailTextPatch(target *OrderEmailLocalizedText, patch OrderEmailLocalizedTextPatch) error {
	if len(patch) == 0 {
		return nil
	}
	if *target == nil {
		*target = OrderEmailLocalizedText{}
	}
	for locale, v := range patch {
		if !isSupportedOrderEmailLocale(locale) {
			return ErrOrderEmailTemplateConfigInvalid
		}
		if v != nil {
			(*target)[locale] = strings.TrimSpace(*v)
		}
	}
	return nil
}

// ApplyOrderEmailTemplateSettingPatch 应用补丁并完成校验。
func ApplyOrderEmailTemplateSettingPatch(current OrderEmailTemplateSetting, patch OrderEmailTemplateSettingPatch) (OrderEmailTemplateSetting, error) {
	next := NormalizeOrderEmailTemplateSetting(current)
	if patch.Templates != nil {
		pairs := []struct {
			target *OrderEmailSceneTemplate
			patch  OrderEmailSceneTemplatePatch
		}{
			{&next.Templates.Default, patch.Templates.Default},
			{&next.Templates.Paid, patch.Templates.Paid},
			{&next.Templates.Delivered, patch.Templates.Delivered},
			{&next.Templates.DeliveredWithContent, patch.Templates.DeliveredWithContent},
			{&next.Templates.Refunded, patch.Templates.Refunded},
			{&next.Templates.PartiallyRefunded, patch.Templates.PartiallyRefunded},
		}
		for _, p := range pairs {
			if err := applyOrderEmailSceneTemplatePatch(p.target, p.patch); err != nil {
				return OrderEmailTemplateSetting{}, err
			}
		}
	}
	if err := applyOrderEmailTextPatch(&next.GuestTip, patch.GuestTip); err != nil {
		return OrderEmailTemplateSetting{}, err
	}
	if err := applyOrderEmailTextPatch(&next.FulfillmentAttachmentTip, patch.FulfillmentAttachmentTip); err != nil {
		return OrderEmailTemplateSetting{}, err
	}
	normalized := NormalizeOrderEmailTemplateSetting(next)
	if err := ValidateOrderEmailTemplateSetting(normalized); err != nil {
		return OrderEmailTemplateSetting{}, err
	}
	return normalized, nil
}

// --- Locale 解析 ---

// ResolveOrderEmailLocaleTemplate 按 locale 选择模板，缺失时回落简体。
func ResolveOrderEmailLocaleTemplate(t OrderEmailSceneTemplate, locale string) OrderEmailLocalizedTemplate {
	if lt, ok := t[locale]; ok && strings.TrimSpace(lt.Body) != "" {
		return lt
	}
	return t[constants.LocaleZhCN]
}

func resolveOrderEmailText(t OrderEmailLocalizedText, locale string) string {
	if v, ok := t[locale]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return t[constants.LocaleZhCN]
}

// ResolveOrderEmailFulfillmentAttachmentTip 按 locale 选择交付内容附件提示
func ResolveOrderEmailFulfillmentAttachmentTip(tip OrderEmailLocalizedText, locale string) string {
	return resolveOrderEmailText(tip, locale)
}

// ResolveOrderEmailGuestTip 按 locale 选择游客提示
func ResolveOrderEmailGuestTip(tip OrderEmailLocalizedText, locale string) string {
	return resolveOrderEmailText(tip, locale)
}
