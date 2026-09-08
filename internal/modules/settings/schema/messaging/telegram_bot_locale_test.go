package settingsmessaging

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

// assertLocalizedTextComplete 断言一段多语言文本对每种受支持语言都有非空值。
func assertLocalizedTextComplete(t *testing.T, label string, lt LocalizedText) {
	t.Helper()
	for _, locale := range constants.SupportedLocales {
		if strings.TrimSpace(lt[locale]) == "" {
			t.Fatalf("%s 缺少语言 %s 的文案（实得 %q）", label, locale, lt[locale])
		}
	}
}

// Telegram 机器人是直接面向买家的，默认文案必须覆盖每一种受支持语言。
// 这条是守卫：以后新增语言而没补机器人文案，这里会直接红——
// 空串不会编译报错，只会让买家收到一条没有文字的按钮或消息。
func TestTelegramBotDefaultCopyCoversEveryLocale(t *testing.T) {
	cfg := DefaultTelegramBotConfig()

	assertLocalizedTextComplete(t, "help.title", cfg.Help.Title)
	assertLocalizedTextComplete(t, "help.intro", cfg.Help.Intro)
	assertLocalizedTextComplete(t, "help.center_hint", cfg.Help.CenterHint)
	assertLocalizedTextComplete(t, "help.support_hint", cfg.Help.SupportHint)

	if len(cfg.Help.Items) == 0 {
		t.Fatal("帮助中心默认条目为空")
	}
	for _, item := range cfg.Help.Items {
		assertLocalizedTextComplete(t, "help.item["+item.Key+"].summary", item.Summary)
		assertLocalizedTextComplete(t, "help.item["+item.Key+"].title", item.Title)
		assertLocalizedTextComplete(t, "help.item["+item.Key+"].content", item.Content)
	}
}

// 菜单按钮标签同样要覆盖每种语言。
func TestTelegramBotMenuLabelsCoverEveryLocale(t *testing.T) {
	cfg := DefaultTelegramBotConfig()
	if len(cfg.Menu.Items) == 0 {
		t.Fatal("菜单默认条目为空")
	}
	for _, item := range cfg.Menu.Items {
		assertLocalizedTextComplete(t, "menu["+item.Key+"].label", item.Label)
	}
}

// 印尼语必须真的是印尼语，而不是把英文抄一遍充数。
func TestTelegramBotIndonesianIsNotEnglishCopy(t *testing.T) {
	cfg := DefaultTelegramBotConfig()
	pairs := []struct {
		label string
		lt    LocalizedText
	}{
		{"help.title", cfg.Help.Title},
		{"help.intro", cfg.Help.Intro},
		{"help.center_hint", cfg.Help.CenterHint},
		{"help.support_hint", cfg.Help.SupportHint},
	}
	for _, p := range pairs {
		if p.lt[constants.LocaleIdID] == p.lt[constants.LocaleEnUS] {
			t.Fatalf("%s 的印尼语与英文完全相同，疑似未翻译", p.label)
		}
	}
}
