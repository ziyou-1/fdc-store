package productwrite

import (
	"strconv"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/modules/catalog/product/manualform"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// Create 创建商品
func (s *WriteService) Create(input CreateProductInput) (*productdomain.Product, error) {
	if err := productdomain.ValidateCategoryAssignment(s.categories, input.CategoryID, 0, productcontract.ErrProductCategoryInvalid); err != nil {
		return nil, err
	}

	// 补译必须在开事务之前。这个站点默认跑 SQLite、最大连接数为 1，
	// 事务期间那唯一一条连接被独占；把 DeepL 调用放进事务里，一次网络慢
	// 就是整个进程卡死，而 SQLite 的 busy_timeout 只有 5 秒，比 DeepL 的
	// 10 秒超时还短，并发写入方会直接拿到 SQLITE_BUSY。
	s.fillProductLocales(&input, nil)

	count, err := s.products.CountBySlug(input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, productcontract.ErrSlugExists
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}
	isAffiliateEnabled := false
	if input.IsAffiliateEnabled != nil {
		isAffiliateEnabled = *input.IsAffiliateEnabled
	}
	purchaseType := productdomain.NormalizePurchaseType(input.PurchaseType)
	if purchaseType == "" {
		return nil, productcontract.ErrProductPurchaseInvalid
	}
	fulfillmentType := productdomain.NormalizeFulfillmentType(input.FulfillmentType)
	cardSource := productdomain.NormalizeCardSource(input.CardSource)
	var agisoConnID uint
	if input.AgisoConnectionID != nil {
		agisoConnID = *input.AgisoConnectionID
	}
	var agisoCpkID int64
	if input.AgisoCpkID != nil {
		agisoCpkID = *input.AgisoCpkID
	}
	if err := productdomain.ValidateCardSource(fulfillmentType, cardSource); err != nil {
		return nil, err
	}
	if fulfillmentType == "" {
		return nil, productcontract.ErrFulfillmentInvalid
	}

	priceAmount := input.PriceAmount.Round(2)
	if len(input.SKUs) == 0 && priceAmount.LessThanOrEqual(decimal.Zero) {
		return nil, productcontract.ErrProductPriceInvalid
	}

	manualStockTotal := 0
	if input.ManualStockTotal != nil {
		manualStockTotal = *input.ManualStockTotal
	}
	if manualStockTotal < constants.ManualStockUnlimited {
		return nil, productcontract.ErrManualStockInvalid
	}
	maxPurchaseQuantity := 0
	if input.MaxPurchaseQuantity != nil {
		maxPurchaseQuantity = productdomain.NormalizePurchaseQuantityLimit(*input.MaxPurchaseQuantity)
	}
	minPurchaseQuantity := 0
	if input.MinPurchaseQuantity != nil {
		minPurchaseQuantity = productdomain.NormalizePurchaseQuantityLimit(*input.MinPurchaseQuantity)
	}
	if minPurchaseQuantity > 0 && maxPurchaseQuantity > 0 && minPurchaseQuantity > maxPurchaseQuantity {
		return nil, productcontract.ErrProductPurchaseLimitInvalid
	}
	stockDisplayMode := productdomain.NormalizeStockDisplayMode(input.StockDisplayMode)
	if stockDisplayMode == "" {
		return nil, productcontract.ErrProductStockDisplayInvalid
	}

	costPriceAmount := input.CostPriceAmount.Round(2)
	var wholesaleInputs []productdomain.WholesalePriceInput
	if input.WholesalePrices != nil {
		wholesaleInputs = *input.WholesalePrices
	}

	var normalizedSKUs []normalizedProductSKU
	if len(input.SKUs) > 0 {
		if s.skus == nil {
			return nil, productcontract.ErrProductSKUInvalid
		}
		var normalizeErr error
		normalizedSKUs, priceAmount, manualStockTotal, normalizeErr = s.normalizeProductSKUInputs(input.SKUs, fulfillmentType, cardSource, nil)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		costPriceAmount = minActiveCostPrice(normalizedSKUs)
	}
	// 卡券库商品：按 SKU 有效绑定校验（单规格商品退化为校验商品级绑定）。
	if err := validateAgisoSKUBindings(cardSource, agisoConnID, agisoCpkID, normalizedSKUs); err != nil {
		return nil, err
	}
	paymentChannelIDs, err := s.filterAvailablePaymentChannelIDs(input.PaymentChannelIDs)
	if err != nil {
		return nil, err
	}

	product := productdomain.Product{
		CategoryID:           input.CategoryID,
		Slug:                 input.Slug,
		SeoMetaJSON:          jsonmap.JSON(input.SeoMetaJSON),
		TitleJSON:            jsonmap.JSON(input.TitleJSON),
		DescriptionJSON:      jsonmap.JSON(input.DescriptionJSON),
		ContentJSON:          jsonmap.JSON(input.ContentJSON),
		InstructionsJSON:     jsonmap.JSON(input.InstructionsJSON),
		ManualFormSchemaJSON: jsonmap.JSON{},
		PriceAmount:          money.FromDecimal(priceAmount),
		CostPriceAmount:      money.FromDecimal(costPriceAmount),
		PointsPrice:          input.PointsPrice,
		WholesalePrices:      productdomain.WholesalePriceTiers{},
		Images:               jsonslice.Strings(input.Images),
		Tags:                 jsonslice.Strings(input.Tags),
		PurchaseType:         purchaseType,
		MinPurchaseQuantity:  minPurchaseQuantity,
		MaxPurchaseQuantity:  maxPurchaseQuantity,
		StockDisplayMode:     stockDisplayMode,
		FulfillmentType:      fulfillmentType,
		CardSource:           cardSource,
		AgisoConnectionID:    agisoConnID,
		AgisoCpkID:           agisoCpkID,
		ManualStockTotal:     manualStockTotal,
		ManualStockLocked:    0,
		ManualStockSold:      0,
		PaymentChannelIDs:    productdomain.EncodePaymentChannelIDs(paymentChannelIDs),
		IsAffiliateEnabled:   isAffiliateEnabled,
		IsActive:             isActive,
		SortOrder:            input.SortOrder,
	}
	if fulfillmentType == constants.FulfillmentTypeManual {
		normalizedSchemaJSON, err := manualform.NormalizeSchema(jsonmap.JSON(input.ManualFormSchemaJSON))
		if err != nil {
			return nil, err
		}
		product.ManualFormSchemaJSON = normalizedSchemaJSON
		product.ManualFormTiming = productdomain.NormalizeManualFormTiming(input.ManualFormTiming)
	} else {
		product.ManualFormTiming = constants.ManualFormTimingCheckout
	}

	if err := s.transactions.WithinTransaction(func(repositories TransactionRepositories) error {
		productRepo := repositories.Products
		skuRepo := repositories.SKUs
		cardSecretRepo := repositories.CardSecrets
		if err := productRepo.Create(&product); err != nil {
			return err
		}
		if len(normalizedSKUs) > 0 {
			if err := s.applyProductSKUsWithStockGuard(skuRepo, cardSecretRepo, product.ID, fulfillmentType, normalizedSKUs); err != nil {
				return err
			}
		} else if err := s.syncSingleProductSKU(skuRepo, product.ID, priceAmount, costPriceAmount, product.PointsPrice, manualStockTotal, true); err != nil {
			return err
		}
		if input.WholesalePrices != nil {
			var skus []productdomain.ProductSKU
			if skuRepo != nil {
				var err error
				skus, err = skuRepo.ListByProduct(product.ID, false)
				if err != nil {
					return err
				}
			}
			wholesalePrices, err := productdomain.NormalizeWholesalePricesForSKUs(wholesaleInputs, skus)
			if err != nil {
				return err
			}
			product.WholesalePrices = wholesalePrices
			if err := productRepo.QuickUpdate(strconv.FormatUint(uint64(product.ID), 10), map[string]interface{}{"wholesale_prices": wholesalePrices}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s.products.GetByID(strconv.FormatUint(uint64(product.ID), 10))
}
