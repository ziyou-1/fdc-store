package consumer

import (
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
)

// 站点支持的全部界面语言。新增一门语言时这里会先红，
// 提醒把内置文案一并补上——否则该语种的买家会静默收到别国语言的邮件。
var supportedLocales = []string{
	constants.LocaleZhCN,
	constants.LocaleZhTW,
	constants.LocaleEnUS,
	constants.LocaleIdID,
}

// 会员到期提醒是内置文案（不走后台模板），四种语言必须都有，
// 且主题和正文都不能为空——空模板会发出一封只有站点名的信。
func TestMembershipExpiryEmailCoversEveryLocale(t *testing.T) {
	for _, locale := range supportedLocales {
		copyText, ok := membershipExpiryEmailCopy[locale]
		if !ok {
			t.Errorf("会员到期提醒缺少语言 %s", locale)
			continue
		}
		if strings.TrimSpace(copyText.Subject) == "" {
			t.Errorf("语言 %s 的到期提醒主题为空", locale)
		}
		if strings.TrimSpace(copyText.Body) == "" {
			t.Errorf("语言 %s 的到期提醒正文为空", locale)
		}
	}
}

// 印尼语文案必须是真的翻译过，不能复制英文了事。
// 只断言「键存在」挡不住直接把英文粘过去的情况。
func TestMembershipExpiryEmailIndonesianIsTranslated(t *testing.T) {
	id := membershipExpiryEmailCopy[constants.LocaleIdID]
	en := membershipExpiryEmailCopy[constants.LocaleEnUS]
	if id.Subject == en.Subject || id.Body == en.Body {
		t.Fatal("印尼语到期提醒与英文相同，说明没有真正翻译")
	}
}

// 未知语言必须回落站点默认语言，而不是写死的简体中文。
// 站点默认是印尼语，中文邮件对印尼买家等于没发。
func TestBuildMembershipExpiryEmailFallsBackToSiteDefault(t *testing.T) {
	expiresAt := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	unknownSubject, unknownBody := buildMembershipExpiryEmail("fr-FR", "FDC Store", "https://example.com", expiresAt)
	defaultSubject, defaultBody := buildMembershipExpiryEmail(i18n.DefaultLocale, "FDC Store", "https://example.com", expiresAt)

	if unknownSubject != defaultSubject || unknownBody != defaultBody {
		t.Fatalf("未知语言应回落站点默认语言 %s，实得主题 %q", i18n.DefaultLocale, unknownSubject)
	}
	if strings.TrimSpace(unknownSubject) == "" || strings.TrimSpace(unknownBody) == "" {
		t.Fatal("回落后的邮件主题或正文为空")
	}
}

// 使用说明的语言回落链：买家语言优先，其次站点默认语言。
// 老商品可能只录了中文，中文留在链尾兜底，不能因为改了默认语言就取不到值。
func TestLocalizedInstructionsFallbackOrder(t *testing.T) {
	t.Run("命中买家语言", func(t *testing.T) {
		raw := map[string]interface{}{"en-US": "English text", i18n.DefaultLocale: "Teks Indonesia"}
		if got := localizedInstructionsText(raw, constants.LocaleEnUS); got != "English text" {
			t.Fatalf("应取买家语言，实得 %q", got)
		}
	})

	t.Run("缺买家语言时回落站点默认语言而非中文", func(t *testing.T) {
		raw := map[string]interface{}{
			constants.LocaleZhCN: "中文说明",
			i18n.DefaultLocale:   "Teks Indonesia",
		}
		if got := localizedInstructionsText(raw, "fr-FR"); got != "Teks Indonesia" {
			t.Fatalf("应回落站点默认语言，实得 %q", got)
		}
	})

	t.Run("只有中文的老数据仍能取到", func(t *testing.T) {
		raw := map[string]interface{}{constants.LocaleZhCN: "中文说明"}
		if got := localizedInstructionsText(raw, i18n.DefaultLocale); got != "中文说明" {
			t.Fatalf("只录了中文的老商品应仍能取到值，实得 %q", got)
		}
	})
}
