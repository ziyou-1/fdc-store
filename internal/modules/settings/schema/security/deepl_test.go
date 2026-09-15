package settingssecurity

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/config"
)

// 后台读接口绝不能回显密钥；前端靠 has_auth_key 显示"已配置"。
func TestMaskDeepLSettingHidesAuthKey(t *testing.T) {
	masked := MaskDeepLSettingForAdmin(DeepLSetting{Enabled: true, AuthKey: "secret-key-value"})
	if got := masked["auth_key"]; got != "" {
		t.Fatalf("后台读接口回显了密钥: %v", got)
	}
	if got := masked["has_auth_key"]; got != true {
		t.Fatalf("has_auth_key 应为 true，实得 %v", got)
	}

	empty := MaskDeepLSettingForAdmin(DeepLSetting{})
	if got := empty["has_auth_key"]; got != false {
		t.Fatalf("未配置时 has_auth_key 应为 false，实得 %v", got)
	}
}

// 落库必须是真值，否则保存一次密钥就没了。
func TestEncodeDeepLSettingKeepsAuthKey(t *testing.T) {
	encoded := EncodeDeepLSetting(DeepLSetting{Enabled: true, AuthKey: "  secret  "})
	if got := encoded["auth_key"]; got != "secret" {
		t.Fatalf("落库密钥应为去空白后的真值，实得 %v", got)
	}
}

// 补丁里的空密钥表示"保持不变"——后台表单里密钥框默认就是空的，
// 直接覆盖会让运营每改一次别的字段就把密钥清掉一次。
func TestApplyDeepLPatchEmptyAuthKeyKeepsCurrent(t *testing.T) {
	current := DeepLSetting{Enabled: true, AuthKey: "existing"}

	blank := ""
	next := ApplyDeepLSettingPatch(current, DeepLSettingPatch{AuthKey: &blank})
	if next.AuthKey != "existing" {
		t.Fatalf("提交空密钥应保持原值，实得 %q", next.AuthKey)
	}

	spaces := "   "
	next = ApplyDeepLSettingPatch(current, DeepLSettingPatch{AuthKey: &spaces})
	if next.AuthKey != "existing" {
		t.Fatalf("提交全空白密钥应保持原值，实得 %q", next.AuthKey)
	}

	replacement := "  brand-new  "
	next = ApplyDeepLSettingPatch(current, DeepLSettingPatch{AuthKey: &replacement})
	if next.AuthKey != "brand-new" {
		t.Fatalf("提交新密钥应覆盖并去空白，实得 %q", next.AuthKey)
	}
}

// 开关必须能独立关掉，不依赖清空密钥。
func TestApplyDeepLPatchCanDisableWithoutClearingKey(t *testing.T) {
	disabled := false
	next := ApplyDeepLSettingPatch(DeepLSetting{Enabled: true, AuthKey: "existing"}, DeepLSettingPatch{Enabled: &disabled})
	if next.Enabled {
		t.Fatal("关闭开关未生效")
	}
	if next.AuthKey != "existing" {
		t.Fatalf("关闭开关不应清掉密钥，实得 %q", next.AuthKey)
	}
	if err := ValidateDeepLSetting(next); err != nil {
		t.Fatalf("关闭状态下不该因为密钥而报错: %v", err)
	}
}

// 启用却没有密钥必须被拦下，否则运营以为开了、实际一直没翻译。
func TestValidateDeepLSettingRequiresKeyWhenEnabled(t *testing.T) {
	err := ValidateDeepLSetting(DeepLSetting{Enabled: true})
	if !errors.Is(err, ErrDeepLConfigInvalid) {
		t.Fatalf("启用但无密钥应报 ErrDeepLConfigInvalid，实得 %v", err)
	}
	if err := ValidateDeepLSetting(DeepLSetting{Enabled: false}); err != nil {
		t.Fatalf("未启用且无密钥应通过校验，实得 %v", err)
	}
}

// 老配置里没有 enabled 字段。升级到本版本后，原本靠"填了密钥就翻译"
// 在跑的站点不能突然停止翻译。
func TestDefaultDeepLSettingInfersEnabledFromLegacyKey(t *testing.T) {
	legacy := DefaultDeepLSetting(config.DeepLConfig{AuthKey: "legacy-key"})
	if !legacy.Enabled {
		t.Fatal("老配置只填了密钥，应推断为启用")
	}

	blank := DefaultDeepLSetting(config.DeepLConfig{})
	if blank.Enabled {
		t.Fatal("既没开关也没密钥时不应视为启用")
	}
}

// 解码要能吃下缺字段的历史数据，并且不丢 fallback。
func TestDecodeDeepLSettingFallsBack(t *testing.T) {
	fallback := DeepLSetting{Enabled: true, AuthKey: "from-config"}
	if got := DecodeDeepLSetting(nil, fallback); got != fallback {
		t.Fatalf("raw 为空应原样返回 fallback，实得 %+v", got)
	}
	got := DecodeDeepLSetting(map[string]interface{}{"enabled": false}, fallback)
	if got.Enabled {
		t.Fatal("库里的 enabled=false 应覆盖 fallback")
	}
	if got.AuthKey != "from-config" {
		t.Fatalf("库里没有 auth_key 时应保留 fallback，实得 %q", got.AuthKey)
	}
}

// 清除密钥是唯一能把已存密钥抹掉的入口，且必须连带关闭翻译。
func TestApplyDeepLPatchClearAuthKey(t *testing.T) {
	current := DeepLSetting{Enabled: true, AuthKey: "existing"}

	clear := true
	next := ApplyDeepLSettingPatch(current, DeepLSettingPatch{ClearAuthKey: &clear})
	if next.AuthKey != "" {
		t.Fatalf("清除后密钥应为空，实得 %q", next.AuthKey)
	}
	if next.Enabled {
		t.Fatal("清除密钥必须连带关闭翻译，否则校验必然失败")
	}
	if err := ValidateDeepLSetting(next); err != nil {
		t.Fatalf("清除后的配置应当通过校验，实得 %v", err)
	}

	// clear=false 不该动任何东西
	keep := false
	untouched := ApplyDeepLSettingPatch(current, DeepLSettingPatch{ClearAuthKey: &keep})
	if untouched != current {
		t.Fatalf("clear_auth_key=false 不应改动配置，实得 %+v", untouched)
	}

	// 同时提交清除和新密钥，按"换一把"处理
	replacement := "brand-new"
	swapped := ApplyDeepLSettingPatch(current, DeepLSettingPatch{ClearAuthKey: &clear, AuthKey: &replacement})
	if swapped.AuthKey != "brand-new" {
		t.Fatalf("同时清除并提交新密钥应换成新的，实得 %q", swapped.AuthKey)
	}
}
