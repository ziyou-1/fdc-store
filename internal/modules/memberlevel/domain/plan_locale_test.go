package domain

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

// 会员套餐名与描述直接展示给买家，必须覆盖每一种受支持语言。
// 这是守卫：以后新增语言而没补种子文案，这里会直接红——
// 漏了不会报错，只会让该语言的买家看到回退的中文。
func TestDefaultPlansCoverEveryLocale(t *testing.T) {
	plans := DefaultPlans()
	if len(plans) == 0 {
		t.Fatal("默认套餐为空")
	}
	for _, plan := range plans {
		for _, locale := range constants.SupportedLocales {
			name, _ := plan.NameJSON[locale].(string)
			if strings.TrimSpace(name) == "" {
				t.Fatalf("套餐 %s 缺少语言 %s 的名称", plan.Period, locale)
			}
			desc, _ := plan.DescriptionJSON[locale].(string)
			if strings.TrimSpace(desc) == "" {
				t.Fatalf("套餐 %s 缺少语言 %s 的描述", plan.Period, locale)
			}
		}
		// 印尼语不得直接抄英文充数
		if plan.NameJSON[constants.LocaleIdID] == plan.NameJSON[constants.LocaleEnUS] {
			t.Fatalf("套餐 %s 的印尼语名称与英文相同，疑似未翻译", plan.Period)
		}
	}
}
