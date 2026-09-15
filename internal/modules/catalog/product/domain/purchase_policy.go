package productdomain

import (
	"errors"
	"strconv"
	"strings"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
)

var (
	ErrPurchaseQuantityInvalid = errors.New("invalid order item")
	ErrMaxPurchaseExceeded     = errors.New("product max purchase exceeded")
	ErrMinPurchaseNotMet       = errors.New("product min purchase not met")
	ErrCategoryInvalid         = errors.New("product category invalid")
	// ErrCardSourceNotAuto 非自动发卡的商品配了卡券库来源。
	ErrCardSourceNotAuto = errors.New("card source requires auto fulfillment")
	// ErrCardSourceIncomplete 选了 Agiso 卡券库但没指定连接或卡种。
	ErrCardSourceIncomplete = errors.New("card source connection and cpk id required")
)

// CategoryAssignmentRepository 是商品分类归属校验所需的最小端口。
type CategoryAssignmentRepository interface {
	GetByID(id string) (*categorydomain.Category, error)
	CountChildren(categoryID string) (int64, error)
}

// NormalizePurchaseType 归一化商品购买身份限制。
func NormalizePurchaseType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.ProductPurchaseMember:
		return constants.ProductPurchaseMember
	case constants.ProductPurchaseGuest:
		return constants.ProductPurchaseGuest
	default:
		return ""
	}
}

// NormalizeFulfillmentType 归一化商品交付类型。
func NormalizeFulfillmentType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.FulfillmentTypeManual:
		return constants.FulfillmentTypeManual
	case constants.FulfillmentTypeAuto:
		return constants.FulfillmentTypeAuto
	case constants.FulfillmentTypeUpstream:
		return constants.FulfillmentTypeUpstream
	case constants.FulfillmentTypeContent:
		return constants.FulfillmentTypeContent
	default:
		return ""
	}
}

// NormalizeManualFormTiming 归一化人工交付表单的采集时机。
// 空值与非法值一律回落到 checkout，保持既有商品行为不变。
func NormalizeManualFormTiming(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case constants.ManualFormTimingAfterPayment:
		return constants.ManualFormTimingAfterPayment
	default:
		return constants.ManualFormTimingCheckout
	}
}

// NormalizeStockDisplayMode 归一化商品库存展示模式。
func NormalizeStockDisplayMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", constants.ProductStockDisplayExact:
		return constants.ProductStockDisplayExact
	case constants.ProductStockDisplayStatus:
		return constants.ProductStockDisplayStatus
	case constants.ProductStockDisplayRange:
		return constants.ProductStockDisplayRange
	case constants.ProductStockDisplayHidden:
		return constants.ProductStockDisplayHidden
	default:
		return ""
	}
}

// ValidateCategoryAssignment 禁止把商品直接分配给拥有子分类的父分类。
func ValidateCategoryAssignment(categoryRepo CategoryAssignmentRepository, categoryID uint, currentCategoryID uint, invalidError error) error {
	if categoryID == 0 || categoryRepo == nil {
		return nil
	}
	if invalidError == nil {
		invalidError = ErrCategoryInvalid
	}

	categoryIDText := strconv.FormatUint(uint64(categoryID), 10)
	category, err := categoryRepo.GetByID(categoryIDText)
	if err != nil {
		return err
	}
	if category == nil {
		return invalidError
	}

	childCount, err := categoryRepo.CountChildren(categoryIDText)
	if err != nil {
		return err
	}
	if childCount > 0 && categoryID != currentCategoryID {
		return invalidError
	}
	return nil
}

// NormalizePurchaseQuantityLimit 归一化商品单次购买数量上下限；非正数表示不限制。
func NormalizePurchaseQuantityLimit(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func productMaxPurchaseQuantity(product *Product) int {
	if product == nil {
		return 0
	}
	return NormalizePurchaseQuantityLimit(product.MaxPurchaseQuantity)
}

func productMinPurchaseQuantity(product *Product) int {
	if product == nil {
		return 0
	}
	return NormalizePurchaseQuantityLimit(product.MinPurchaseQuantity)
}

// ValidatePurchaseQuantity 校验单次购买数量是否在商品上下限内。
func ValidatePurchaseQuantity(product *Product, quantity int) error {
	if quantity <= 0 {
		return ErrPurchaseQuantityInvalid
	}
	if minLimit := productMinPurchaseQuantity(product); minLimit > 0 && quantity < minLimit {
		return ErrMinPurchaseNotMet
	}
	if maxLimit := productMaxPurchaseQuantity(product); maxLimit > 0 && quantity > maxLimit {
		return ErrMaxPurchaseExceeded
	}
	return nil
}

// NormalizeCardSource 归一化卡密来源。空值与非法值一律回落到 local，
// 保证既有商品（没有这个字段）行为完全不变。
func NormalizeCardSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case constants.CardSourceAgiso:
		return constants.CardSourceAgiso
	default:
		return constants.CardSourceLocal
	}
}

// ValidateCardSource 校验卡密来源与交付类型是否自洽。
//
// 只有自动发卡才谈得上卡密来源：人工交付的库存在商品自身，
// 上游代销的货在上游站。给它们配一个卡券库会让运营以为生效了，实际不会被读到。
//
// 连接/卡种是否完整不在这里判：卡种绑定已下沉到 SKU 级（每个 SKU 可绑不同卡种），
// 完整性要按「每个活跃 SKU 的有效绑定」来校验，见 ValidateAgisoBinding，
// 由写入用例在归一化 SKU 之后逐个调用。
func ValidateCardSource(fulfillmentType, cardSource string) error {
	if cardSource != constants.CardSourceAgiso {
		return nil
	}
	if fulfillmentType != constants.FulfillmentTypeAuto {
		return ErrCardSourceNotAuto
	}
	return nil
}

// ResolveAgisoBinding 计算一个 SKU 的有效绑定：SKU 自己配了卡种就用 SKU 的
// （连接缺省则继承商品级），否则整体回落到商品级绑定。
func ResolveAgisoBinding(productConnID uint, productCpkID int64, skuConnID uint, skuCpkID int64) (uint, int64) {
	if skuCpkID > 0 {
		if skuConnID == 0 {
			skuConnID = productConnID
		}
		return skuConnID, skuCpkID
	}
	return productConnID, productCpkID
}

// ValidateAgisoBinding 校验一份有效绑定是否完整（连接与卡种都已指定）。
func ValidateAgisoBinding(connID uint, cpkID int64) error {
	if connID == 0 || cpkID <= 0 {
		return ErrCardSourceIncomplete
	}
	return nil
}
