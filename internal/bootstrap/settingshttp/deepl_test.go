package settingsbootstrap

import (
	"fmt"
	"sync"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	supportapp "github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// 后台关掉翻译后，正在跑的翻译器必须立刻停用。
//
// 少了这条，会出现"后台显示已关闭、旧密钥还在后台继续翻译并向 DeepL 计费"的
// 反向 bug——ApplyRuntime 里只在有密钥时才推送就会这样。
func TestApplyRuntimeDisablesLiveTranslator(t *testing.T) {
	cfg := &config.Config{DeepL: config.DeepLConfig{Enabled: true, AuthKey: "seed-key"}}
	holder := supportapp.NewTranslatorHolder(cfg.DeepL.Enabled, cfg.DeepL.AuthKey)
	if !holder.Configured() {
		t.Fatal("前置条件不成立：初始应有可用翻译器")
	}

	adapter := settingsDeepLAdapter{cfg: cfg, translators: holder}
	adapter.ApplyRuntime(settingssecurity.DeepLSetting{Enabled: false, AuthKey: "seed-key"})

	if holder.Configured() {
		t.Fatal("后台已关闭翻译，运行时翻译器仍然可用")
	}
	// 启动配置是"数据库还没有这一行"时的回退，ApplyRuntime 不该回写它
	// （回写会与并发的读构成 data race，见 KeepsStartupFallbackImmutable）。
	if !cfg.DeepL.Enabled || cfg.DeepL.AuthKey != "seed-key" {
		t.Fatalf("启动配置不该被 ApplyRuntime 改动，实得 %#v", cfg.DeepL)
	}
}

// 后台第一次填密钥（启动时为空）必须不重启就生效。
// 这是原实现最大的坑：启动时密钥为空则翻译器压根没被注入。
func TestApplyRuntimeEnablesTranslatorWithoutRestart(t *testing.T) {
	cfg := &config.Config{DeepL: config.DeepLConfig{}}
	holder := supportapp.NewTranslatorHolder(cfg.DeepL.Enabled, cfg.DeepL.AuthKey)
	if holder.Configured() {
		t.Fatal("前置条件不成立：初始不该有翻译器")
	}

	adapter := settingsDeepLAdapter{cfg: cfg, translators: holder}
	adapter.ApplyRuntime(settingssecurity.DeepLSetting{Enabled: true, AuthKey: "fresh-key"})

	if !holder.Configured() {
		t.Fatal("后台首次配置密钥后，运行时翻译器仍未启用")
	}
	if cfg.DeepL.AuthKey != "" {
		t.Fatalf("启动配置不该被 ApplyRuntime 写入密钥，实得 %q", cfg.DeepL.AuthKey)
	}
}

// 热换发生在后台请求的 goroutine，翻译发生在买家请求的 goroutine。
// 这条用 -race 跑才有意义，但即便不带 -race 也能挡住"忘了加锁"的实现。
func TestTranslatorHolderIsRaceFree(t *testing.T) {
	holder := supportapp.NewTranslatorHolder(false, "")

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			// 交替开关，模拟运营反复保存设置
			holder.Configure(n%2 == 0, "key-value")
		}(i)
		go func() {
			defer wg.Done()
			// 买家侧只关心"当前有没有配置"，不真的发网络请求
			_ = holder.Configured()
		}()
	}
	wg.Wait()
}

