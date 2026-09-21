package producthttp

import (
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	domaincatalog "github.com/dujiao-next/internal/modules/catalog"
)

// decorateProductStock 按「每个 SKU 各自的交付方式」计算库存并汇总到商品级。
//
// 交付方式已下沉到 SKU 级：同一商品下自动发卡、人工交付、上游代销、纯内容可以混合，
// 因此不能再按商品级交付方式一刀切。规则：
//   - auto     ：卡密/卡券库可用量（ApplyAutoStockCounts 已算好在 SKU 上）
//   - manual   ：人工库存（-1 无限）
//   - upstream ：SKU 映射的上游库存（-1 无限；无映射记录降级为无限，避免误售罄）
//   - content  ：恒为无限
//
// 商品级：任一活跃 SKU 无限则商品无限；否则各 SKU 可用量求和。
// 没有活跃 SKU 时退化为商品级交付方式（兼容没有 SKU 行的历史数据）。
func (h *PublicHandler) decorateProductStock(product *productdomain.Product, item *publicProductView) {
	if product == nil || item == nil {
		return
	}

	item.ManualStockAvailable = 0
	item.AutoStockTotal = 0
	item.AutoStockLocked = 0
	item.AutoStockSold = 0
	item.AutoStockAvailable = 0
	item.StockQuantity = 0
	item.StockStatus = constants.ProductStockStatusInStock
	item.IsSoldOut = false
	item.UpstreamDisplayType = ""
	item.OriginalFulfillmentType = product.FulfillmentType

	// 对接商品：先把上游库存写到各 SKU 的 UpstreamStock 上，并解析上游展示类型。
	// 非对接商品不会有 upstream SKU（EffectiveFulfillmentType 已兜住），不必查映射。
	if product.IsMapped || productdomain.NormalizeFulfillmentType(product.FulfillmentType) == constants.FulfillmentTypeUpstream {
		h.applyUpstreamSKUStock(product, item)
	}

	activeCount := 0
	unlimited := false
	var total int64
	var autoAvailable, autoTotal, autoLocked, autoSold int64
	var manualAvailable int64
	for i := range item.Product.SKUs {
		sku := &item.Product.SKUs[i]
		if !sku.IsActive {
			continue
		}
		activeCount++
		fulfillmentType := productdomain.EffectiveFulfillmentType(product, sku)
		quantity := publicSKUStockQuantity(fulfillmentType, sku)
		switch fulfillmentType {
		case constants.FulfillmentTypeAuto:
			autoAvailable += sku.AutoStockAvailable
			autoTotal += sku.AutoStockTotal
			autoLocked += sku.AutoStockLocked
			autoSold += sku.AutoStockSold
		case constants.FulfillmentTypeUpstream:
			// 按上游展示类型把库存写回对应字段，让买家端沿用 auto/manual 的库存判断。
			if item.UpstreamDisplayType == constants.FulfillmentTypeAuto {
				sku.AutoStockAvailable = quantity
				if quantity >= 0 {
					autoAvailable += quantity
				}
			} else {
				if quantity < 0 {
					sku.ManualStockTotal = constants.ManualStockUnlimited
				} else {
					sku.ManualStockTotal = int(quantity)
					manualAvailable += quantity
				}
			}
		case constants.FulfillmentTypeContent:
			// 纯内容没有库存概念
		default:
			if quantity >= 0 {
				manualAvailable += quantity
			}
		}
		if quantity < 0 {
			unlimited = true
		} else {
			total += quantity
		}
	}

	if activeCount == 0 {
		h.decorateProductStockWithoutSKUs(product, item)
		return
	}

	item.AutoStockAvailable = autoAvailable
	item.AutoStockTotal = autoTotal
	item.AutoStockLocked = autoLocked
	item.AutoStockSold = autoSold
	item.ManualStockAvailable = int(manualAvailable)

	if unlimited {
		item.StockQuantity = int64(constants.ManualStockUnlimited)
		item.ManualStockAvailable = constants.ManualStockUnlimited
		item.StockStatus = constants.ProductStockStatusUnlimited
		item.IsSoldOut = false
		return
	}
	item.StockQuantity = total
	item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(total)
	item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
}

