package application

import (
	"context"
	"testing"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestResolveTenantReturnURLMainTenantKeepsConfigFallback(t *testing.T) {
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.MainTenantContext("main.example.com"))
	channel := &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"return_url": "https://main.example.com/pay"}}

	if got := resolveTenantReturnURL(ctx, "https", channel); got != "" {
		t.Fatalf("main tenant want empty got %q", got)
	}
	if got := resolveTenantReturnURL(context.Background(), "https", channel); got != "" {
		t.Fatalf("no tenant want empty got %q", got)
	}
	//lint:ignore SA1012 This assertion deliberately verifies the nil-context fallback.
	if got := resolveTenantReturnURL(nil, "https", channel); got != "" {
		t.Fatalf("nil ctx want empty got %q", got)
	}
}

func TestResolveTenantReturnURLMainTenantWithoutConfigGeneratesPay(t *testing.T) {
	// 主站渠道没自配 return_url 时，用当前请求域名兜一个 /pay，
	// 否则 Payssion 这类跳转支付付完/取消后买家会被丢在第三方页回不来。
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.MainTenantContext("oiois.top"))

	// 无配置渠道
	if got := resolveTenantReturnURL(ctx, "https", nil); got != "https://oiois.top/pay" {
		t.Fatalf("main tenant no config want https://oiois.top/pay got %q", got)
	}
	// config 里只有 live_mode（生产实际状态），也算无回跳配置
	channel := &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"live_mode": true}}
	if got := resolveTenantReturnURL(ctx, "https", channel); got != "https://oiois.top/pay" {
		t.Fatalf("main tenant live_mode-only want https://oiois.top/pay got %q", got)
	}
	// 自配了 return_url 的渠道仍返回空，保持原兜底
	configured := &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"return_url": "https://oiois.top/custom"}}
	if got := resolveTenantReturnURL(ctx, "https", configured); got != "" {
		t.Fatalf("main tenant configured want empty got %q", got)
	}
}

func TestResolveTenantReturnURLResellerTenantUsesRequestHost(t *testing.T) {
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.ResellerTenantContext("shop.example.com", 7, 3, "primary.example.com"))
	channel := &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"return_url": "https://main.example.com/pay"}}

	if got := resolveTenantReturnURL(ctx, "https", channel); got != "https://shop.example.com/pay" {
		t.Fatalf("want https://shop.example.com/pay got %q", got)
	}
}

func TestResolveTenantReturnURLResellerTenantFallsBackToPrimaryDomain(t *testing.T) {
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.ResellerTenantContext("", 7, 3, "primary.example.com"))

	if got := resolveTenantReturnURL(ctx, "https", nil); got != "https://primary.example.com/pay" {
		t.Fatalf("want https://primary.example.com/pay got %q", got)
	}
}

func TestResolveTenantReturnURLSchemeHandling(t *testing.T) {
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.ResellerTenantContext("shop.example.com", 7, 3, "primary.example.com"))

	if got := resolveTenantReturnURL(ctx, "http", nil); got != "http://shop.example.com/pay" {
		t.Fatalf("http scheme want http://shop.example.com/pay got %q", got)
	}
	// 空值 / 非法值默认 https
	if got := resolveTenantReturnURL(ctx, "", nil); got != "https://shop.example.com/pay" {
		t.Fatalf("empty scheme want https got %q", got)
	}
	if got := resolveTenantReturnURL(ctx, "ftp", nil); got != "https://shop.example.com/pay" {
		t.Fatalf("invalid scheme want https got %q", got)
	}
}

func TestResolveTenantReturnURLUnavailableTenantKeepsConfigFallback(t *testing.T) {
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.UnavailableTenantContext("gone.example.com", "disabled"))

	if got := resolveTenantReturnURL(ctx, "https", nil); got != "" {
		t.Fatalf("unavailable tenant want empty got %q", got)
	}
}

func TestTenantReturnPathReusesConfiguredPath(t *testing.T) {
	channel := &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"return_url": "https://main.example.com/checkout/result?from=gateway"}}
	if got := tenantReturnPath(channel); got != "/checkout/result?from=gateway" {
		t.Fatalf("want /checkout/result?from=gateway got %q", got)
	}

	// stripe/dujiaopay 使用 success_url
	channel = &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"success_url": "https://main.example.com/pay/success"}}
	if got := tenantReturnPath(channel); got != "/pay/success" {
		t.Fatalf("want /pay/success got %q", got)
	}

	// return_url 优先于 success_url
	channel = &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{
		"return_url":  "https://main.example.com/pay/return",
		"success_url": "https://main.example.com/pay/success",
	}}
	if got := tenantReturnPath(channel); got != "/pay/return" {
		t.Fatalf("want /pay/return got %q", got)
	}
}

func TestTenantReturnPathDefaults(t *testing.T) {
	if got := tenantReturnPath(nil); got != "/pay" {
		t.Fatalf("nil channel want /pay got %q", got)
	}
	if got := tenantReturnPath(&paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{}}); got != "/pay" {
		t.Fatalf("empty config want /pay got %q", got)
	}
	// 配置只有域名没有路径时也回落 /pay
	channel := &paymentdomain.PaymentChannel{ConfigJSON: jsonmap.JSON{"return_url": "https://main.example.com/"}}
	if got := tenantReturnPath(channel); got != "/pay" {
		t.Fatalf("root path want /pay got %q", got)
	}
}
