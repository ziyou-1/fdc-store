package categoryapp

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

// 预置分类名展示在买家端导航与商品卡上，必须覆盖每一种受支持语言。
func TestDefaultStorefrontCategoriesCoverEveryLocale(t *testing.T) {
	seeds := DefaultStorefrontCategories()
	if len(seeds) == 0 {
		t.Fatal("默认分类为空")
	}
	for _, seed := range seeds {
		for _, locale := range constants.SupportedLocales {
			name, _ := seed.NameJSON[locale].(string)
			if strings.TrimSpace(name) == "" {
				t.Fatalf("分类 %s 缺少语言 %s 的名称", seed.Slug, locale)
			}
		}
	}
}
