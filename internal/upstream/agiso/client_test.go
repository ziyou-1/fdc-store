package agiso

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := NewClient(Config{
		BaseURL:     srv.URL,
		AppSecret:   "sk_test",
		AccessToken: "tk_test",
		AppID:       "ap_test",
	}, srv.Client())
	c.now = func() time.Time { return time.Unix(1468476350, 0) } // 固定时间戳，签名可复算
	return c
}

// 每次调用都必须带齐鉴权头、表单编码，且 sign 要能被服务端用同一算法验回来。
func TestCallSendsSignedForm(t *testing.T) {
	var gotAuth, gotVer, gotCT, gotSign, gotCpk string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotAuth, gotVer, gotCT = r.Header.Get("Authorization"), r.Header.Get("ApiVersion"), r.Header.Get("Content-Type")
		gotSign, gotCpk = r.PostFormValue("sign"), r.PostFormValue("cpkId")
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":{"IdNo":32,"Title":"t","CardType":800}}`))
	})

	if _, err := c.GetCardKind(context.Background(), 32); err != nil {
		t.Fatalf("GetCardKind: %v", err)
	}
	if gotAuth != "Bearer tk_test" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotVer != "1" {
		t.Errorf("ApiVersion = %q", gotVer)
	}
	if gotCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	// 服务端用同样的算法验签
	want := SignRequest("sk_test", map[string]string{"cpkId": gotCpk, "timestamp": "1468476350"})
	if gotSign != want {
		t.Errorf("sign 不匹配\n got=%s\nwant=%s", gotSign, want)
	}
}

func TestListCardKindsParsesPage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":{"List":[
			{"IdNo":123110,"CardType":800,"Title":"测试Jd","RemainingCount":13,"UsedCount":1,"TotalCount":14,"PurPrice":1}
		],"TotalCount":65,"PageNo":1,"PageSize":20}}`))
	})
	page, err := c.ListCardKinds(context.Background(), 1, 20, "")
	if err != nil {
		t.Fatalf("ListCardKinds: %v", err)
	}
	if len(page.List) != 1 || page.List[0].IdNo != 123110 || page.List[0].RemainingCount != 13 {
		t.Fatalf("解析结果不对: %+v", page)
	}
	if page.TotalCount != 65 {
		t.Errorf("TotalCount = %d", page.TotalCount)
	}
}

// pageSize 上限 100，越界要在客户端夹住，别指望服务端行为。
func TestListCardKindsClampsPageSize(t *testing.T) {
	var got string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.PostFormValue("pageSize")
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":{"List":[],"TotalCount":0}}`))
	})
	if _, err := c.ListCardKinds(context.Background(), 0, 5000, ""); err != nil {
		t.Fatalf("ListCardKinds: %v", err)
	}
	if got != "100" {
		t.Fatalf("pageSize 未被夹到 100，实际发出 %q", got)
	}
}

func TestHandPickReturnsCards(t *testing.T) {
	var gotOrderID string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotOrderID = r.PostFormValue("handPickOrderId")
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":{"CardPwdArr":[
			{"c":"xx003","p":"123","d":"2024-11-21 23:59:59"}
		]}}`))
	})
	res, err := c.HandPick(context.Background(), HandPickInput{CpkID: 1, Num: 1, OrderNo: "DJ20260909001"})
	if err != nil {
		t.Fatalf("HandPick: %v", err)
	}
	if gotOrderID != "DJ20260909001" {
		t.Fatalf("订单号未作为幂等键发出: %q", gotOrderID)
	}
	if len(res.Cards) != 1 || res.Cards[0].CardNo != "xx003" || res.Cards[0].Password != "123" {
		t.Fatalf("卡密解析不对: %+v", res.Cards)
	}
}

// 没有幂等键就提卡，一次超时重试就可能重复扣卡，必须在客户端拦住。
func TestHandPickRequiresOrderNo(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("缺少订单号时不应发出任何请求")
	})
	_, err := c.HandPick(context.Background(), HandPickInput{CpkID: 1, Num: 1})
	if err == nil {
		t.Fatal("缺少 OrderNo 应当报错")
	}
}

// 服务端说成功却没给卡，当成功处理会让订单显示已发货但内容为空。
func TestHandPickRejectsEmptySuccess(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":{"CardPwdArr":[]}}`))
	})
	_, err := c.HandPick(context.Background(), HandPickInput{CpkID: 1, Num: 1, OrderNo: "X"})
	if err == nil {
		t.Fatal("成功但零张卡应当报错")
	}
}

func TestHandPickBuildCpd(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostFormValue("buildCpd") != "true" {
			t.Errorf("buildCpd = %q", r.PostFormValue("buildCpd"))
		}
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":{"CpdUrl":"https://mai.91kami.com/cpd/abc.aspx"}}`))
	})
	res, err := c.HandPick(context.Background(), HandPickInput{CpkID: 1, Num: 1, OrderNo: "X", BuildCpd: true})
	if err != nil {
		t.Fatalf("HandPick: %v", err)
	}
	if res.CpdURL == "" {
		t.Fatal("应当返回提取链接")
	}
}