// publicSKUStockQuantity 返回 SKU 按其交付方式的可用量；负数表示无限。
func publicSKUStockQuantity(fulfillmentType string, sku *productdomain.ProductSKU) int64 {
	if sku == nil {
		return 0
	}
	switch fulfillmentType {
	case constants.FulfillmentTypeAuto:
		return sku.AutoStockAvailable
	case constants.FulfillmentTypeContent:
		return int64(constants.ManualStockUnlimited)
	case constants.FulfillmentTypeUpstream:
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

// decorateProductStockWithoutSKUs 兼容没有活跃 SKU 行的商品：按商品级交付方式算库存。
func (h *PublicHandler) decorateProductStockWithoutSKUs(product *productdomain.Product, item *publicProductView) {
	switch productdomain.NormalizeFulfillmentType(product.FulfillmentType) {
	case constants.FulfillmentTypeContent:
		item.StockQuantity = int64(constants.ManualStockUnlimited)
		item.ManualStockAvailable = constants.ManualStockUnlimited
		item.StockStatus = constants.ProductStockStatusUnlimited
		item.IsSoldOut = false
	case constants.FulfillmentTypeUpstream:
		// 没有任何活跃 SKU 映射的对接商品视作售罄。
		item.StockQuantity = 0
		item.StockStatus = constants.ProductStockStatusOutOfStock
		item.IsSoldOut = true
	case constants.FulfillmentTypeAuto:
		item.AutoStockAvailable = product.AutoStockAvailable
		item.AutoStockTotal = product.AutoStockTotal
		item.AutoStockLocked = product.AutoStockLocked
		item.AutoStockSold = product.AutoStockSold
		item.StockQuantity = product.AutoStockAvailable
		item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(product.AutoStockAvailable)
		item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
	default:
		if product.ManualStockTotal == constants.ManualStockUnlimited {
			item.StockQuantity = int64(constants.ManualStockUnlimited)
			item.ManualStockAvailable = constants.ManualStockUnlimited
			item.StockStatus = constants.ProductStockStatusUnlimited
			item.IsSoldOut = false
			return
		}
		manualAvailable := product.ManualStockTotal
		if manualAvailable < 0 {
			manualAvailable = 0
		}
		item.ManualStockAvailable = manualAvailable
		item.StockQuantity = int64(manualAvailable)
		item.StockStatus = domaincatalog.StorefrontStockPolicy().Status(int64(manualAvailable))
		item.IsSoldOut = item.StockStatus == constants.ProductStockStatusOutOfStock
	}
}

// applyUpstreamSKUStock 把 SKU 映射里的上游库存写到各 SKU 的 UpstreamStock 上，
// 并解析上游展示类型（auto/manual）写到 item.UpstreamDisplayType；
// 商品级交付方式为 upstream 时同步替换为展示类型（后台/买家端只认 auto/manual）。
//
// 降级语义：没有商品映射记录时所有 SKU 视作无限库存（避免脏数据误售罄）；
// 有商品映射但某 SKU 没有映射或上游已停用时该 SKU 视作 0。
func (h *PublicHandler) applyUpstreamSKUStock(product *productdomain.Product, item *publicProductView) {
	displayType := constants.FulfillmentTypeManual
	var skuMappingByLocal map[uint]*mappingdomain.SKUMapping

	mapping, err := h.mappings.GetByLocalProductID(product.ID)
	if err == nil && mapping != nil {
		if mapping.UpstreamFulfillmentType == constants.FulfillmentTypeAuto {
			displayType = constants.FulfillmentTypeAuto
		}
		skuMappings, listErr := h.skuMappings.ListByProductMapping(mapping.ID)
		if listErr == nil {
			skuMappingByLocal = make(map[uint]*mappingdomain.SKUMapping, len(skuMappings))
			for i := range skuMappings {
				skuMappingByLocal[skuMappings[i].LocalSKUID] = &skuMappings[i]
			}
		}
	}
	item.UpstreamDisplayType = displayType
	if productdomain.NormalizeFulfillmentType(item.Product.FulfillmentType) == constants.FulfillmentTypeUpstream {
		item.Product.FulfillmentType = displayType
	}

	for i := range item.Product.SKUs {
		sku := &item.Product.SKUs[i]
		if productdomain.EffectiveFulfillmentType(product, sku) != constants.FulfillmentTypeUpstream {
			continue
		}
		if mapping == nil {
			sku.UpstreamStock = -1
			continue
		}
		sm, ok := skuMappingByLocal[sku.ID]
		if !ok || !sm.UpstreamIsActive {
			sku.UpstreamStock = 0
			continue
		}
		sku.UpstreamStock = sm.UpstreamStock
	}
}
