package settingssecurity

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/config"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrDeepLConfigInvalid = errors.New("deepl config invalid")

// DeepLSetting DeepL 翻译服务配置实体。
//
// Enabled 与 AuthKey 是两件事，必须分开存。
// 原先"是否翻译"这个开关隐含在"AuthKey 是否为空"里，导致运营想临时停用翻译
// 只能把密钥删掉，重新启用时还得再去 DeepL 后台抄一遍。
type DeepLSetting struct {
	Enabled bool   `json:"enabled"`
	AuthKey string `json:"auth_key"`
}

// DeepLSettingPatch DeepL 配置补丁。指针为 nil 表示本次不提交该字段。
type DeepLSettingPatch struct {
	Enabled *bool `json:"enabled"`
	// AuthKey 提交空串表示"保持原值"，见 ApplyDeepLSettingPatch。
	AuthKey *string `json:"auth_key"`
	// ClearAuthKey 是唯一能把已存密钥抹掉的入口。
	// 因为空串被占用为"保持不变"，没有它就永远删不掉密钥——
	// 密钥泄露时运营只能去改数据库，而旧密钥会留在之后每一份备份里。
	ClearAuthKey *bool `json:"clear_auth_key"`
}

// DefaultDeepLSetting 根据 config.yml 生成默认设置。
//
// config.yml 只是首次启动的回退值，数据库里的 settings 才是权威。
// 老配置里没有 enabled 字段，此时按"填了密钥即视为启用"推断，
// 否则升级到本版本之后原本在翻译的站点会突然不翻译了。
func DefaultDeepLSetting(cfg config.DeepLConfig) DeepLSetting {
	authKey := strings.TrimSpace(cfg.AuthKey)
	return NormalizeDeepLSetting(DeepLSetting{
		Enabled: cfg.Enabled || authKey != "",
		AuthKey: authKey,
	})
}

// NormalizeDeepLSetting 归一化 DeepL 配置。
func NormalizeDeepLSetting(setting DeepLSetting) DeepLSetting {
	setting.AuthKey = strings.TrimSpace(setting.AuthKey)
	return setting
}

// ValidateDeepLSetting 校验 DeepL 配置。
// 只在启用时才要求密钥非空——没启用就不该拦着运营把密钥清空。
func ValidateDeepLSetting(setting DeepLSetting) error {
	normalized := NormalizeDeepLSetting(setting)
	if normalized.Enabled && normalized.AuthKey == "" {
		return fmt.Errorf("%w: 启用翻译时 DeepL Auth Key 不能为空", ErrDeepLConfigInvalid)
	}
	return nil
}

// ApplyDeepLSettingPatch 把补丁应用到当前配置（不做校验，校验由调用方做）。
//
// AuthKey 的语义与 Turnstile SecretKey / SMTP Password 一致：
// 提交空串表示"保持原值不变"，因为后台读接口永远返回空串，
// 运营不改密钥时表单里就是空的，直接覆盖会把已配置的密钥清掉。
//
// 正因为空串被占用了，清除密钥必须有个显式入口，就是 ClearAuthKey。
// 顺序是先清后写：同时提交清除和新密钥时按"换一把"处理。
func ApplyDeepLSettingPatch(current DeepLSetting, patch DeepLSettingPatch) DeepLSetting {
	next := current
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.ClearAuthKey != nil && *patch.ClearAuthKey {
		next.AuthKey = ""
		// 密钥都没了就不可能还开着翻译，否则校验必然失败、运营得再点一次。
		next.Enabled = false
	}
	if patch.AuthKey != nil {
		if authKey := strings.TrimSpace(*patch.AuthKey); authKey != "" {
			next.AuthKey = authKey
		}
	}
	return next
}

// DeepLSettingToConfig 转换为运行时配置。
func DeepLSettingToConfig(setting DeepLSetting) config.DeepLConfig {
	normalized := NormalizeDeepLSetting(setting)
	return config.DeepLConfig{
		Enabled: normalized.Enabled,
		AuthKey: normalized.AuthKey,
	}
}

// EncodeDeepLSetting 转换为 settings 存储结构（含密钥真值）。
func EncodeDeepLSetting(setting DeepLSetting) jsonmap.JSON {
	normalized := NormalizeDeepLSetting(setting)
	return jsonmap.JSON{
		"enabled":  normalized.Enabled,
		"auth_key": normalized.AuthKey,
	}
}

// MaskDeepLSettingForAdmin 返回后台可见配置。
// 密钥恒为空串，只用 has_auth_key 告诉前端"已配置过"。
func MaskDeepLSettingForAdmin(setting DeepLSetting) jsonmap.JSON {
	normalized := NormalizeDeepLSetting(setting)
	return jsonmap.JSON{
		"enabled":      normalized.Enabled,
		"auth_key":     "",
		"has_auth_key": normalized.AuthKey != "",
	}
}

// DecodeDeepLSetting 从持久化 JSON 解码，缺失字段用 fallback 补。
func DecodeDeepLSetting(raw jsonmap.JSON, fallback DeepLSetting) DeepLSetting {
	next := fallback
	if raw == nil {
		return NormalizeDeepLSetting(next)
	}
	if value, exists := raw["enabled"]; exists {
		next.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["auth_key"]; exists {
		if text, ok := value.(string); ok {
			next.AuthKey = text
		}
	}
	return NormalizeDeepLSetting(next)
}

// NormalizeDeepLSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeDeepLSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeDeepLSetting(DecodeDeepLSetting(raw, DefaultDeepLSetting(config.DeepLConfig{})))
}
