package productwrite

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// 卡券库商品的卡种绑定已下沉到 SKU 级：完整性要按「每个活跃 SKU 的有效绑定」判，
// 而不是只看商品级那一份。这里覆盖回落、覆盖、部分缺失与忽略停用 SKU 几种组合。
func TestValidateAgisoSKUBindings(t *testing.T) {
	active := func(conn uint, cpk int64) normalizedProductSKU {
		return normalizedProductSKU{IsActive: true, AgisoConnectionID: conn, AgisoCpkID: cpk}
	}

	// local 来源不校验
	if err := validateAgisoSKUBindings(constants.CardSourceLocal, 0, 0, nil); err != nil {
		t.Fatalf("local 不应校验: %v", err)
	}
	// 单规格：商品级绑定齐全
	if err := validateAgisoSKUBindings(constants.CardSourceAgiso, 3, 100, nil); err != nil {
		t.Fatalf("单规格商品级齐全应通过: %v", err)
	}
	// 单规格：商品级绑定缺失
	if err := validateAgisoSKUBindings(constants.CardSourceAgiso, 0, 0, nil); !errors.Is(err, productdomain.ErrCardSourceIncomplete) {
		t.Fatalf("单规格商品级缺失应报不完整，实际 %v", err)
	}
	// 每个 SKU 各自绑定卡种：即使商品级为空也应通过
	if err := validateAgisoSKUBindings(constants.CardSourceAgiso, 0, 0, []normalizedProductSKU{active(3, 1111), active(3, 2222)}); err != nil {
		t.Fatalf("各 SKU 自绑卡种应通过: %v", err)
	}
	// 有 SKU 回落但商品级为空：应报不完整
	if err := validateAgisoSKUBindings(constants.CardSourceAgiso, 0, 0, []normalizedProductSKU{active(3, 1111), active(0, 0)}); !errors.Is(err, productdomain.ErrCardSourceIncomplete) {
		t.Fatalf("回落 SKU 缺商品级绑定应报不完整，实际 %v", err)
	}
	// 有 SKU 回落且商品级齐全：应通过
	if err := validateAgisoSKUBindings(constants.CardSourceAgiso, 3, 999, []normalizedProductSKU{active(3, 1111), active(0, 0)}); err != nil {
		t.Fatalf("回落 SKU + 商品级齐全应通过: %v", err)
	}
	// 停用 SKU 即使没绑定也应被忽略
	if err := validateAgisoSKUBindings(constants.CardSourceAgiso, 0, 0, []normalizedProductSKU{active(3, 1111), {IsActive: false}}); err != nil {
		t.Fatalf("停用 SKU 不应参与校验: %v", err)
	}
}

// 回归：编辑一个多规格商品，给每个 SKU 绑不同卡种并保存，保存后每个 SKU 必须
// 各自持久化自己的连接与卡种，互不覆盖。
func TestUpdatePersistsPerSKUAgisoCpk(t *testing.T) {
	skuRepo := &memorySKURepository{}
	if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: "A", PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true}); err != nil {
		t.Fatalf("seed sku A: %v", err)
	}
	if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: "B", PriceAmount: money.FromDecimal(decimal.NewFromInt(20)), IsActive: true}); err != nil {
		t.Fatalf("seed sku B: %v", err)
	}

	repo := &cardSourceProductRepo{product: &productdomain.Product{
		ID:              7,
		FulfillmentType: constants.FulfillmentTypeAuto,
		CardSource:      constants.CardSourceLocal,
	}}
	service := NewWriteService(Options{
		Products:     repo,
		SKUs:         skuRepo,
		Transactions: cardSourceUoW{products: repo, skus: skuRepo},
	})

	active := true
	if _, err := service.Update("7", CreateProductInput{
		PriceAmount:       decimal.NewFromInt(1),
		FulfillmentType:   constants.FulfillmentTypeAuto,
		CardSource:        constants.CardSourceAgiso,
		AgisoConnectionID: uintPtr(3),
		AgisoCpkID:        int64Ptr(1000), // 商品级回落卡种
		SKUs: []ProductSKUInput{
			{ID: 1, SKUCode: "A", PriceAmount: decimal.NewFromInt(10), IsActive: &active, AgisoConnectionID: uintPtr(3), AgisoCpkID: int64Ptr(1111)},
			{ID: 2, SKUCode: "B", PriceAmount: decimal.NewFromInt(20), IsActive: &active, AgisoCpkID: int64Ptr(2222)}, // 连接继承商品级
		},
	}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	rows, err := skuRepo.ListByProduct(7, false)
	if err != nil {
		t.Fatalf("list sku: %v", err)
	}
	byCode := map[string]productdomain.ProductSKU{}
	for _, row := range rows {
		byCode[row.SKUCode] = row
	}
	if a := byCode["A"]; a.AgisoCpkID != 1111 || a.AgisoConnectionID != 3 {
		t.Fatalf("SKU A 应绑定 conn=3 cpk=1111，实际 conn=%d cpk=%d", a.AgisoConnectionID, a.AgisoCpkID)
	}
	if b := byCode["B"]; b.AgisoCpkID != 2222 {
		t.Fatalf("SKU B 卡种应为 2222，实际 %d", b.AgisoCpkID)
	}
}

// 反向：把卡券库商品切回本地时，SKU 上残留的卡种绑定必须一并清空，
// 否则提卡/库存逻辑会误以为该 SKU 还接着卡券库。
func TestUpdateClearsPerSKUAgisoWhenSwitchingToLocal(t *testing.T) {
	skuRepo := &memorySKURepository{}
	if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: "A", PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true, AgisoConnectionID: 3, AgisoCpkID: 1111}); err != nil {
		t.Fatalf("seed sku A: %v", err)
	}

	repo := &cardSourceProductRepo{product: &productdomain.Product{
		ID:                7,
		FulfillmentType:   constants.FulfillmentTypeAuto,
		CardSource:        constants.CardSourceAgiso,
		AgisoConnectionID: 3,
		AgisoCpkID:        1000,
	}}
	service := NewWriteService(Options{
		Products:     repo,
		SKUs:         skuRepo,
		Transactions: cardSourceUoW{products: repo, skus: skuRepo},
	})

	active := true
	if _, err := service.Update("7", CreateProductInput{
		PriceAmount:     decimal.NewFromInt(1),
		FulfillmentType: constants.FulfillmentTypeAuto,
		CardSource:      constants.CardSourceLocal,
		SKUs: []ProductSKUInput{
			{ID: 1, SKUCode: "A", PriceAmount: decimal.NewFromInt(10), IsActive: &active, AgisoCpkID: int64Ptr(1111)},
		},
	}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	rows, err := skuRepo.ListByProduct(7, false)
	if err != nil {
		t.Fatalf("list sku: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("应仍有 1 个 SKU，实际 %d", len(rows))
	}
	if rows[0].AgisoConnectionID != 0 || rows[0].AgisoCpkID != 0 {
		t.Fatalf("切回本地后 SKU 的 Agiso 字段应清空，实际 conn=%d cpk=%d", rows[0].AgisoConnectionID, rows[0].AgisoCpkID)
	}
}
