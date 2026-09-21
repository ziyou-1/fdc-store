package productapplication

import (
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
)

// ApplyAutoStockCounts 聚合卡密自动发货库存信息并填充到商品中。
//
// 交付方式已下沉到 SKU 级：只要商品下有任一 SKU 实际走自动发卡就参与聚合，
// 且只给走自动发卡的 SKU 算卡密/卡券库库存，人工、内容、上游 SKU 保持为 0。
func (s *Service) ApplyAutoStockCounts(products []productdomain.Product) error {
	var productIDs []uint
	for i := range products {
		if productdomain.ProductHasFulfillmentType(&products[i], constants.FulfillmentTypeAuto) {
			productIDs = append(productIDs, products[i].ID)
		}
	}
	if len(productIDs) == 0 {
		return nil
	}

	counts, err := s.stock.CountStockByProductIDs(productIDs)
	if err != nil {
		return err
	}

	// map[product_id]map[sku_id]map[status]total
	stockMap := make(map[uint]map[uint]map[string]int64)
	for _, count := range counts {
		if stockMap[count.ProductID] == nil {
			stockMap[count.ProductID] = make(map[uint]map[string]int64)
		}
		if stockMap[count.ProductID][count.SKUID] == nil {
			stockMap[count.ProductID][count.SKUID] = make(map[string]int64)
		}
		stockMap[count.ProductID][count.SKUID][count.Status] = count.Total
	}

	for i := range products {
		prod := &products[i]
		if !productdomain.ProductHasFulfillmentType(prod, constants.FulfillmentTypeAuto) {
			continue
		}
		pMap := stockMap[prod.ID]

		// 每个 SKU 按其「有效来源」独立算库存：agiso 来源用卡券库缓存，
		// 本地来源用本地卡密聚合。同一商品下两种来源可以混合。
		agisoProductAvailable := int64(prod.AgisoStockCached)
		if agisoProductAvailable < 0 {
			agisoProductAvailable = 0
		}
		// 回落 SKU（未单独绑卡种、共用商品级卡池）与本地历史卡密（skuID=0）的
		// 承载目标，只在各自来源的 SKU 中挑，避免混合时算错。
		agisoInheritTarget := resolveAgisoInheritTargetSKUIndex(prod)
		legacyTargetIdx := resolveLegacyStockTargetSKUIndex(prod)

		var pAvailable, pLocked, pUsed int64

		if len(prod.SKUs) == 0 {
			// 单规格：按商品级来源（走到这里说明商品级就是自动发卡）。
			if productdomain.EffectiveCardSource(prod, nil) == constants.CardSourceAgiso {
				pAvailable = agisoProductAvailable
			} else if pMap != nil {
				for _, statusMap := range pMap {
					pAvailable += statusMap[cardsecretdomain.StatusAvailable]
					pLocked += statusMap[cardsecretdomain.StatusReserved]
					pUsed += statusMap[cardsecretdomain.StatusUsed]
				}
			}
		} else {
			for j := range prod.SKUs {
				sku := &prod.SKUs[j]
				var available, locked, used int64
				if productdomain.EffectiveFulfillmentType(prod, sku) != constants.FulfillmentTypeAuto {
					// 非自动发卡的 SKU 没有卡密库存概念，保持 0，不参与商品级汇总。
					continue
				}
				if productdomain.EffectiveCardSource(prod, sku) == constants.CardSourceAgiso {
					// 卡券库来源：SKU 自己绑了卡种用自己的缓存，否则回落 SKU 承载商品级卡池。
					if sku.AgisoCpkID > 0 {
						available = int64(sku.AgisoStockCached)
						if available < 0 {
							available = 0
						}
					} else if j == agisoInheritTarget {
						available = agisoProductAvailable
					}
				} else if pMap != nil {
					// 本地来源：按本地卡密聚合。
					statusMap := pMap[sku.ID]
					available = statusMap[cardsecretdomain.StatusAvailable]
					locked = statusMap[cardsecretdomain.StatusReserved]
					used = statusMap[cardsecretdomain.StatusUsed]
					// skuID=0 的历史卡密归并到本地来源的承载 SKU。
					if j == legacyTargetIdx && pMap[0] != nil {
						available += pMap[0][cardsecretdomain.StatusAvailable]
						locked += pMap[0][cardsecretdomain.StatusReserved]
						used += pMap[0][cardsecretdomain.StatusUsed]
					}
				}
				sku.AutoStockAvailable = available
				sku.AutoStockTotal = available + locked
				sku.AutoStockLocked = locked
				sku.AutoStockSold = used
				pAvailable += available
				pLocked += locked
				pUsed += used
			}
		}

		prod.AutoStockAvailable = pAvailable
		prod.AutoStockTotal = pAvailable + pLocked
		prod.AutoStockLocked = pLocked
		prod.AutoStockSold = pUsed
	}
	return nil
}