// 错误码要归到正确的哨兵错误上——上层据此决定告警、重试还是转人工。
func TestErrorCodeClassification(t *testing.T) {
	cases := []struct {
		code int
		want error
		name string
	}{
		{1, ErrTokenInvalid, "无效AccessToken"},
		{16, ErrInsufficientDeposit, "预存款不足"},
		{2, ErrRateLimited, "调用限制"},
		{6, ErrPartialFailure, "部分失败"},
		{4, ErrSignInvalid, "验签失败"},
		{13, ErrTransient, "超时"},
		{999, ErrBusiness, "未知码归为业务异常"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"IsSuccess":false,"Error_Code":` + itoa(tc.code) + `,"Error_Msg":"e","RequestId":"r1"}`))
			})
			_, err := c.QueryDeposit(context.Background())
			if !errors.Is(err, tc.want) {
				t.Fatalf("错误码 %d 应归为 %v，实际 %v", tc.code, tc.want, err)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.RequestID != "r1" {
				t.Fatalf("应保留 RequestId 便于排查: %v", err)
			}
		})
	}
}

// 服务端显式给了 AllowRetry 时以它为准，压过按错误码的默认判断。
func TestAllowRetryOverridesClassification(t *testing.T) {
	yes := true
	e := &APIError{Code: 3, kind: classify(3), AllowRetry: &yes}
	if !e.Retryable() {
		t.Fatal("AllowRetry=true 应当可重试，即便错误码默认不可重试")
	}
	no := false
	e2 := &APIError{Code: 13, kind: classify(13), AllowRetry: &no}
	if e2.Retryable() {
		t.Fatal("AllowRetry=false 应当禁止重试，即便超时默认可重试")
	}
	e3 := &APIError{Code: 13, kind: classify(13)}
	if !e3.Retryable() {
		t.Fatal("未给 AllowRetry 时，超时应按错误码判为可重试")
	}
}

func TestHTTP5xxIsTransient(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	_, err := c.QueryDeposit(context.Background())
	if !errors.Is(err, ErrTransient) {
		t.Fatalf("5xx 应为瞬时错误: %v", err)
	}
}

func TestMissingCredentialsRejected(t *testing.T) {
	c := NewClient(Config{BaseURL: "http://x", AppSecret: "", AccessToken: "t"}, nil)
	if _, err := c.QueryDeposit(context.Background()); err == nil {
		t.Fatal("缺 AppSecret 应当报错")
	}
	// 余额查询不需要 AccessToken，因此这条要用卡券接口来验
	c2 := NewClient(Config{BaseURL: "http://x", AppSecret: "s", AccessToken: "", AppID: "a"}, nil)
	if _, err := c2.ListCardKinds(context.Background(), 1, 10, ""); err == nil {
		t.Fatal("卡券接口缺 AccessToken 应当报错")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// 余额查询与其余三个接口都不同：路径在 /open、按 appId 鉴权不发 Authorization、
// Data 是裸数字。这三点任一写错都会得到「未知的URL」或解码失败。
func TestQueryDepositUsesOpenNamespaceAndAppID(t *testing.T) {
	var gotPath, gotAuth, gotAppID string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAppID = r.PostFormValue("appId")
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":9610.737}`))
	})
	c.cfg.AppID = "ap_test"

	dep, err := c.QueryDeposit(context.Background())
	if err != nil {
		t.Fatalf("QueryDeposit: %v", err)
	}
	if gotPath != "/open/Bankroll/QueryDeposit" {
		t.Errorf("路径错误: %s", gotPath)
	}
	if gotAuth != "" {
		t.Errorf("该接口不应发 Authorization 头，实际发了 %q", gotAuth)
	}
	if gotAppID != "ap_test" {
		t.Errorf("appId 未作为参数发出: %q", gotAppID)
	}
	if dep.Balance != 9610.737 {
		t.Errorf("余额解析错误: %v", dep.Balance)
	}
}

// 缺 AppID 时不该发请求——发了也只会拿到一个难懂的参数错误。
func TestQueryDepositRequiresAppID(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("缺 AppID 时不应发出请求")
	})
	c.cfg.AppID = ""
	if _, err := c.QueryDeposit(context.Background()); err == nil {
		t.Fatal("缺 AppID 应当报错")
	}
}

// 余额查询不需要商家授权：它查的是开发者账户，与具体商家无关。
func TestQueryDepositWorksWithoutAccessToken(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"IsSuccess":true,"Error_Code":0,"Data":1.5}`))
	})
	c.cfg.AccessToken = ""
	c.cfg.AppID = "ap_test"
	if _, err := c.QueryDeposit(context.Background()); err != nil {
		t.Fatalf("无 AccessToken 也应可查余额: %v", err)
	}
	// 但卡券接口仍必须要有
	if _, err := c.ListCardKinds(context.Background(), 1, 10, ""); err == nil {
		t.Fatal("卡券接口缺 AccessToken 应当报错")
	}
}
