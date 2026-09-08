package settingsmessaging

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

// 默认模板必须覆盖「全部场景 × 全部受支持语言」。
// 这条同时是守卫：以后新增语言而没补默认文案，这里会直接红。
func TestDefaultVerifyEmailTemplateCoversEverySceneAndLocale(t *testing.T) {
	def := DefaultVerifyEmailTemplateSetting()
	for _, scene := range VerifyEmailScenes() {
		sceneTmpl, ok := def.Templates[scene]
		if !ok {
			t.Fatalf("场景 %s 没有默认模板", scene)
		}
		for _, locale := range constants.SupportedLocales {
			tmpl, ok := sceneTmpl[locale]
			if !ok {
				t.Fatalf("场景 %s 缺少语言 %s", scene, locale)
			}
			if strings.TrimSpace(tmpl.Subject) == "" || strings.TrimSpace(tmpl.Body) == "" {
				t.Fatalf("场景 %s 语言 %s 的默认模板为空", scene, locale)
			}
			if !strings.Contains(tmpl.Body, "{{code}}") {
				t.Fatalf("场景 %s 语言 %s 的默认正文没有 {{code}}", scene, locale)
			}
		}
	}
}

// 默认文案必须与迁移前的硬编码逐字一致，否则「什么都没改」的站点会突然换文案。
func TestDefaultVerifyEmailTemplateMatchesLegacyCopy(t *testing.T) {
	def := DefaultVerifyEmailTemplateSetting()
	cases := []struct{ scene, locale, subject, bodyPart string }{
		{constants.VerifyPurposeRegister, constants.LocaleZhCN, "注册验证码", "该验证码用于 注册，请勿泄露。"},
		{constants.VerifyPurposeReset, constants.LocaleZhCN, "重置密码验证码", "该验证码用于 重置密码，请勿泄露。"},
		{constants.VerifyPurposeRegister, constants.LocaleZhTW, "註冊驗證碼", "該驗證碼用於 註冊，請勿洩露。"},
		{constants.VerifyPurposeRegister, constants.LocaleEnUS, "Registration Code", "This code is for registration."},
		{constants.VerifyPurposeRegister, constants.LocaleIdID, "Kode Verifikasi Pendaftaran", "Kode ini digunakan untuk pendaftaran."},
	}
	for _, c := range cases {
		got := def.Templates[c.scene][c.locale]
		if got.Subject != c.subject {
			t.Fatalf("%s/%s 标题 = %q，期望 %q", c.scene, c.locale, got.Subject, c.subject)
		}
		if !strings.Contains(got.Body, c.bodyPart) {
			t.Fatalf("%s/%s 正文缺少 %q，实得 %q", c.scene, c.locale, c.bodyPart, got.Body)
		}
	}
}

// 正文不带 {{code}} 等于发了一封没有验证码的邮件，必须被拒。
func TestVerifyEmailTemplateRejectsBodyWithoutCode(t *testing.T) {
	setting := DefaultVerifyEmailTemplateSetting()
	setting.Templates[constants.VerifyPurposeRegister][constants.LocaleZhCN] = VerifyEmailLocalizedTemplate{
		Subject: "注册验证码",
		Body:    "你好，请完成注册。",
	}
	if err := ValidateVerifyEmailTemplateSetting(setting); err == nil {
		t.Fatal("正文没有 {{code}} 时必须校验失败")
	}
}

// 未知场景 / 未知语言的补丁必须被拒，避免把垃圾键写进设置表。
func TestVerifyEmailTemplatePatchRejectsUnknownSceneOrLocale(t *testing.T) {
	cur := DefaultVerifyEmailTemplateSetting()
	body := "你的码 {{code}}"
	for _, p := range []VerifyEmailTemplateSettingPatch{
		{Templates: map[string]map[string]VerifyEmailLocalizedTemplatePatch{
			"no_such_scene": {constants.LocaleZhCN: {Body: &body}}}},
		{Templates: map[string]map[string]VerifyEmailLocalizedTemplatePatch{
			constants.VerifyPurposeRegister: {"xx-YY": {Body: &body}}}},
	} {
		if _, err := ApplyVerifyEmailTemplateSettingPatch(cur, p); err == nil {
			t.Fatalf("非法补丁应被拒: %+v", p.Templates)
		}
	}
}

// 清空某个字段应回落默认，而不是发出没有标题或没有正文的邮件。
func TestVerifyEmailTemplateBlankFieldFallsBackToDefault(t *testing.T) {
	cur := DefaultVerifyEmailTemplateSetting()
	blank := "   "
	next, err := ApplyVerifyEmailTemplateSettingPatch(cur, VerifyEmailTemplateSettingPatch{
		Templates: map[string]map[string]VerifyEmailLocalizedTemplatePatch{
			constants.VerifyPurposeRegister: {constants.LocaleZhCN: {Subject: &blank, Body: &blank}},
		},
	})
	if err != nil {
		t.Fatalf("清空字段不该报错: %v", err)
	}
	got := next.Templates[constants.VerifyPurposeRegister][constants.LocaleZhCN]
	if got.Subject != "注册验证码" || !strings.Contains(got.Body, "{{code}}") {
		t.Fatalf("清空后未回落默认，实得 %+v", got)
	}
}

// 存回去再读出来必须完全一致。
func TestVerifyEmailTemplateEncodeDecodeRoundTrip(t *testing.T) {
	cur := DefaultVerifyEmailTemplateSetting()
	custom := "【自定义】验证码 {{code}}，站点 {{site_name}}"
	next, err := ApplyVerifyEmailTemplateSettingPatch(cur, VerifyEmailTemplateSettingPatch{
		Templates: map[string]map[string]VerifyEmailLocalizedTemplatePatch{
			constants.VerifyPurposeRegister: {constants.LocaleIdID: {Body: &custom}},
		},
	})
	if err != nil {
		t.Fatalf("补丁失败: %v", err)
	}
	decoded := NormalizeVerifyEmailTemplateSetting(
		DecodeVerifyEmailTemplateSetting(EncodeVerifyEmailTemplateSetting(next), DefaultVerifyEmailTemplateSetting()))
	if got := decoded.Templates[constants.VerifyPurposeRegister][constants.LocaleIdID].Body; got != custom {
		t.Fatalf("往返后正文 = %q，期望 %q", got, custom)
	}
	// 没改过的场景必须还是默认值
	if got := decoded.Templates[constants.VerifyPurposeReset][constants.LocaleZhCN].Subject; got != "重置密码验证码" {
		t.Fatalf("未改动的场景被污染: %q", got)
	}
}

// 语言缺失时回落简体，场景未知时返回 false 交给调用方兜底。
func TestResolveVerifyEmailTemplateFallback(t *testing.T) {
	setting := DefaultVerifyEmailTemplateSetting()
	delete(setting.Templates[constants.VerifyPurposeRegister], constants.LocaleIdID)

	got, ok := ResolveVerifyEmailTemplate(setting, constants.VerifyPurposeRegister, constants.LocaleIdID)
	if !ok || got.Subject != "注册验证码" {
		t.Fatalf("语言缺失应回落简体，实得 ok=%v %+v", ok, got)
	}
	if _, ok := ResolveVerifyEmailTemplate(setting, "no_such_scene", constants.LocaleZhCN); ok {
		t.Fatal("未知场景应返回 false")
	}
}