// resolveAgisoInheritTargetSKUIndex 在「未单独绑定卡种、回落到商品级卡池」的活跃
// SKU 里挑一个承载商品级库存的目标：优先 DEFAULT，其次首个活跃回落 SKU。
// 若所有活跃 SKU 都各自绑了卡种（无人回落），返回 -1，商品级缓存不再展示。
func resolveAgisoInheritTargetSKUIndex(product *productdomain.Product) int {
	if product == nil {
		return -1
	}
	skus := product.SKUs
	defaultCode := strings.ToUpper(strings.TrimSpace(productdomain.DefaultSKUCode))
	firstActive := -1
	for idx := range skus {
		// 只在「走自动发卡、有效来源为 agiso、且未单独绑卡种（回落商品级卡池）」的活跃 SKU 中挑。
		if skus[idx].AgisoCpkID > 0 || !skus[idx].IsActive {
			continue
		}
		if productdomain.EffectiveFulfillmentType(product, &skus[idx]) != constants.FulfillmentTypeAuto {
			continue
		}
		if productdomain.EffectiveCardSource(product, &skus[idx]) != constants.CardSourceAgiso {
			continue
		}
		if firstActive < 0 {
			firstActive = idx
		}
		if strings.ToUpper(strings.TrimSpace(skus[idx].SKUCode)) == defaultCode {
			return idx
		}
	}
	return firstActive
}

func resolveLegacyStockTargetSKUIndex(product *productdomain.Product) int {
	if product == nil {
		return -1
	}
	skus := product.SKUs
	if len(skus) == 0 {
		return -1
	}

	defaultCode := strings.ToUpper(strings.TrimSpace(productdomain.DefaultSKUCode))
	firstActiveIdx := -1
	for idx := range skus {
		// 只在「走自动发卡且有效来源为本地」的活跃 SKU 中挑，避免把历史卡密归并到
		// agiso SKU 或人工/内容 SKU 上。
		if !skus[idx].IsActive {
			continue
		}
		if productdomain.EffectiveFulfillmentType(product, &skus[idx]) != constants.FulfillmentTypeAuto {
			continue
		}
		if productdomain.EffectiveCardSource(product, &skus[idx]) == constants.CardSourceAgiso {
			continue
		}
		if firstActiveIdx < 0 {
			firstActiveIdx = idx
		}
		if strings.ToUpper(strings.TrimSpace(skus[idx].SKUCode)) == defaultCode {
			return idx
		}
	}
	if firstActiveIdx >= 0 {
		return firstActiveIdx
	}

	// 防御性回退：没有本地来源的启用 SKU 时，仍尽量归并到走自动发卡的 DEFAULT SKU；
	// 商品下根本没有自动发卡 SKU 时返回 -1，历史卡密不再挂到任何规格上。
	for idx := range skus {
		if productdomain.EffectiveFulfillmentType(product, &skus[idx]) != constants.FulfillmentTypeAuto {
			continue
		}
		if productdomain.EffectiveCardSource(product, &skus[idx]) == constants.CardSourceAgiso {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(skus[idx].SKUCode)) == defaultCode {
			return idx
		}
	}
	for idx := range skus {
		if productdomain.EffectiveFulfillmentType(product, &skus[idx]) == constants.FulfillmentTypeAuto &&
			productdomain.EffectiveCardSource(product, &skus[idx]) != constants.CardSourceAgiso {
			return idx
		}
	}
	return -1
}
