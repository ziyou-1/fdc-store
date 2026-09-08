package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/gin-gonic/gin"
)

func newCtx(headers map[string]string, query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	target := "/"
	if query != "" {
		target += "?" + query
	}
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}
	return c
}

// 站点面向印尼市场：请求里问不出语言时必须按印尼语走。
// 这里曾经有四处独立的中文兜底（ResolveLocale 三处 + T 一处），
// 改默认语言时只改了 normalizeLocale 一处，导致买家不带 X-Lang 仍收到中文报错。
func TestResolveLocaleFallsBackToSiteDefault(t *testing.T) {
	if DefaultLocale != constants.LocaleIdID {
		t.Fatalf("站点默认语言应为印尼语，实得 %s", DefaultLocale)
	}
	cases := []struct {
		name string
		ctx  *gin.Context
		want string
	}{
		{"无任何语言信息", newCtx(nil, ""), DefaultLocale},
		{"Accept-Language 为空串", newCtx(map[string]string{"Accept-Language": "  "}, ""), DefaultLocale},
		{"nil context", nil, DefaultLocale},
		{"X-Lang 显式中文", newCtx(map[string]string{"X-Lang": "zh-CN"}, ""), constants.LocaleZhCN},
		{"X-Lang 显式英文", newCtx(map[string]string{"X-Lang": "en-US"}, ""), constants.LocaleEnUS},
		{"Accept-Language 繁体", newCtx(map[string]string{"Accept-Language": "zh-TW,zh;q=0.9"}, ""), constants.LocaleZhTW},
		{"query 参数优先", newCtx(map[string]string{"X-Lang": "zh-CN"}, "lang=id-ID"), constants.LocaleIdID},
		{"无法识别的语言", newCtx(map[string]string{"X-Lang": "xx-YY"}, ""), DefaultLocale},
	}
	for _, c := range cases {
		if got := ResolveLocale(c.ctx); got != c.want {
			t.Fatalf("%s: ResolveLocale = %q，期望 %q", c.name, got, c.want)
		}
	}
}

// T 传空 locale 时按站点默认语言取，而不是中文。
func TestTranslateEmptyLocaleUsesSiteDefault(t *testing.T) {
	key := "error.user_not_found"
	got := T("", key)
	want := T(DefaultLocale, key)
	if got != want {
		t.Fatalf("T(\"\") = %q，期望与 T(默认语言) 一致 %q", got, want)
	}
	if got == T(constants.LocaleZhCN, key) {
		t.Fatal("空 locale 仍回落到了中文")
	}
}
