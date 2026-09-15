package agiso

import "strings"

// Render 按卡种详情把一张卡拼成发给买家的文本。
//
// Agiso 的卡密不是「卡号+密码」直接拼起来就行：卡种上配了展示模板、卡号/密码前缀
// 和两者之间的分隔符，后台怎么配，买家就该看到什么。忽略这些字段会发出格式错误的卡密，
// 而这类错误买家拿到手才会发现，事后补救成本很高。
//
// 模板占位符用中文（官方示例为 "{卡号}"）。模板为空时回落到「前缀卡号 分隔符 前缀密码」，
// 密码为空则只输出卡号——重复卡这类卡种常常只有卡号。
func Render(d *CardKindDetail, c Card) string {
	cardNo := d.PrefixCardNo + c.CardNo
	pwd := c.Password
	if pwd != "" {
		pwd = d.PrefixPwd + pwd
	}

	if tpl := strings.TrimSpace(d.ShowTpl); tpl != "" {
		out := strings.ReplaceAll(tpl, "{卡号}", cardNo)
		out = strings.ReplaceAll(out, "{密码}", pwd)
		out = strings.ReplaceAll(out, "{到期时间}", c.Deadline)
		return out
	}

	if pwd == "" {
		return cardNo
	}
	sep := d.SplitCardPwd
	if sep == "" {
		sep = " "
	}
	return cardNo + sep + pwd
}

// NeedsRichRender 报告该卡种是否必须由前端渲染成条码或二维码。
// 返回 true 时把值当纯文本发给买家，对方是用不了的。
func NeedsRichRender(d *CardKindDetail) bool {
	return d.CardNoShowType != ShowTypeText || d.PwdShowType != ShowTypeText
}
