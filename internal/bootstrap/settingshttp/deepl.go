package settingsbootstrap

import (
	"strings"

	"github.com/dujiao-next/internal/config"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	supportapp "github.com/dujiao-next/internal/modules/support/application"
)

type settingsDeepLAdapter struct {
	settings    *settingsapp.Service
	cfg         *config.Config
	translators *supportapp.TranslatorHolder
}

func (a settingsDeepLAdapter) GetDeepLSetting() (settingssecurity.DeepLSetting, error) {
	return a.settings.GetDeepLSetting(a.cfg.DeepL)
}

func (a settingsDeepLAdapter) PatchDeepLSetting(patch settingssecurity.DeepLSettingPatch) (settingssecurity.DeepLSetting, error) {
	return a.settings.PatchDeepLSetting(a.cfg.DeepL, patch)
}

// ApplyRuntime 把刚保存的设置推给正在跑的翻译器。
//
// 两条约束：
//
//  1. 必须无条件调用 Configure。只在"有密钥"时才推，就会留下
//     "后台把翻译关了、旧密钥还在后台继续翻译并计费"的反向 bug。
//  2. 绝不回写 a.cfg。这个 *config.Config 被容器和路由共享，
//     一个管理员保存设置、另一个同时刷新页面就是并发读写同一个 string 字段。
//     启动配置只作为"数据库还没有这一行"时的回退，落库之后它就该保持不变——
//     同目录的 Google 适配器就是这么做的，并且有专门的并发回归测试钉着。
func (a settingsDeepLAdapter) ApplyRuntime(setting settingssecurity.DeepLSetting) {
	runtimeCfg := settingssecurity.DeepLSettingToConfig(setting)
	a.translators.Configure(runtimeCfg.Enabled, runtimeCfg.AuthKey)
}

// VerifyAuthKey 试译一次。传空串表示测当前已保存的密钥——
// 后台读接口不回显密钥，运营不改密钥时表单里就是空的。
func (a settingsDeepLAdapter) VerifyAuthKey(authKey string) error {
	candidate := strings.TrimSpace(authKey)
	if candidate == "" {
		setting, err := a.GetDeepLSetting()
		if err != nil {
			return err
		}
		candidate = setting.AuthKey
	}
	return supportapp.VerifyAuthKey(candidate)
}
