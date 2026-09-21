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
	FulfillmentType   string // 该 SKU 单独交付方式（auto/manual/upstream/content）；空=跟随商品级
	InstructionsJSON  jsonmap.JSON
	PaymentChannelIDs string // 已编码的允许支付渠道 JSON；空=不单独限制
	AgisoConnectionID uint
	AgisoCpkID        int64
	IsActive          bool
	SortOrder         int
}

// normalizeProductSKUInputs 归一化并校验 SKU 输入。
//
// 交付方式已下沉到 SKU 级：每个 SKU 按「自己指定的交付方式，否则商品级」决定
// 哪些字段有意义——manual 才保留人工库存，auto 才保留卡密来源与卡种绑定；
// upstream 只允许出现在对接商品（isMapped）上。返回的商品级人工库存是
// 所有人工交付活跃 SKU 的汇总（任一无限则无限，没有人工 SKU 则为 0）。
func (s *WriteService) normalizeProductSKUInputs(inputs []ProductSKUInput, fulfillmentType, cardSource string, isMapped bool, existingSKUMap map[uint]productdomain.ProductSKU) ([]normalizedProductSKU, decimal.Decimal, int, error) {
	if len(inputs) == 0 {
		return nil, decimal.Zero, 0, productcontract.ErrProductSKUInvalid
	}
	seenCode := make(map[string]struct{}, len(inputs))
	normalized := make([]normalizedProductSKU, 0, len(inputs))
	hasActive := false
	minActivePrice := decimal.Zero
	manualStockTotal := 0
	hasUnlimitedStock := false
	hasManualSKU := false

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

		// 该 SKU 单独指定的交付方式：空=跟随商品级；非法值按空处理。
		// upstream 依赖商品映射发货，只有对接商品才允许。
		skuFulfillment := productdomain.NormalizeSKUFulfillmentType(input.FulfillmentType)
		if skuFulfillment == constants.FulfillmentTypeUpstream && !isMapped {
			return nil, decimal.Zero, 0, productcontract.ErrFulfillmentInvalid
		}
		effectiveFulfillment := skuFulfillment
		if effectiveFulfillment == "" {
			effectiveFulfillment = fulfillmentType
		}

		manualTotal := input.ManualStockTotal
		if manualTotal < constants.ManualStockUnlimited {
			return nil, decimal.Zero, 0, productcontract.ErrManualStockInvalid
		}
		if effectiveFulfillment != constants.FulfillmentTypeManual {
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
		// 卡密来源只对自动发卡有意义：该 SKU 不走自动发卡时一并清空，
		// 避免切成人工/内容后残留的 agiso 来源误导后台与库存逻辑。
		skuCardSource := strings.ToLower(strings.TrimSpace(input.CardSource))
		if skuCardSource != constants.CardSourceLocal && skuCardSource != constants.CardSourceAgiso {
			skuCardSource = ""
		}
		if effectiveFulfillment != constants.FulfillmentTypeAuto {
			skuCardSource = ""
		}
		// 有效来源 = SKU 自己选的（非空）否则商品级。用于决定是否保留 agiso 绑定。
		effectiveSource := skuCardSource
		if effectiveSource == "" {
			effectiveSource = cardSource
		}

		// SKU 级卡种绑定只在该 SKU 走自动发卡且有效来源为 agiso 时保留；
		// 切回本地或切成其它交付方式时一并清空，否则残留的卡种会让提卡/库存逻辑
		// 误以为还接着卡券库。
		var agisoConnID uint
		var agisoCpkID int64
		if effectiveFulfillment == constants.FulfillmentTypeAuto && effectiveSource == constants.CardSourceAgiso {
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

		// SKU 允许的支付渠道：按现存有效渠道过滤后编码，空表示不单独限制。
		skuPaymentChannels := ""
		if len(input.PaymentChannelIDs) > 0 {
			filtered, filterErr := s.filterAvailablePaymentChannelIDs(input.PaymentChannelIDs)
			if filterErr != nil {
				return nil, decimal.Zero, 0, filterErr
			}
			skuPaymentChannels = productdomain.EncodePaymentChannelIDs(filtered)
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
			FulfillmentType:   skuFulfillment,
			InstructionsJSON:  skuInstructions,
			PaymentChannelIDs: skuPaymentChannels,
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
			if effectiveFulfillment == constants.FulfillmentTypeManual {
				hasManualSKU = true
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
	if !hasManualSKU {
		manualStockTotal = 0
	} else if hasUnlimitedStock {
		manualStockTotal = constants.ManualStockUnlimited
	}
	return normalized, minActivePrice, manualStockTotal, nil
}

// skuEffectiveFulfillmentType 返回已归一化 SKU 的有效交付方式（SKU 自己的否则商品级）。
func skuEffectiveFulfillmentType(productFulfillmentType string, sku normalizedProductSKU) string {
	if sku.FulfillmentType != "" {
		return sku.FulfillmentType
	}
	return productFulfillmentType
}

// anyNormalizedSKUFulfillment 报告是否有任一活跃 SKU 实际走指定交付方式。
func anyNormalizedSKUFulfillment(productFulfillmentType string, skus []normalizedProductSKU, target string) bool {
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		if skuEffectiveFulfillmentType(productFulfillmentType, sku) == target {
			return true
		}
	}
	return false
}

// validateAgisoSKUBindings 校验卡券库商品每个 SKU 的有效绑定是否完整。
//
// 卡种绑定已下沉到 SKU 级：SKU 自己配了卡种就用 SKU 的，没配则回落到商品级。
// 因此完整性要按「每个活跃 SKU 的有效绑定」逐个判，而不是只看商品级那一份。
// 没有显式 SKU（单规格商品）时，商品级绑定就是唯一那份，直接校验它。
func validateAgisoSKUBindings(fulfillmentType, cardSource string, productConnID uint, productCpkID int64, skus []normalizedProductSKU) error {
	// 单规格（无显式 SKU）：商品级来源就是唯一那份。
	if len(skus) == 0 {
		if cardSource != constants.CardSourceAgiso {
			return nil
		}
		return productdomain.ValidateAgisoBinding(productConnID, productCpkID)
	}
	// 多规格：逐个按「该 SKU 的有效交付方式与有效来源」判。只有走自动发卡且来源为
	// agiso 的 SKU 才要求绑定完整，因此同一商品下人工/内容 SKU、本地 SKU 与卡券库
	// SKU 可以共存。
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		if skuEffectiveFulfillmentType(fulfillmentType, sku) != constants.FulfillmentTypeAuto {
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
			existing.FulfillmentType = row.FulfillmentType
			existing.InstructionsJSON = row.InstructionsJSON
			existing.PaymentChannelIDs = row.PaymentChannelIDs
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
			existing.FulfillmentType = row.FulfillmentType
			existing.InstructionsJSON = row.InstructionsJSON
			existing.PaymentChannelIDs = row.PaymentChannelIDs
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
			FulfillmentType:   row.FulfillmentType,
			InstructionsJSON:  row.InstructionsJSON,
			PaymentChannelIDs: row.PaymentChannelIDs,
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
	if cardSecretRepo == nil || productID == 0 {
		return nil
	}
	// 只有「原本走自动发卡」的 SKU 才可能挂着本地卡密：SKU 自己标了 auto，
	// 或没单独标而商品级是 auto。其余 SKU 停用/删除不必查卡密。
	isAutoSKU := func(existing productdomain.ProductSKU) bool {
		switch productdomain.NormalizeSKUFulfillmentType(existing.FulfillmentType) {
		case constants.FulfillmentTypeAuto:
			return true
		case "":
			return strings.TrimSpace(fulfillmentType) == constants.FulfillmentTypeAuto
		default:
			return false
		}
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
		if !existing.IsActive || nextActive[existing.ID] || !isAutoSKU(existing) {
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
