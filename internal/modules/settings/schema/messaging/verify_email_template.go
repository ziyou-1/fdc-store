package settingsmessaging

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrVerifyEmailTemplateConfigInvalid = errors.New("verify email template config invalid")

// 单个模板字段的长度上限。验证码邮件是纯文本，正文再长也没有意义，
// 设个上限避免把整篇 HTML 塞进设置表。
const (
	verifyEmailSubjectMaxLen = 200
	verifyEmailBodyMaxLen    = 4000
)

// VerifyEmailLocalizedTemplate 验证码邮件的单语言模板。
type VerifyEmailLocalizedTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// VerifyEmailSceneTemplate 一个验证码场景的多语言模板，按 locale 索引。
//
// 这里刻意用 map 而不是像订单邮件模板那样写死 ZHCN/ZHTW/ENUS 三个字段：
// 固定字段意味着每加一门语言都要改结构体、改 Resolve、改前端，漏掉任何一处
// 都不会编译报错，只会让该语言的用户静默收到中文邮件——订单邮件模板就正好
// 踩了这个坑。用 map 之后语言集合只由 constants.SupportedLocales 决定。
type VerifyEmailSceneTemplate map[string]VerifyEmailLocalizedTemplate

// VerifyEmailTemplateSetting 验证码邮件模板配置，按场景索引。
type VerifyEmailTemplateSetting struct {
	Templates map[string]VerifyEmailSceneTemplate `json:"templates"`
}

// VerifyEmailLocalizedTemplatePatch 单语言模板补丁。
type VerifyEmailLocalizedTemplatePatch struct {
	Subject *string `json:"subject"`
	Body    *string `json:"body"`
}

// VerifyEmailTemplateSettingPatch 配置补丁：templates[场景][语言]。
type VerifyEmailTemplateSettingPatch struct {
	Templates map[string]map[string]VerifyEmailLocalizedTemplatePatch `json:"templates"`
}

// VerifyEmailScenes 返回全部验证码场景，顺序即后台展示顺序。
func VerifyEmailScenes() []string {
	return []string{
		constants.VerifyPurposeRegister,
		constants.VerifyPurposeReset,
		constants.VerifyPurposeTelegramBind,
		constants.VerifyPurposeChangeEmailOld,
		constants.VerifyPurposeChangeEmailNew,
	}
}

func isVerifyEmailScene(scene string) bool {
	for _, s := range VerifyEmailScenes() {
		if s == scene {
			return true
		}
	}
	return false
}

func isSupportedVerifyEmailLocale(locale string) bool {
	for _, l := range constants.SupportedLocales {
		if l == locale {
			return true
		}
	}
	return false
}

// 各语言的正文骨架与「用途」措辞。默认模板由它们拼出来，
// 保证开箱即用的文案与迁移前的硬编码逐字一致。
type verifyEmailLocaleCopy struct {
	body     string // 含 {{code}} 与 %PURPOSE% 占位
	subjects map[string]string
	purposes map[string]string
}

