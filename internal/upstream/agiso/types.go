package agiso

// 卡种类型（Agiso GetList/GetDetails 的 CardType 字段）。
//
// 已从实盘核实：800 = 唯一卡，是 Agiso 建卡种时的默认类型，也是目前最常见的类型——
// 纯文本展示、每张卡唯一、提卡即扣库存，可安全自动发货。
//
// 注意：此前这里写的 700/820/840/850 是臆造的错误编码，曾把整库的 800 卡误判成
// 「套卡·不支持」，导致后台卡种下拉全部置灰。套卡/图片卡/循环卡/重复卡的真实数字码
// 还没从实盘取样确认过，确认前一律 fail-closed（不放行），宁可挡住也不错发。
const (
	CardTypeUnique = 800 // 唯一卡（已核实）
)

// SupportedCardType 报告该卡种能否安全地接入自动发卡。
// 白名单放行：只认已核实安全的类型，未知类型一律不放行（fail-closed）。
func SupportedCardType(t int) bool {
	return t == CardTypeUnique
}

// 卡号/密码的展示类型。非文本类型需要前端渲染成条码或二维码，
// 直接把值当文本发给买家会导致对方无法使用。
const (
	ShowTypeText         = 0
	ShowTypeBarcode      = 1
	ShowTypeQRCode       = 2
	ShowTypeBarcodeAndQR = 4
)

// envelope 是所有接口的统一响应外壳。
type envelope struct {
	IsSuccess  bool   `json:"IsSuccess"`
	ErrorCode  int    `json:"Error_Code"`
	ErrorMsg   string `json:"Error_Msg"`
	AllowRetry *bool  `json:"AllowRetry"`
	RequestID  string `json:"RequestId"`
}

// CardKind 卡种（对应 GetList 列表项）。
type CardKind struct {
	IdNo           int64   `json:"IdNo"`           // 卡种 Id，提卡时作为 cpkId
	CardType       int     `json:"CardType"`       // 卡种类型
	Title          string  `json:"Title"`          // 卡种名称
	RemainingCount int     `json:"RemainingCount"` // 剩余库存
	UsedCount      int     `json:"UsedCount"`
	TotalCount     int     `json:"TotalCount"`
	CreateTime     string  `json:"CreateTime"`
	PurPrice       float64 `json:"PurPrice"` // 成本单价
}

// CardKindPage GetList 的分页结果。
type CardKindPage struct {
	List       []CardKind `json:"List"`
	TotalCount int        `json:"TotalCount"`
	PageNo     int        `json:"PageNo"`
	PageSize   int        `json:"PageSize"`
}

// CardKindDetail 卡种详情（GetDetails）。
// 展示相关的四个字段决定了卡密最终怎么拼给买家，缺一不可。
type CardKindDetail struct {
	IdNo           int64   `json:"IdNo"`
	Title          string  `json:"Title"`
	AliasName      string  `json:"AliasName"`
	ClassId        int64   `json:"ClassId"`
	Sort           int     `json:"Sort"`
	WarningNum     int     `json:"WarningNum"` // 库存预警阈值，低于它 Agiso 会推送通知
	TotalCount     int     `json:"TotalCount"`
	UsedCount      int     `json:"UsedCount"`
	CardType       int     `json:"CardType"`
	CardNoShowType int     `json:"CardNoShowType"`
	PwdShowType    int     `json:"PwdShowType"`
	ShowTpl        string  `json:"ShowTpl"`      // 展示模板，如 "{卡号}"
	PrefixCardNo   string  `json:"PrefixCardNo"` // 卡号前缀
	PrefixPwd      string  `json:"PrefixPwd"`    // 密码前缀
	SplitCardPwd   string  `json:"SplitCardPwd"` // 卡号与密码的分隔符
	PurPrice       float64 `json:"PurPrice"`
	ModifyTime     string  `json:"ModifyTime"`
	CreateTime     string  `json:"CreateTime"`
}

// Card 一张卡密。字段名在 Agiso 的响应里是极短的 c/p/d。
type Card struct {
	CardNo   string `json:"c"` // 卡号
	Password string `json:"p"` // 密码
	Deadline string `json:"d"` // 到期时间
}

// HandPickResult 提卡结果。
// buildCpd=false 返回 Cards；buildCpd=true 返回一个卡券提取链接。
type HandPickResult struct {
	Cards  []Card `json:"CardPwdArr"`
	CpdURL string `json:"CpdUrl"`
}

// Deposit 开放平台预存款余额。
// 余额耗尽会触发错误码 16、全站停止发卡，因此必须定时巡检并告警。
type Deposit struct {
	Balance float64 `json:"Balance"`
}

// StockWarnNotice 库存预警推送的载荷。
// 由 Agiso 主动 POST 到应用配置的推送地址，可省掉一部分主动轮询。
type StockWarnNotice struct {
	AliasName      string `json:"AliasName"`
	CpkName        string `json:"CpkName"`
	CpkId          int64  `json:"CpkId"`
	WarningNum     int    `json:"WarningNum"`
	RemainingCount int    `json:"RemainingCount"`
	CreateTime     string `json:"CreateTime"`
}
