package productdomain

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
)

func hasMultipleActiveSKUs(product *Product) bool {
	if product == nil || len(product.SKUs) == 0 {
		return false
	}
	activeCount := 0
	for i := range product.SKUs {
		if !product.SKUs[i].IsActive {
			continue
		}
		activeCount++
		if activeCount > 1 {
			return true
		}
	}
	return false
}

// ManualSKUAvailable 返回 SKU 当前可用于规则判断的手工库存；无限库存映射为最大 int。
func ManualSKUAvailable(sku *ProductSKU) int {
	if sku == nil {
		return 0
	}
	if sku.ManualStockTotal == constants.ManualStockUnlimited {
		return int(^uint(0) >> 1)
	}
	if sku.ManualStockTotal < 0 {
		return 0
	}
	return sku.ManualStockTotal
}

// ShouldEnforceManualSKUStock 判断当前 SKU 是否需要执行手工库存约束。
func ShouldEnforceManualSKUStock(product *Product, sku *ProductSKU) bool {
	if product == nil || sku == nil {
		return false
	}
	if sku.ManualStockTotal == constants.ManualStockUnlimited {
		return false
	}
	if sku.ManualStockTotal >= 0 {
		return true
	}
	if strings.ToUpper(strings.TrimSpace(sku.SKUCode)) != DefaultSKUCode {
		return true
	}
	return hasMultipleActiveSKUs(product)
}

// SKUStockQuantity 按 SKU 实际生效的交付方式返回可用量；负数表示无限。
//
//	auto     ：卡密/卡券库可用量（须先由 ApplyAutoStockCounts 填好 AutoStockAvailable）
//	manual   ：人工库存（-1 无限）
//	upstream ：SKU 映射的上游库存（须先由调用方写好 UpstreamStock；-1 无限）
//	content  ：恒为无限
func SKUStockQuantity(product *Product, sku *ProductSKU) int64 {
	if sku == nil {
		return 0
	}
	switch EffectiveFulfillmentType(product, sku) {
	case constants.FulfillmentTypeAuto:
		return sku.AutoStockAvailable
	case constants.FulfillmentTypeContent:
		return int64(constants.ManualStockUnlimited)
	case constants.FulfillmentTypeUpstream:
		if sku.UpstreamStock < 0 {
			return int64(constants.ManualStockUnlimited)
		}
		return int64(sku.UpstreamStock)
	default:
		if sku.ManualStockTotal == constants.ManualStockUnlimited {
			return int64(constants.ManualStockUnlimited)
		}
		if sku.ManualStockTotal < 0 {
			return 0
		}
		return int64(sku.ManualStockTotal)
	}
}

// ProductStockQuantity 汇总商品下所有活跃 SKU 的可用量：任一无限则返回 -1，否则求和。
// 没有活跃 SKU 时按商品级交付方式取商品级字段（兼容没有 SKU 行的历史数据）。
// 交付方式已下沉到 SKU 级，混合交付的商品只能这样按 SKU 逐个算。
func ProductStockQuantity(product *Product) int64 {
	if product == nil {
		return 0
	}
	activeCount := 0
	var total int64
	for i := range product.SKUs {
		if !product.SKUs[i].IsActive {
			continue
		}
		activeCount++
		quantity := SKUStockQuantity(product, &product.SKUs[i])
		if quantity < 0 {
			return int64(constants.ManualStockUnlimited)
		}
		total += quantity
	}
	if activeCount > 0 {
		return total
	}
	switch NormalizeFulfillmentType(product.FulfillmentType) {
	case constants.FulfillmentTypeAuto:
		return product.AutoStockAvailable
	case constants.FulfillmentTypeContent:
		return int64(constants.ManualStockUnlimited)
	case constants.FulfillmentTypeUpstream:
		return 0
	default:
		if product.ManualStockTotal == constants.ManualStockUnlimited {
			return int64(constants.ManualStockUnlimited)
		}
		if product.ManualStockTotal < 0 {
			return 0
		}
		return int64(product.ManualStockTotal)
	}
}