func verifyEmailDefaultCopy() map[string]verifyEmailLocaleCopy {
	return map[string]verifyEmailLocaleCopy{
		constants.LocaleZhCN: {
			body: "您的验证码是：{{code}}\n\n该验证码用于 %PURPOSE%，请勿泄露。",
			subjects: map[string]string{
				constants.VerifyPurposeRegister:       "注册验证码",
				constants.VerifyPurposeReset:          "重置密码验证码",
				constants.VerifyPurposeTelegramBind:   "Telegram 绑定验证码",
				constants.VerifyPurposeChangeEmailOld: "更换邮箱验证码",
				constants.VerifyPurposeChangeEmailNew: "更换邮箱验证码",
			},
			purposes: map[string]string{
				constants.VerifyPurposeRegister:       "注册",
				constants.VerifyPurposeReset:          "重置密码",
				constants.VerifyPurposeTelegramBind:   "绑定 Telegram",
				constants.VerifyPurposeChangeEmailOld: "更换邮箱",
				constants.VerifyPurposeChangeEmailNew: "更换邮箱",
			},
		},
		constants.LocaleZhTW: {
			body: "您的驗證碼是：{{code}}\n\n該驗證碼用於 %PURPOSE%，請勿洩露。",
			subjects: map[string]string{
				constants.VerifyPurposeRegister:       "註冊驗證碼",
				constants.VerifyPurposeReset:          "重置密碼驗證碼",
				constants.VerifyPurposeTelegramBind:   "Telegram 綁定驗證碼",
				constants.VerifyPurposeChangeEmailOld: "更換郵箱驗證碼",
				constants.VerifyPurposeChangeEmailNew: "更換郵箱驗證碼",
			},
			purposes: map[string]string{
				constants.VerifyPurposeRegister:       "註冊",
				constants.VerifyPurposeReset:          "重置密碼",
				constants.VerifyPurposeTelegramBind:   "綁定 Telegram",
				constants.VerifyPurposeChangeEmailOld: "更換郵箱",
				constants.VerifyPurposeChangeEmailNew: "更換郵箱",
			},
		},
		constants.LocaleEnUS: {
			body: "Your verification code is: {{code}}\n\nThis code is for %PURPOSE%. Do not share it.",
			subjects: map[string]string{
				constants.VerifyPurposeRegister:       "Registration Code",
				constants.VerifyPurposeReset:          "Password Reset Code",
				constants.VerifyPurposeTelegramBind:   "Telegram Binding Code",
				constants.VerifyPurposeChangeEmailOld: "Change Email Code",
				constants.VerifyPurposeChangeEmailNew: "Change Email Code",
			},
			purposes: map[string]string{
				constants.VerifyPurposeRegister:       "registration",
				constants.VerifyPurposeReset:          "password reset",
				constants.VerifyPurposeTelegramBind:   "binding Telegram",
				constants.VerifyPurposeChangeEmailOld: "change email",
				constants.VerifyPurposeChangeEmailNew: "change email",
			},
		},
		constants.LocaleIdID: {
			body: "Kode verifikasi kamu: {{code}}\n\nKode ini digunakan untuk %PURPOSE%. Jangan bagikan kepada siapa pun.",
			subjects: map[string]string{
				constants.VerifyPurposeRegister:       "Kode Verifikasi Pendaftaran",
				constants.VerifyPurposeReset:          "Kode Atur Ulang Kata Sandi",
				constants.VerifyPurposeTelegramBind:   "Kode Tautan Telegram",
				constants.VerifyPurposeChangeEmailOld: "Kode Ganti Email",
				constants.VerifyPurposeChangeEmailNew: "Kode Ganti Email",
			},
			purposes: map[string]string{
				constants.VerifyPurposeRegister:       "pendaftaran",
				constants.VerifyPurposeReset:          "atur ulang kata sandi",
				constants.VerifyPurposeTelegramBind:   "menautkan Telegram",
				constants.VerifyPurposeChangeEmailOld: "penggantian email",
				constants.VerifyPurposeChangeEmailNew: "penggantian email",
			},
		},
	}
}

// DefaultVerifyEmailTemplateSetting 返回默认模板，文案与迁移前的硬编码一致。
func DefaultVerifyEmailTemplateSetting() VerifyEmailTemplateSetting {
	copies := verifyEmailDefaultCopy()
	setting := VerifyEmailTemplateSetting{Templates: map[string]VerifyEmailSceneTemplate{}}
	for _, scene := range VerifyEmailScenes() {
		sceneTmpl := VerifyEmailSceneTemplate{}
		for _, locale := range constants.SupportedLocales {
			c, ok := copies[locale]
			if !ok {
				// 新增语言但没补默认文案时回落到简体，至少不会发出空邮件。
				c = copies[constants.LocaleZhCN]
			}
			sceneTmpl[locale] = VerifyEmailLocalizedTemplate{
				Subject: c.subjects[scene],
				Body:    strings.ReplaceAll(c.body, "%PURPOSE%", c.purposes[scene]),
			}
		}
		setting.Templates[scene] = sceneTmpl
	}
	return setting
}

// NormalizeVerifyEmailTemplateSetting 去掉未知场景/语言，补齐缺失项为默认值。
func NormalizeVerifyEmailTemplateSetting(setting VerifyEmailTemplateSetting) VerifyEmailTemplateSetting {
	def := DefaultVerifyEmailTemplateSetting()
	out := VerifyEmailTemplateSetting{Templates: map[string]VerifyEmailSceneTemplate{}}
	for _, scene := range VerifyEmailScenes() {
		sceneTmpl := VerifyEmailSceneTemplate{}
		incoming := setting.Templates[scene]
		for _, locale := range constants.SupportedLocales {
			t := incoming[locale]
			t.Subject = strings.TrimSpace(t.Subject)
			t.Body = strings.TrimSpace(t.Body)
			// 任一字段留空即回落默认，避免管理员清空后发出没有标题或没有正文的邮件。
			if t.Subject == "" {
				t.Subject = def.Templates[scene][locale].Subject
			}
			if t.Body == "" {
				t.Body = def.Templates[scene][locale].Body
			}
			sceneTmpl[locale] = t
		}
		out.Templates[scene] = sceneTmpl
	}
	return out
}

