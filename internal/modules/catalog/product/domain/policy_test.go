package productdomain

import (
	"errors"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
)

func TestValidatePurchaseQuantityKeepsLimitSemantics(t *testing.T) {
	product := &Product{MinPurchaseQuantity: 2, MaxPurchaseQuantity: 5}
	tests := []struct {
		name     string
		quantity int
		wantErr  error
	}{
		{name: "non-positive", quantity: 0, wantErr: ErrPurchaseQuantityInvalid},
		{name: "below minimum", quantity: 1, wantErr: ErrMinPurchaseNotMet},
		{name: "minimum boundary", quantity: 2},
		{name: "maximum boundary", quantity: 5},
		{name: "above maximum", quantity: 6, wantErr: ErrMaxPurchaseExceeded},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePurchaseQuantity(product, test.quantity)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ValidatePurchaseQuantity(%d) want %v got %v", test.quantity, test.wantErr, err)
			}
		})
	}
}

func TestNormalizePurchaseQuantityLimitDisablesNonPositiveValues(t *testing.T) {
	for _, value := range []int{-1, 0} {
		if got := NormalizePurchaseQuantityLimit(value); got != 0 {
			t.Fatalf("NormalizePurchaseQuantityLimit(%d) want 0 got %d", value, got)
		}
	}
	if got := NormalizePurchaseQuantityLimit(3); got != 3 {
		t.Fatalf("NormalizePurchaseQuantityLimit(3) want 3 got %d", got)
	}
}

func TestManualSKUStockPolicyPreservesLegacyDefaultFallback(t *testing.T) {
	product := &Product{SKUs: []ProductSKU{
		{SKUCode: DefaultSKUCode, IsActive: true},
		{SKUCode: "SECOND", IsActive: true},
	}}
	legacyDefault := &ProductSKU{SKUCode: DefaultSKUCode, ManualStockTotal: -2}
	if !ShouldEnforceManualSKUStock(product, legacyDefault) {
		t.Fatal("legacy DEFAULT SKU must enforce stock when multiple active SKUs exist")
	}
	if got := ManualSKUAvailable(legacyDefault); got != 0 {
		t.Fatalf("legacy negative stock want 0 available got %d", got)
	}

	unlimited := &ProductSKU{SKUCode: "SECOND", ManualStockTotal: constants.ManualStockUnlimited}
	if ShouldEnforceManualSKUStock(product, unlimited) {
		t.Fatal("unlimited SKU must not enforce manual stock")
	}
	if got := ManualSKUAvailable(unlimited); got <= 0 {
		t.Fatalf("unlimited SKU must expose a positive sentinel, got %d", got)
	}
}

func TestProductConfigurationNormalizersKeepDefaultsAndRejectUnknownValues(t *testing.T) {
	if got := NormalizePurchaseType(""); got != constants.ProductPurchaseMember {
		t.Fatalf("empty purchase type want %q got %q", constants.ProductPurchaseMember, got)
	}
	if got := NormalizePurchaseType(constants.ProductPurchaseGuest); got != constants.ProductPurchaseGuest {
		t.Fatalf("guest purchase type want %q got %q", constants.ProductPurchaseGuest, got)
	}
	if got := NormalizePurchaseType("invalid"); got != "" {
		t.Fatalf("invalid purchase type must be rejected, got %q", got)
	}

	if got := NormalizeFulfillmentType(""); got != constants.FulfillmentTypeManual {
		t.Fatalf("empty fulfillment type want %q got %q", constants.FulfillmentTypeManual, got)
	}
	if got := NormalizeFulfillmentType(constants.FulfillmentTypeUpstream); got != constants.FulfillmentTypeUpstream {
		t.Fatalf("upstream fulfillment type want %q got %q", constants.FulfillmentTypeUpstream, got)
	}
	if got := NormalizeFulfillmentType("invalid"); got != "" {
		t.Fatalf("invalid fulfillment type must be rejected, got %q", got)
	}

	if got := NormalizeStockDisplayMode(""); got != constants.ProductStockDisplayExact {
		t.Fatalf("empty stock display mode want %q got %q", constants.ProductStockDisplayExact, got)
	}
	if got := NormalizeStockDisplayMode(constants.ProductStockDisplayHidden); got != constants.ProductStockDisplayHidden {
		t.Fatalf("hidden stock mode want %q got %q", constants.ProductStockDisplayHidden, got)
	}
	if got := NormalizeStockDisplayMode("invalid"); got != "" {
		t.Fatalf("invalid stock display mode must be rejected, got %q", got)
	}
}

type categoryAssignmentRepositoryStub struct {
	category *categorydomain.Category
	children int64
}

func (stub categoryAssignmentRepositoryStub) GetByID(string) (*categorydomain.Category, error) {
	return stub.category, nil
}

