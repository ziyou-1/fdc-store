// Agiso 卡券协议层联调程序。
// 需要先启动 mock_agiso.py（或把 BaseURL 换成真实网关 + 真实凭据）。
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dujiao-next/internal/upstream/agiso"
)

func main() {
	base := envOr("AGISO_BASE_URL", "http://127.0.0.1:9098")
	secret := envOr("AGISO_APP_SECRET", "sk_agiso_mock")
	token := envOr("AGISO_ACCESS_TOKEN", "tk_agiso_mock")
	live := base != "http://127.0.0.1:9098"

	fmt.Printf("目标: %s%s\n\n", base, map[bool]string{true: "  【真实网关】", false: "  【本地 mock】"}[live])

	c := agiso.NewClient(agiso.Config{BaseURL: base, AppSecret: secret, AccessToken: token},
		&http.Client{Timeout: 15 * time.Second})
	ctx := context.Background()
	fail := 0
	check := func(ok bool, label string, detail ...interface{}) {
		if ok {
			fmt.Printf("  ✓ %s\n", label)
		} else {
			fail++
			fmt.Printf("  ✗ %s %v\n", label, detail)
		}
	}

	fmt.Println("=== 1. 卡种列表（后台下拉框的数据源）===")
	page, err := c.ListCardKinds(ctx, 1, 100, "")
	if err != nil {
		fmt.Println("  ✗ 拉取失败:", err)
		os.Exit(1)
	}
	var firstUsable int64
	for _, k := range page.List {
		mark := "可接入"
		if !agiso.SupportedCardType(k.CardType) {
			mark = "不支持(已过滤)"
		} else if firstUsable == 0 {
			firstUsable = k.IdNo
		}
		fmt.Printf("     #%-8d %-20s 类型=%d 库存=%-5d 成本=%.2f  %s\n",
			k.IdNo, k.Title, k.CardType, k.RemainingCount, k.PurPrice, mark)
	}
	check(len(page.List) > 0, "至少返回一个卡种", len(page.List))
	if firstUsable == 0 {
		fmt.Println("\n没有可接入的卡种（只支持唯一卡820/重复卡840），后续用例跳过")
		os.Exit(map[bool]int{true: 1, false: 0}[fail > 0])
	}

	fmt.Println("=== 2. 卡种详情（决定卡密怎么拼给买家）===")
	d, err := c.GetCardKind(ctx, firstUsable)
	if err != nil {
		fmt.Println("  ✗ 详情失败:", err)
		os.Exit(1)
	}
	fmt.Printf("     模板=%q 卡号前缀=%q 密码前缀=%q 分隔符=%q 预警值=%d\n",
		d.ShowTpl, d.PrefixCardNo, d.PrefixPwd, d.SplitCardPwd, d.WarningNum)
	check(d.IdNo == firstUsable, "详情与列表对得上")
	if agiso.NeedsRichRender(d) {
		fmt.Println("     ⚠ 该卡种是条码/二维码类型，纯文本发给买家用不了")
	}

	fmt.Println("=== 3. 余额（余额见底会停掉全站发卡）===")
	dep, err := c.QueryDeposit(ctx)
	check(err == nil, "查询成功", err)
	if err == nil {
		fmt.Printf("     预存款余额 = %.4f\n", dep.Balance)
		if dep.Balance < 1 {
			fmt.Println("     ⚠ 余额偏低，上线前请充值并配置告警")
		}
	}

	if live {
		fmt.Println("\n真实网关模式：到此为止。")
		fmt.Println("提卡会真实消耗库存与余额，需要时用 -pick 参数单独跑：")
		fmt.Println("  go run . -pick")
		if !hasFlag("-pick") {
			done(fail)
		}
	}

	fmt.Println("=== 4. 提卡 + 幂等（同一订单号重试不重复扣卡）===")
	orderNo := fmt.Sprintf("SMOKE-%d", time.Now().Unix())
	r1, err := c.HandPick(ctx, agiso.HandPickInput{CpkID: firstUsable, Num: 1, OrderNo: orderNo})
	if err != nil {
		fmt.Println("  ✗ 提卡失败:", err)
		done(fail + 1)
	}
	check(len(r1.Cards) == 1, "提到 1 张卡")
	fmt.Printf("     发给买家的内容: %s\n", agiso.Render(d, r1.Cards[0]))

	r2, err := c.HandPick(ctx, agiso.HandPickInput{CpkID: firstUsable, Num: 1, OrderNo: orderNo})
	check(err == nil && len(r2.Cards) == 1 && r2.Cards[0].CardNo == r1.Cards[0].CardNo,
		"重复提卡返回同一张（幂等生效）", err)

	after, _ := c.ListCardKinds(ctx, 1, 100, "")
	for _, k := range after.List {
		if k.IdNo == firstUsable {
			var before int
			for _, b := range page.List {
				if b.IdNo == firstUsable {
					before = b.RemainingCount
				}
			}
			check(before-k.RemainingCount == 1, fmt.Sprintf("库存只扣 1 张（%d→%d）", before, k.RemainingCount))
		}
	}

	if !live {
		fmt.Println("=== 5. 故障注入（仅 mock 支持）===")
		for _, tc := range []struct {
			code string
			want error
			name string
		}{
			{"16", agiso.ErrInsufficientDeposit, "预存款不足 → 应告警"},
			{"1", agiso.ErrTokenInvalid, "授权失效 → 应告警"},
			{"2", agiso.ErrRateLimited, "触发限流 → 应退避重试"},
		} {
			_, _ = http.Post(base+"/__fault", "application/x-www-form-urlencoded", strings.NewReader("code="+tc.code))
			_, e := c.QueryDeposit(ctx)
			check(errors.Is(e, tc.want), tc.name, e)
		}
	}

	done(fail)
}

func done(fail int) {
	if fail > 0 {
		fmt.Printf("\n%d 项未通过\n", fail)
		os.Exit(1)
	}
	fmt.Println("\n全部通过")
	os.Exit(0)
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func hasFlag(f string) bool {
	for _, a := range os.Args[1:] {
		if a == f {
			return true
		}
	}
	return false
}
