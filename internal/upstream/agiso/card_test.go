package agiso

import "testing"

func TestRenderUsesTemplate(t *testing.T) {
	d := &CardKindDetail{ShowTpl: "{卡号}", PrefixCardNo: "JD-"}
	got := Render(d, Card{CardNo: "abc", Password: "123"})
	if got != "JD-abc" {
		t.Fatalf("模板只要卡号时不应带出密码: %q", got)
	}
}

func TestRenderTemplateWithPasswordAndDeadline(t *testing.T) {
	d := &CardKindDetail{ShowTpl: "卡号：{卡号} 密码：{密码} 到期：{到期时间}"}
	got := Render(d, Card{CardNo: "a", Password: "p", Deadline: "2024-11-21 23:59:59"})
	want := "卡号：a 密码：p 到期：2024-11-21 23:59:59"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderFallsBackToSeparator(t *testing.T) {
	d := &CardKindDetail{SplitCardPwd: " | ", PrefixCardNo: "C", PrefixPwd: "P"}
	if got := Render(d, Card{CardNo: "1", Password: "2"}); got != "C1 | P2" {
		t.Fatalf("回落拼接不对: %q", got)
	}
}

// 重复卡这类卡种常常只有卡号没有密码，不能拼出一个尾随分隔符。
func TestRenderCardWithoutPassword(t *testing.T) {
	d := &CardKindDetail{SplitCardPwd: "----"}
	if got := Render(d, Card{CardNo: "only"}); got != "only" {
		t.Fatalf("无密码时应只输出卡号: %q", got)
	}
}

// 条码/二维码类卡种当文本发给买家是用不了的，必须能被识别出来。
func TestNeedsRichRender(t *testing.T) {
	if NeedsRichRender(&CardKindDetail{CardNoShowType: ShowTypeText, PwdShowType: ShowTypeText}) {
		t.Error("纯文本卡种不需要富渲染")
	}
	if !NeedsRichRender(&CardKindDetail{CardNoShowType: ShowTypeQRCode}) {
		t.Error("二维码卡号需要富渲染")
	}
	if !NeedsRichRender(&CardKindDetail{PwdShowType: ShowTypeBarcodeAndQR}) {
		t.Error("条码+二维码密码需要富渲染")
	}
}

// 白名单放行：已核实的唯一卡(800)支持，其余未核实类型一律 fail-closed。
func TestSupportedCardType(t *testing.T) {
	if !SupportedCardType(CardTypeUnique) {
		t.Errorf("唯一卡 %d 应当支持", CardTypeUnique)
	}
	for _, no := range []int{700, 810, 840, 850, 0} {
		if SupportedCardType(no) {
			t.Errorf("未核实类型 %d 不应放行", no)
		}
	}
}