// ValidateVerifyEmailTemplateSetting 校验长度与必需占位符。
func ValidateVerifyEmailTemplateSetting(setting VerifyEmailTemplateSetting) error {
	for scene, sceneTmpl := range setting.Templates {
		if !isVerifyEmailScene(scene) {
			return ErrVerifyEmailTemplateConfigInvalid
		}
		for locale, t := range sceneTmpl {
			if !isSupportedVerifyEmailLocale(locale) {
				return ErrVerifyEmailTemplateConfigInvalid
			}
			if len([]rune(t.Subject)) > verifyEmailSubjectMaxLen ||
				len([]rune(t.Body)) > verifyEmailBodyMaxLen {
				return ErrVerifyEmailTemplateConfigInvalid
			}
			// 正文必须带 {{code}}，否则这封邮件等于没发验证码。
			if !strings.Contains(t.Body, "{{code}}") {
				return ErrVerifyEmailTemplateConfigInvalid
			}
		}
	}
	return nil
}

// EncodeVerifyEmailTemplateSetting 转为可入库的 JSON。
func EncodeVerifyEmailTemplateSetting(setting VerifyEmailTemplateSetting) jsonmap.JSON {
	templates := map[string]interface{}{}
	for scene, sceneTmpl := range setting.Templates {
		locales := map[string]interface{}{}
		for locale, t := range sceneTmpl {
			locales[locale] = map[string]interface{}{"subject": t.Subject, "body": t.Body}
		}
		templates[scene] = locales
	}
	return jsonmap.JSON{"templates": templates}
}

// DecodeVerifyEmailTemplateSetting 从库内 JSON 还原，缺失项用 fallback 补齐。
func DecodeVerifyEmailTemplateSetting(value map[string]interface{}, fallback VerifyEmailTemplateSetting) VerifyEmailTemplateSetting {
	out := VerifyEmailTemplateSetting{Templates: map[string]VerifyEmailSceneTemplate{}}
	for scene, sceneTmpl := range fallback.Templates {
		cloned := VerifyEmailSceneTemplate{}
		for locale, t := range sceneTmpl {
			cloned[locale] = t
		}
		out.Templates[scene] = cloned
	}
	rawTemplates, _ := value["templates"].(map[string]interface{})
	for scene, rawScene := range rawTemplates {
		if !isVerifyEmailScene(scene) {
			continue
		}
		rawLocales, ok := rawScene.(map[string]interface{})
		if !ok {
			continue
		}
		for locale, rawTmpl := range rawLocales {
			if !isSupportedVerifyEmailLocale(locale) {
				continue
			}
			m, ok := rawTmpl.(map[string]interface{})
			if !ok {
				continue
			}
			t := out.Templates[scene][locale]
			if s, ok := m["subject"].(string); ok {
				t.Subject = s
			}
			if b, ok := m["body"].(string); ok {
				t.Body = b
			}
			out.Templates[scene][locale] = t
		}
	}
	return out
}

// ApplyVerifyEmailTemplateSettingPatch 把补丁合并进当前配置。
func ApplyVerifyEmailTemplateSettingPatch(current VerifyEmailTemplateSetting, patch VerifyEmailTemplateSettingPatch) (VerifyEmailTemplateSetting, error) {
	next := NormalizeVerifyEmailTemplateSetting(current)
	for scene, locales := range patch.Templates {
		if !isVerifyEmailScene(scene) {
			return VerifyEmailTemplateSetting{}, ErrVerifyEmailTemplateConfigInvalid
		}
		for locale, p := range locales {
			if !isSupportedVerifyEmailLocale(locale) {
				return VerifyEmailTemplateSetting{}, ErrVerifyEmailTemplateConfigInvalid
			}
			t := next.Templates[scene][locale]
			if p.Subject != nil {
				t.Subject = strings.TrimSpace(*p.Subject)
			}
			if p.Body != nil {
				t.Body = strings.TrimSpace(*p.Body)
			}
			next.Templates[scene][locale] = t
		}
	}
	next = NormalizeVerifyEmailTemplateSetting(next)
	if err := ValidateVerifyEmailTemplateSetting(next); err != nil {
		return VerifyEmailTemplateSetting{}, err
	}
	return next, nil
}

// ResolveVerifyEmailTemplate 按场景与语言取模板。
// 语言缺失时回落简体，场景未知时返回 false 由调用方决定兜底。
func ResolveVerifyEmailTemplate(setting VerifyEmailTemplateSetting, scene, locale string) (VerifyEmailLocalizedTemplate, bool) {
	sceneTmpl, ok := setting.Templates[scene]
	if !ok {
		return VerifyEmailLocalizedTemplate{}, false
	}
	if t, ok := sceneTmpl[locale]; ok && strings.TrimSpace(t.Body) != "" {
		return t, true
	}
	if t, ok := sceneTmpl[constants.LocaleZhCN]; ok && strings.TrimSpace(t.Body) != "" {
		return t, true
	}
	return VerifyEmailLocalizedTemplate{}, false
}
