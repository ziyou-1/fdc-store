package productwrite

import (
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func (s *WriteService) syncSingleProductSKU(skuRepo SKURepository, productID uint, priceAmount decimal.Decimal, costPriceAmount decimal.Decimal, pointsPrice int64, manualStockTotal int, createWhenMissing bool) error {
	if skuRepo == nil || productID == 0 {
		return nil
	}
	skus, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	if len(skus) == 0 {
		if !createWhenMissing {
			return nil
		}
		return skuRepo.Create(&productdomain.ProductSKU{
			ProductID:         productID,
			SKUCode:           productdomain.DefaultSKUCode,
			SpecValuesJSON:    jsonmap.JSON{},
			PriceAmount:       money.FromDecimal(priceAmount),
			CostPriceAmount:   money.FromDecimal(costPriceAmount),
			PointsPrice:       pointsPrice,
			ManualStockTotal:  manualStockTotal,
			ManualStockLocked: 0,
			ManualStockSold:   0,
			IsActive:          true,
			SortOrder:         0,
		})
	}
	targetIndex := pickSingleModeTargetSKUIndex(skus)
	if targetIndex < 0 || targetIndex >= len(skus) {
		return productcontract.ErrProductSKUInvalid
	}

	target := skus[targetIndex]
	target.PriceAmount = money.FromDecimal(priceAmount)
	target.CostPriceAmount = money.FromDecimal(costPriceAmount)
	target.PointsPrice = pointsPrice
	target.ManualStockTotal = manualStockTotal
	target.IsActive = true
	if strings.TrimSpace(target.SKUCode) == "" {
		target.SKUCode = productdomain.DefaultSKUCode
	}
	if err := skuRepo.Update(&target); err != nil {
		return err
	}

	for i := range skus {
		if i == targetIndex {
			continue
		}
		if err := skuRepo.Delete(skus[i].ID); err != nil {
			return err
		}
	}
	return nil
}

func pickSingleModeTargetSKUIndex(skus []productdomain.ProductSKU) int {
	if len(skus) == 0 {
		return -1
	}
	defaultCode := strings.ToUpper(strings.TrimSpace(productdomain.DefaultSKUCode))

	for i := range skus {
		if !skus[i].IsActive {
			continue
		}
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	for i := range skus {
		if skus[i].IsActive {
			return i
		}
	}
	for i := range skus {
		if strings.ToUpper(strings.TrimSpace(skus[i].SKUCode)) == defaultCode {
			return i
		}
	}
	return 0
}

type normalizedProductSKU struct {
	ID                uint
	SKUCode           string
	SpecValuesJSON    jsonmap.JSON
	PriceAmount       money.Amount
	CostPriceAmount   money.Amount
	PointsPrice       int64
	ManualStockTotal  int
	CardSource        string // 该 SKU 单独来源（local/agiso）；空=跟随商品级
	InstructionsJSON  jsonmap.JSON
	AgisoConnectionID uint
	AgisoCpkID        int64
	IsActive          bool
	SortOrder         int
}

func (s *WriteService) normalizeProductSKUInputs(inputs []ProductSKUInput, fulfillmentType, cardSource string, existingSKUMap map[uint]productdomain.ProductSKU) ([]normalizedProductSKU, decimal.Decimal, int, error) {
	if len(inputs) == 0 {
		return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
	}
	seenCode := make(map[string]struct{}, len(inputs))
	normalized := make([]normalizedProductSKU, 0, len(inputs))
	hasActive := false
	minActivePrice := decimal.Zero
	manualStockTotal := 0
	hasUnlimitedStock := false

	for _, input := range inputs {
		skuCode := strings.TrimSpace(input.SKUCode)
		if skuCode == "" {
			return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
		}
		codeKey := strings.ToLower(skuCode)
		if _, exists := seenCode[codeKey]; exists {
			return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
		}
		seenCode[codeKey] = struct{}{}

		priceAmount := input.PriceAmount.Round(2)
		if priceAmount.LessThanOrEqual(decimal.Zero) {
			return nil, decimal.Zero, 0, productcontract.ErrProductPriceInvalid
		}
		costPriceAmount := input.CostPriceAmount.Round(2)
		if costPriceAmount.LessThan(decimal.Zero) {
			return nil, decimal.Zero, 0, productcontract.ErrProductPriceInvalid
		}

		manualTotal := input.ManualStockTotal
		if manualTotal < constants.ManualStockUnlimited {
			return nil, decimal.Zero, 0, productcontract.ErrManualStockInvalid
		}
		if fulfillmentType != constants.FulfillmentTypeManual {
			manualTotal = 0
		}
		if existingSKUMap != nil && input.ID > 0 {
			_, ok := existingSKUMap[input.ID]
			if !ok {
				return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
			}
		}

		isActive := true
		if input.IsActive != nil {
			isActive = *input.IsActive
		}
		specValues := jsonmap.JSON{}
		if input.SpecValuesJSON != nil {
			specValues = jsonmap.JSON(input.SpecValuesJSON)
		}

		// 该 SKU 单独指定的卡密来源：local / agiso / 空（跟随商品级）。
		// 非法值一律按空处理（跟随商品级），保证既有数据行为不变。
		skuCardSource := strings.ToLower(strings.TrimSpace(input.CardSource))
		if skuCardSource != constants.CardSourceLocal && skuCardSource != constants.CardSourceAgiso {
			skuCardSource = ""
		}
		// 有效来源 = SKU 自己选的（非空）否则商品级。用于决定是否保留 agiso 绑定。
		effectiveSource := skuCardSource
		if effectiveSource == "" {
			effectiveSource = cardSource
		}

		// SKU 级卡种绑定只在该 SKU 有效来源为 agiso 时保留；切回本地时一并清空，
		// 否则残留的卡种会让提卡/库存逻辑误以为还接着卡券库。
		var agisoConnID uint
		var agisoCpkID int64
		if effectiveSource == constants.CardSourceAgiso {
			if input.AgisoConnectionID != nil {
				agisoConnID = *input.AgisoConnectionID
			}
			if input.AgisoCpkID != nil && *input.AgisoCpkID > 0 {
				agisoCpkID = *input.AgisoCpkID
			}
		}

		var skuInstructions jsonmap.JSON
		if input.InstructionsJSON != nil {
			skuInstructions = jsonmap.JSON(input.InstructionsJSON)
		}

		normalized = append(normalized, normalizedProductSKU{
			ID:                input.ID,
			SKUCode:           skuCode,
			SpecValuesJSON:    specValues,
			PriceAmount:       money.FromDecimal(priceAmount),
			CostPriceAmount:   money.FromDecimal(costPriceAmount),
			PointsPrice:       input.PointsPrice,
			ManualStockTotal:  manualTotal,
			CardSource:        skuCardSource,
			InstructionsJSON:  skuInstructions,
			AgisoConnectionID: agisoConnID,
			AgisoCpkID:        agisoCpkID,
			IsActive:          isActive,
			SortOrder:         input.SortOrder,
		})

		if isActive {
			if !hasActive || priceAmount.LessThan(minActivePrice) {
				minActivePrice = priceAmount
			}
			hasActive = true
			if fulfillmentType == constants.FulfillmentTypeManual {
				if manualTotal == constants.ManualStockUnlimited {
					hasUnlimitedStock = true
				} else {
					manualStockTotal += manualTotal
				}
			}
		}
	}

	if !hasActive {
		return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
	}
	if fulfillmentType != constants.FulfillmentTypeManual {
		manualStockTotal = 0
	} else if hasUnlimitedStock {
		manualStockTotal = constants.ManualStockUnlimited
	}
	return normalized, minActivePrice, manualStockTotal, nil
}

// validateAgisoSKUBindings 校验卡券库商品每个 SKU 的有效绑定是否完整。
//
// 卡种绑定已下沉到 SKU 级：SKU 自己配了卡种就用 SKU 的，没配则回落到商品级。
// 因此完整性要按「每个活跃 SKU 的有效绑定」逐个判，而不是只看商品级那一份。
// 没有显式 SKU（单规格商品）时，商品级绑定就是唯一那份，直接校验它。
func validateAgisoSKUBindings(cardSource string, productConnID uint, productCpkID int64, skus []normalizedProductSKU) error {
	// 单规格（无显式 SKU）：商品级来源就是唯一那份。
	if len(skus) == 0 {
		if cardSource != constants.CardSourceAgiso {
			return nil
		}
		return productdomain.ValidateAgisoBinding(productConnID, productCpkID)
	}
	// 多规格：逐个按「该 SKU 的有效来源」判。来源为 agiso 的 SKU 才要求绑定完整，
	// 因此同一商品下本地 SKU 与卡券库 SKU 可以共存。
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		effectiveSource := sku.CardSource
		if effectiveSource == "" {
			effectiveSource = cardSource
		}
		if effectiveSource != constants.CardSourceAgiso {
			continue
		}
		connID, cpkID := productdomain.ResolveAgisoBinding(productConnID, productCpkID, sku.AgisoConnectionID, sku.AgisoCpkID)
		if err := productdomain.ValidateAgisoBinding(connID, cpkID); err != nil {
			return err
		}
	}
	return nil
}

// minActiveCostPrice 从已标准化的 SKU 列表中取最低活跃 SKU 的成本价
func minActiveCostPrice(skus []normalizedProductSKU) decimal.Decimal {
	first := true
	min := decimal.Zero
	for _, s := range skus {
		if !s.IsActive {
			continue
		}
		d := s.CostPriceAmount.Decimal
		if first || d.LessThan(min) {
			min = d
			first = false
		}
	}
	return min
}

func (s *WriteService) applyProductSKUsWithStockGuard(
	skuRepo SKURepository,
	cardSecretRepo CardSecretStockRepository,
	productID uint,
	fulfillmentType string,
	rows []normalizedProductSKU,
) error {
	if skuRepo == nil || productID == 0 || len(rows) == 0 {
		return productcontract.ErrProductSKUInvalid
	}
	existingRows, err := skuRepo.ListByProduct(productID, false)
	if err != nil {
		return err
	}
	existingByID := make(map[uint]productdomain.ProductSKU, len(existingRows))
	existingByCode := make(map[string]productdomain.ProductSKU, len(existingRows))
	for _, row := range existingRows {
		existingByID[row.ID] = row
		existingByCode[strings.ToLower(strings.TrimSpace(row.SKUCode))] = row
	}
	if err := s.ensureAutoSKUCardSecretStockSafe(cardSecretRepo, productID, fulfillmentType, existingRows, rows, existingByID, existingByCode); err != nil {
		return err
	}

	kept := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return productcontract.ErrProductSKUInvalid
			}
			existing.SKUCode = row.SKUCode
			existing.SpecValuesJSON = row.SpecValuesJSON
			existing.PriceAmount = row.PriceAmount
			existing.CostPriceAmount = row.CostPriceAmount
			existing.PointsPrice = row.PointsPrice
			existing.ManualStockTotal = row.ManualStockTotal
			existing.CardSource = row.CardSource
			existing.InstructionsJSON = row.InstructionsJSON
			existing.AgisoConnectionID = row.AgisoConnectionID
			existing.AgisoCpkID = row.AgisoCpkID
			existing.IsActive = row.IsActive
			existing.SortOrder = row.SortOrder
			if err := skuRepo.Update(&existing); err != nil {
				return err
			}
			kept[existing.ID] = struct{}{}
			existingByCode[strings.ToLower(strings.TrimSpace(existing.SKUCode))] = existing
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			existing.SpecValuesJSON = row.SpecValuesJSON
			existing.PriceAmount = row.PriceAmount
			existing.CostPriceAmount = row.CostPriceAmount
			existing.PointsPrice = row.PointsPrice
			existing.ManualStockTotal = row.ManualStockTotal
			existing.CardSource = row.CardSource
			existing.InstructionsJSON = row.InstructionsJSON
			existing.AgisoConnectionID = row.AgisoConnectionID
			existing.AgisoCpkID = row.AgisoCpkID
			existing.IsActive = row.IsActive
			existing.SortOrder = row.SortOrder
			if err := skuRepo.Update(&existing); err != nil {
				return err
			}
			kept[existing.ID] = struct{}{}
			continue
		}

		// 清理同 sku_code 的软删除残留，避免唯一索引冲突
		if err := skuRepo.PurgeSoftDeletedByProductAndCode(productID, row.SKUCode); err != nil {
			return err
		}
		item := productdomain.ProductSKU{
			ProductID:         productID,
			SKUCode:           row.SKUCode,
			SpecValuesJSON:    row.SpecValuesJSON,
			PriceAmount:       row.PriceAmount,
			CostPriceAmount:   row.CostPriceAmount,
			PointsPrice:       row.PointsPrice,
			ManualStockTotal:  row.ManualStockTotal,
			ManualStockLocked: 0,
			ManualStockSold:   0,
			CardSource:        row.CardSource,
			InstructionsJSON:  row.InstructionsJSON,
			AgisoConnectionID: row.AgisoConnectionID,
			AgisoCpkID:        row.AgisoCpkID,
			IsActive:          row.IsActive,
			SortOrder:         row.SortOrder,
		}
		if err := skuRepo.Create(&item); err != nil {
			return err
		}
		kept[item.ID] = struct{}{}
	}

	for _, existing := range existingRows {
		if _, ok := kept[existing.ID]; ok {
			continue
		}
		if err := skuRepo.Delete(existing.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *WriteService) ensureAutoSKUCardSecretStockSafe(
	cardSecretRepo CardSecretStockRepository,
	productID uint,
	fulfillmentType string,
	existingRows []productdomain.ProductSKU,
	rows []normalizedProductSKU,
	existingByID map[uint]productdomain.ProductSKU,
	existingByCode map[string]productdomain.ProductSKU,
) error {
	if cardSecretRepo == nil || productID == 0 || strings.TrimSpace(fulfillmentType) != constants.FulfillmentTypeAuto {
		return nil
	}

	nextActive := make(map[uint]bool, len(existingRows))
	kept := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			existing, ok := existingByID[row.ID]
			if !ok {
				return productcontract.ErrProductSKUInvalid
			}
			nextActive[existing.ID] = row.IsActive
			kept[existing.ID] = struct{}{}
			continue
		}

		codeKey := strings.ToLower(strings.TrimSpace(row.SKUCode))
		if existing, ok := existingByCode[codeKey]; ok {
			nextActive[existing.ID] = row.IsActive
			kept[existing.ID] = struct{}{}
		}
	}

	for _, existing := range existingRows {
		if _, ok := nextActive[existing.ID]; !ok {
			nextActive[existing.ID] = false
		}
		if _, ok := kept[existing.ID]; !ok {
			nextActive[existing.ID] = false
		}
		if !existing.IsActive || nextActive[existing.ID] {
			continue
		}
		total, available, used, err := cardSecretRepo.CountByProduct(productID, existing.ID)
		if err != nil {
			return err
		}
		outstanding := total - used
		if available > 0 || outstanding > 0 {
			return productcontract.ErrProductSKUHasCardSecretStock
		}
	}
	return nil
}