func (stub categoryAssignmentRepositoryStub) CountChildren(string) (int64, error) {
	return stub.children, nil
}

func TestValidateCategoryAssignmentPreservesCompatibilityError(t *testing.T) {
	compatibilityError := errors.New("legacy category invalid")
	repository := categoryAssignmentRepositoryStub{
		category: &categorydomain.Category{ID: 10},
		children: 1,
	}

	if err := ValidateCategoryAssignment(repository, 10, 0, compatibilityError); !errors.Is(err, compatibilityError) {
		t.Fatalf("new parent assignment want compatibility error, got %v", err)
	}
	if err := ValidateCategoryAssignment(repository, 10, 10, compatibilityError); err != nil {
		t.Fatalf("keeping current parent assignment must remain valid, got %v", err)
	}
}

// 空值与非法值都要回落 local——既有商品没有这个字段，
// 若归一化成别的值，它们会突然被当成卡券库商品而发不出货。
func TestNormalizeCardSourceFallsBackToLocal(t *testing.T) {
	for _, in := range []string{"", "   ", "unknown", "LOCAL", "local"} {
		if got := NormalizeCardSource(in); got != constants.CardSourceLocal {
			t.Errorf("NormalizeCardSource(%q) = %q，应为 local", in, got)
		}
	}
	for _, in := range []string{"agiso", "AGISO", " Agiso "} {
		if got := NormalizeCardSource(in); got != constants.CardSourceAgiso {
			t.Errorf("NormalizeCardSource(%q) = %q，应为 agiso", in, got)
		}
	}
}

// 只有自动发卡才谈得上卡密来源。人工交付的库存在商品自身、
// 上游代销的货在上游站，给它们配卡券库不会被读到，等于埋一个「配了却不生效」的坑。
func TestValidateCardSourceRequiresAutoFulfillment(t *testing.T) {
	for _, ft := range []string{constants.FulfillmentTypeManual, constants.FulfillmentTypeUpstream} {
		err := ValidateCardSource(ft, constants.CardSourceAgiso)
		if !errors.Is(err, ErrCardSourceNotAuto) {
			t.Errorf("交付方式 %s 配卡券库应被拒，实际 %v", ft, err)
		}
	}
	if err := ValidateCardSource(constants.FulfillmentTypeAuto, constants.CardSourceAgiso); err != nil {
		t.Errorf("自动发卡 + 卡券库应通过: %v", err)
	}
}

// 选了卡券库却没指定连接或卡种，下单时才会失败——必须在保存时就拦住。
// 卡种绑定已下沉到 SKU 级，完整性由 ValidateAgisoBinding 按有效绑定逐个判。
func TestValidateAgisoBindingRequiresConnectionAndCpk(t *testing.T) {
	cases := []struct {
		connID uint
		cpkID  int64
		name   string
	}{
		{0, 100, "缺连接"},
		{1, 0, "缺卡种"},
		{0, 0, "两者都缺"},
		{1, -5, "卡种为负"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAgisoBinding(tc.connID, tc.cpkID)
			if !errors.Is(err, ErrCardSourceIncomplete) {
				t.Fatalf("应报配置不完整，实际 %v", err)
			}
		})
	}
	if err := ValidateAgisoBinding(1, 100); err != nil {
		t.Errorf("连接与卡种齐全应通过: %v", err)
	}
}

// ResolveAgisoBinding：SKU 自己配了卡种就用 SKU 的（连接缺省继承商品级），否则整体回落到商品级。
func TestResolveAgisoBinding(t *testing.T) {
	if conn, cpk := ResolveAgisoBinding(1, 100, 0, 0); conn != 1 || cpk != 100 {
		t.Errorf("SKU 未绑定应回落商品级，实际 conn=%d cpk=%d", conn, cpk)
	}
	if conn, cpk := ResolveAgisoBinding(1, 100, 2, 200); conn != 2 || cpk != 200 {
		t.Errorf("SKU 绑定应覆盖商品级，实际 conn=%d cpk=%d", conn, cpk)
	}
	if conn, cpk := ResolveAgisoBinding(1, 100, 0, 200); conn != 1 || cpk != 200 {
		t.Errorf("SKU 只配卡种应继承商品级连接，实际 conn=%d cpk=%d", conn, cpk)
	}
}

// local 来源不该被连带校验：本地卡密库商品本来就没有连接和卡种。
func TestValidateCardSourceIgnoresLocal(t *testing.T) {
	for _, ft := range []string{
		constants.FulfillmentTypeAuto,
		constants.FulfillmentTypeManual,
		constants.FulfillmentTypeUpstream,
	} {
		if err := ValidateCardSource(ft, constants.CardSourceLocal); err != nil {
			t.Errorf("local 来源不应校验连接与卡种（%s）: %v", ft, err)
		}
	}
}
