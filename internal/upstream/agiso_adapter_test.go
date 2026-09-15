package upstream

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestEncodeParseAgisoCredentialsRoundTrip(t *testing.T) {
	packed, err := EncodeAgisoCredentials(" sk_app ", " tk_merchant ")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := ParseAgisoCredentials(packed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.AppSecret != "sk_app" || got.AccessToken != "tk_merchant" {
		t.Fatalf("往返丢字段或未去空白: %+v", got)
	}
}

// 两个密钥都必须齐全才能建连接——缺一个就调不通，
// 与其等到测试连接时报难懂的验签失败，不如在保存时就拒绝。
func TestParseAgisoCredentialsRequiresBoth(t *testing.T) {
	onlySecret, _ := EncodeAgisoCredentials("sk", "")
	if _, err := ParseAgisoCredentials(onlySecret); err == nil {
		t.Error("缺 AccessToken 应当报错")
	}
	onlyToken, _ := EncodeAgisoCredentials("", "tk")
	if _, err := ParseAgisoCredentials(onlyToken); err == nil {
		t.Error("缺 AppSecret 应当报错")
	}
	if _, err := ParseAgisoCredentials(""); err == nil {
		t.Error("空凭据应当报错")
	}
}

// 兼容裸串：历史数据或误填时视为只给了 AppSecret，
// 报「缺 AccessToken」而不是抛出难以理解的 JSON 解析错误。
func TestParseAgisoCredentialsAcceptsLegacyPlainSecret(t *testing.T) {
	_, err := ParseAgisoCredentials("plain-secret-without-json")
	if err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("裸串应当报缺 access_token，实际: %v", err)
	}
}

// 工厂要能按协议分发到 agiso 适配器。
func TestNewAdapterDispatchesAgiso(t *testing.T) {
	packed, _ := EncodeAgisoCredentials("sk", "tk")
	a, err := NewAdapter(&siteconnectiondomain.Connection{
		BaseURL:   "https://gw-api.agiso.com",
		ApiSecret: packed,
		Protocol:  constants.ConnectionProtocolAgiso,
	}, "")
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if _, ok := a.(*agisoAdapter); !ok {
		t.Fatalf("应当返回 agiso 适配器，实际 %T", a)
	}
}

// 凭据不全时工厂就该失败，不要造出一个注定调不通的适配器。
func TestNewAdapterRejectsIncompleteAgisoCredentials(t *testing.T) {
	if _, err := NewAdapter(&siteconnectiondomain.Connection{
		BaseURL:   "https://gw-api.agiso.com",
		ApiSecret: "",
		Protocol:  constants.ConnectionProtocolAgiso,
	}, ""); err == nil {
		t.Fatal("空凭据应当在构造时就失败")
	}
}

// 提卡不可撤销、也没有可轮询的订单：这三个方法必须明确报错，
// 静默成功会让采购流程误以为库存已归还或订单还在处理中。
func TestAgisoUnsupportedOperationsFailLoudly(t *testing.T) {
	packed, _ := EncodeAgisoCredentials("sk", "tk")
	a, err := NewAdapter(&siteconnectiondomain.Connection{
		BaseURL: "https://example.invalid", ApiSecret: packed,
		Protocol: constants.ConnectionProtocolAgiso,
	}, "")
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if _, err := a.GetOrder(t.Context(), 1); err == nil {
		t.Error("GetOrder 应当明确不支持")
	}
	if err := a.CancelOrder(t.Context(), 1); err == nil {
		t.Error("CancelOrder 应当明确不支持")
	}
	if _, err := a.DownloadImage(t.Context(), "x"); err == nil {
		t.Error("DownloadImage 应当明确不支持")
	}
	// 分类接口应报告「不支持」而非返回零个分类，否则映射流程会误判上游没有分类
	cats, err := a.ListCategories(t.Context())
	if err != nil || cats == nil || cats.Supported {
		t.Errorf("ListCategories 应返回 Supported=false: %+v %v", cats, err)
	}
}

// 没有幂等键就提卡，一次超时重试可能重复扣卡，必须在发请求前拦住。
func TestAgisoCreateOrderRequiresOrderNo(t *testing.T) {
	packed, _ := EncodeAgisoCredentials("sk", "tk")
	a, _ := NewAdapter(&siteconnectiondomain.Connection{
		BaseURL: "https://example.invalid", ApiSecret: packed,
		Protocol: constants.ConnectionProtocolAgiso,
	}, "")
	if _, err := a.CreateOrder(t.Context(), CreateUpstreamOrderReq{SKUID: 1, Quantity: 1}); err == nil {
		t.Fatal("缺下游订单号应当报错")
	}
}
