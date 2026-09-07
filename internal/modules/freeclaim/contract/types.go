package contract

import freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"

// ConfigInput 后台修改免费领取规则入参。
type ConfigInput struct {
	Enabled          bool
	MemberOnly       bool
	MinMemberLevelID uint
}

// PoolItemInput 后台维护商品池入参。
type PoolItemInput struct {
	ProductID uint
	SKUID     uint
	IsActive  bool
	SortOrder int
}

// TodayItem 今日可领取的商品。
type TodayItem struct {
	Date      string                    `json:"date"`
	Enabled    bool                      `json:"enabled"`
	Available  bool                      `json:"available"`
	Claimed    bool                      `json:"claimed"`
	MemberOnly bool                      `json:"member_only"`
	Eligible   bool                      `json:"eligible"`
	Item      *freeclaimdomain.PoolItem `json:"item,omitempty"`
	// Product 是展示用的商品快照。池条目本身只有 ID，
	// 首页卡片没有名称、图片与 slug 就无法渲染，也无法跳转到商品页。
	// 商品读取端口未注入或商品已下架时为 nil。
	Product *ProductView `json:"product,omitempty"`
}

// ProductView 免费领取卡片所需的商品展示字段。
// 只取展示必需的部分，不让 freeclaim 依赖 catalog 的领域类型。
type ProductView struct {
	ID    uint              `json:"id"`
	Slug  string            `json:"slug"`
	Title map[string]string `json:"title"`
	Image string            `json:"image"`
	// Price 是该 SKU 的原价，用于展示"原价 X，今日免费"。
	Price string `json:"price"`
}

// ClaimResult 领取结果。
type ClaimResult struct {
	Date      string `json:"date"`
	OrderNo   string `json:"order_no"`
	ProductID uint   `json:"product_id"`
	SKUID     uint   `json:"sku_id"`
}
