package productdomain

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

const (
	// DefaultSKUCode 历史单规格商品迁移时使用的默认 SKU 编码
	DefaultSKUCode = "DEFAULT"
)

// ProductSKU 商品 SKU 表（v1：价格+库存维度）
type ProductSKU struct {
	ID                 uint         `gorm:"primarykey" json:"id"`                                                                       // 主键
	ProductID          uint         `gorm:"not null;index;uniqueIndex:idx_product_sku_code" json:"product_id"`                          // 商品ID
	SKUCode            string       `gorm:"column:sku_code;type:varchar(64);not null;uniqueIndex:idx_product_sku_code" json:"sku_code"` // SKU编码（同商品内唯一）
	SpecValuesJSON     jsonmap.JSON `gorm:"type:json" json:"spec_values"`                                                               // 规格值（如颜色/版本）
	PriceAmount        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`                                  // SKU价格
	CostPriceAmount    money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"cost_price_amount"`                             // 成本价
	PointsPrice        int64        `gorm:"not null;default:0" json:"points_price"`                                                     // 积分兑换价（0 表示该 SKU 不支持积分兑换）
	ManualStockTotal   int          `gorm:"not null;default:0" json:"manual_stock_total"`                                               // 手动剩余库存（-1 表示无限库存，>=0 表示当前可售数量）
	ManualStockLocked  int          `gorm:"not null;default:0" json:"manual_stock_locked"`                                              // 手动库存占用量（待支付）
	ManualStockSold    int          `gorm:"not null;default:0" json:"manual_stock_sold"`                                                // 手动库存已售量（支付成功后累加）
	// CardSource 为该 SKU 单独指定的卡密来源（local / agiso）。
	// 空串表示「跟随商品级」，保证既有 SKU 无需迁移。允许同一商品下
	// 不同 SKU 混合来源：A 规格走卡券库、B 规格用本地导入的卡密。
	CardSource         string       `gorm:"type:varchar(20);not null;default:''" json:"card_source"`
	// InstructionsJSON 为该 SKU 单独的交付说明（多语言）。为空则回落商品级说明。
	InstructionsJSON   jsonmap.JSON `gorm:"type:json" json:"instructions"`
	// 以下 Agiso 字段仅在（该 SKU 的有效来源为）agiso 时有意义：允许每个 SKU 绑定不同卡种。
	// cpk 为 0 表示该 SKU 不单独绑定，回落到商品级卡种（保证老的单卡种商品无需迁移）。
	AgisoConnectionID  uint         `gorm:"index" json:"agiso_connection_id"`                                                           // SKU 级卡券库连接（0 表示继承商品级）
	AgisoCpkID         int64        `json:"agiso_cpk_id"`                                                                               // SKU 级 Agiso 卡种 Id（0 表示继承商品级）
	AgisoStockCached   int          `gorm:"not null;default:0" json:"agiso_stock_cached"`                                               // SKU 级卡券库库存缓存（由定时任务刷新）
	AgisoStockSyncedAt *time.Time   `json:"agiso_stock_synced_at,omitempty"`                                                            // SKU 级库存缓存刷新时间
	AutoStockAvailable int64        `gorm:"-" json:"auto_stock_available"`                                                              // 自动发货库存可用量（仅结构，不写入数据库）
	AutoStockTotal     int64        `gorm:"-" json:"auto_stock_total"`                                                                  // 自动发货库存总量（仅结构，不写入数据库）
	AutoStockLocked    int64        `gorm:"-" json:"auto_stock_locked"`                                                                 // 自动发货库存占用量（仅结构，不写入数据库）
	AutoStockSold      int64        `gorm:"-" json:"auto_stock_sold"`                                                                   // 自动发货库存已售量（仅结构，不写入数据库）
	UpstreamStock      int          `gorm:"-" json:"upstream_stock"`                                                                    // 上游库存（-1=无限, 0=售罄, >0=有货；仅结构，不写入数据库）
	IsActive           bool         `gorm:"default:true;index" json:"is_active"`                                                        // 是否启用
	SortOrder          int          `gorm:"default:0;index" json:"sort_order"`                                                          // 排序权重
	CreatedAt          time.Time    `gorm:"index" json:"created_at"`                                                                    // 创建时间
	UpdatedAt          time.Time    `gorm:"index" json:"updated_at"`                                                                    // 更新时间
	DeletedAt          *time.Time   `gorm:"index" json:"-"`                                                                             // 软删除时间

	Product *Product `gorm:"foreignKey:ProductID" json:"product,omitempty"` // 关联商品
}

// TableName 指定表名
func (ProductSKU) TableName() string {
	return "product_skus"
}

// EffectiveAgisoBinding 返回该 SKU 实际提卡用的连接与卡种。
// SKU 自己配了卡种就用 SKU 的（连接缺省则继承商品级）；没配则整体回落到商品级。
// 这样单卡种商品和历史数据（SKU cpk 为 0）无需迁移即可继续工作。
func EffectiveAgisoBinding(product *Product, sku *ProductSKU) (connID uint, cpkID int64) {
	var pConn uint
	var pCpk int64
	if product != nil {
		pConn = product.AgisoConnectionID
		pCpk = product.AgisoCpkID
	}
	if sku == nil {
		return pConn, pCpk
	}
	return ResolveAgisoBinding(pConn, pCpk, sku.AgisoConnectionID, sku.AgisoCpkID)
}

// EffectiveCardSource 返回该 SKU 实际生效的卡密来源。
// SKU 自己配了来源（非空）就用 SKU 的；否则回落到商品级 card_source。
// 传入的原始值统一经 NormalizeCardSource 归一化，非法值按 local 处理。
func EffectiveCardSource(product *Product, sku *ProductSKU) string {
	if sku != nil {
		if raw := strings.TrimSpace(sku.CardSource); raw != "" {
			return NormalizeCardSource(raw)
		}
	}
	if product != nil {
		return NormalizeCardSource(product.CardSource)
	}
	return constants.CardSourceLocal
}

// EffectiveInstructions 返回该 SKU 实际展示的交付说明。
// SKU 自己填了说明就用 SKU 的，否则回落商品级说明。
func EffectiveInstructions(product *Product, sku *ProductSKU) jsonmap.JSON {
	if sku != nil && len(sku.InstructionsJSON) > 0 {
		return sku.InstructionsJSON
	}
	if product != nil {
		return product.InstructionsJSON
	}
	return nil
}