// ApplyRuntime 绝不能回写启动配置。
//
// 这个 *config.Config 被容器与路由共享：一个管理员保存设置、另一个同时刷新页面，
// 就是对同一个 string 字段的并发读写。同目录的 Google 适配器为此专门写过一条
// 并发回归测试（TestSettingsGoogleAuthAdapterKeepsStartupFallbackImmutable），
// DeepL 一开始漏了，复审时被指出来。
//
// 注意本机 CGO_ENABLED=0 跑不了 -race，所以这条测试的价值主要在最后那个断言：
// 启动配置必须一字未改。
func TestSettingsDeepLAdapterKeepsStartupFallbackImmutable(t *testing.T) {
	const fallbackKey = "startup-key"
	cfg := &config.Config{DeepL: config.DeepLConfig{Enabled: false, AuthKey: fallbackKey}}
	store := &googleAuthSettingStore{store: make(map[string]jsonmap.JSON)}
	holder := supportapp.NewTranslatorHolder(false, "")
	adapter := settingsDeepLAdapter{
		settings:    settingsapp.NewService(store),
		cfg:         cfg,
		translators: holder,
	}

	var wait sync.WaitGroup
	for index := 0; index < 64; index++ {
		index := index
		wait.Add(3)
		go func() {
			defer wait.Done()
			if _, err := adapter.GetDeepLSetting(); err != nil {
				t.Errorf("get setting: %v", err)
			}
		}()
		go func() {
			defer wait.Done()
			enabled := true
			authKey := fmt.Sprintf("patched-key-%d", index)
			if _, err := adapter.PatchDeepLSetting(settingssecurity.DeepLSettingPatch{
				Enabled: &enabled,
				AuthKey: &authKey,
			}); err != nil {
				t.Errorf("patch setting: %v", err)
			}
		}()
		go func() {
			defer wait.Done()
			adapter.ApplyRuntime(settingssecurity.DeepLSetting{
				Enabled: true,
				AuthKey: fmt.Sprintf("runtime-key-%d", index),
			})
		}()
	}
	wait.Wait()

	if cfg.DeepL.Enabled || cfg.DeepL.AuthKey != fallbackKey {
		t.Fatalf("启动配置被改写了: %#v", cfg.DeepL)
	}
	if _, ok, err := store.GetByKey(constants.SettingKeyDeepLConfig); err != nil || !ok {
		t.Fatalf("数据库里没有落下设置: ok=%v err=%v", ok, err)
	}
	if !holder.Configured() {
		t.Fatal("并发保存之后运行时翻译器应当是可用的")
	}
}

// 清除密钥必须真的把它从库里抹掉，并顺带关闭翻译。
//
// 空串在补丁语义里表示"保持不变"，所以没有这条显式路径，
// 密钥一旦落库就再也删不掉，会留在之后每一份数据库备份里。
func TestClearAuthKeyRemovesStoredSecret(t *testing.T) {
	cfg := &config.Config{DeepL: config.DeepLConfig{}}
	store := &googleAuthSettingStore{store: make(map[string]jsonmap.JSON)}
	holder := supportapp.NewTranslatorHolder(false, "")
	adapter := settingsDeepLAdapter{
		settings:    settingsapp.NewService(store),
		cfg:         cfg,
		translators: holder,
	}

	enabled := true
	authKey := "key-to-be-removed"
	if _, err := adapter.PatchDeepLSetting(settingssecurity.DeepLSettingPatch{
		Enabled: &enabled,
		AuthKey: &authKey,
	}); err != nil {
		t.Fatalf("先存一把密钥失败: %v", err)
	}

	clear := true
	after, err := adapter.PatchDeepLSetting(settingssecurity.DeepLSettingPatch{ClearAuthKey: &clear})
	if err != nil {
		t.Fatalf("清除密钥失败: %v", err)
	}
	if after.AuthKey != "" {
		t.Fatalf("密钥未被清除，实得 %q", after.AuthKey)
	}
	if after.Enabled {
		t.Fatal("密钥清掉之后翻译必须一并关闭，否则校验永远过不去")
	}

	stored, ok, err := store.GetByKey(constants.SettingKeyDeepLConfig)
	if err != nil || !ok {
		t.Fatalf("读回存储失败: ok=%v err=%v", ok, err)
	}
	if got := stored["auth_key"]; got != "" {
		t.Fatalf("库里仍残留密钥: %v", got)
	}
}

// 老配置只写了 auth_key、没有 enabled 时，升级后必须继续翻译。
//
// 装配路径直读 cfg.Enabled 会是 false，翻译会静默停掉；
// 走 GetDeepLSetting 才会经过 DefaultDeepLSetting 的"有密钥即视为启用"推断。
// 这正是启动时必须回灌 DeepL 设置的原因。
func TestLegacyConfigWithOnlyAuthKeyStaysEnabled(t *testing.T) {
	cfg := &config.Config{DeepL: config.DeepLConfig{AuthKey: "legacy-key:fx"}}
	store := &googleAuthSettingStore{store: make(map[string]jsonmap.JSON)}
	adapter := settingsDeepLAdapter{
		settings:    settingsapp.NewService(store),
		cfg:         cfg,
		translators: supportapp.NewTranslatorHolder(false, ""),
	}

	setting, err := adapter.GetDeepLSetting()
	if err != nil {
		t.Fatalf("读设置失败: %v", err)
	}
	if !setting.Enabled {
		t.Fatal("老配置只填了密钥，应推断为启用，否则升级即静默停翻译")
	}
	if setting.AuthKey != "legacy-key:fx" {
		t.Fatalf("密钥丢失，实得 %q", setting.AuthKey)
	}
}
