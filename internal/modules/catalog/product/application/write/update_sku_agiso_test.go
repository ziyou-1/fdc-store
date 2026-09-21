package productwrite

import (
	"errors"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
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
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceLocal, 0, 0, nil); err != nil {
		t.Fatalf("local 不应校验: %v", err)
	}
	// 单规格：商品级绑定齐全
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceAgiso, 3, 100, nil); err != nil {
		t.Fatalf("单规格商品级齐全应通过: %v", err)
	}
	// 单规格：商品级绑定缺失
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceAgiso, 0, 0, nil); !errors.Is(err, productdomain.ErrCardSourceIncomplete) {
		t.Fatalf("单规格商品级缺失应报不完整，实际 %v", err)
	}
	// 每个 SKU 各自绑定卡种：即使商品级为空也应通过
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceAgiso, 0, 0, []normalizedProductSKU{active(3, 1111), active(3, 2222)}); err != nil {
		t.Fatalf("各 SKU 自绑卡种应通过: %v", err)
	}
	// 有 SKU 回落但商品级为空：应报不完整
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceAgiso, 0, 0, []normalizedProductSKU{active(3, 1111), active(0, 0)}); !errors.Is(err, productdomain.ErrCardSourceIncomplete) {
		t.Fatalf("回落 SKU 缺商品级绑定应报不完整，实际 %v", err)
	}
	// 有 SKU 回落且商品级齐全：应通过
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceAgiso, 3, 999, []normalizedProductSKU{active(3, 1111), active(0, 0)}); err != nil {
		t.Fatalf("回落 SKU + 商品级齐全应通过: %v", err)
	}
	// 停用 SKU 即使没绑定也应被忽略
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceAgiso, 0, 0, []normalizedProductSKU{active(3, 1111), {IsActive: false}}); err != nil {
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

// 交付方式已下沉到 SKU 级：同一商品下 A 规格自动发卡（本地卡密）、B 规格人工交付、
// C 规格纯内容，保存后每个 SKU 必须各自持久化自己的交付方式，且只保留与该方式
// 相关的字段（人工库存只留在人工 SKU 上，卡密来源只留在自发 SKU 上）。
func TestUpdatePersistsMixedSKUFulfillmentTypes(t *testing.T) {
	skuRepo := &memorySKURepository{}
	for _, code := range []string{"A", "B", "C"} {
		if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: code, PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true}); err != nil {
			t.Fatalf("seed sku %s: %v", code, err)
		}
	}
	repo := &cardSourceProductRepo{product: &productdomain.Product{
		ID:              7,
		FulfillmentType: constants.FulfillmentTypeManual,
		CardSource:      constants.CardSourceLocal,
	}}
	service := NewWriteService(Options{
		Products:     repo,
		SKUs:         skuRepo,
		Transactions: cardSourceUoW{products: repo, skus: skuRepo},
	})

	active := true
	if _, err := service.Update("7", CreateProductInput{
		PriceAmount:     decimal.NewFromInt(1),
		FulfillmentType: constants.FulfillmentTypeManual, // 商品级默认人工
		CardSource:      constants.CardSourceLocal,
		SKUs: []ProductSKUInput{
			{ID: 1, SKUCode: "A", PriceAmount: decimal.NewFromInt(10), IsActive: &active, FulfillmentType: constants.FulfillmentTypeAuto, CardSource: constants.CardSourceLocal, ManualStockTotal: 99},
			{ID: 2, SKUCode: "B", PriceAmount: decimal.NewFromInt(20), IsActive: &active, FulfillmentType: "", ManualStockTotal: 5, CardSource: constants.CardSourceAgiso},
			{ID: 3, SKUCode: "C", PriceAmount: decimal.NewFromInt(30), IsActive: &active, FulfillmentType: constants.FulfillmentTypeContent, ManualStockTotal: 7},
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
	if a := byCode["A"]; a.FulfillmentType != constants.FulfillmentTypeAuto || a.ManualStockTotal != 0 || a.CardSource != constants.CardSourceLocal {
		t.Fatalf("SKU A 应为 auto/无人工库存/本地来源，实际 type=%q stock=%d source=%q", a.FulfillmentType, a.ManualStockTotal, a.CardSource)
	}
	if b := byCode["B"]; b.FulfillmentType != "" || b.ManualStockTotal != 5 || b.CardSource != "" {
		t.Fatalf("SKU B 应跟随商品级(人工)、保留人工库存 5、清空卡密来源，实际 type=%q stock=%d source=%q", b.FulfillmentType, b.ManualStockTotal, b.CardSource)
	}
	if c := byCode["C"]; c.FulfillmentType != constants.FulfillmentTypeContent || c.ManualStockTotal != 0 {
		t.Fatalf("SKU C 应为 content 且无人工库存，实际 type=%q stock=%d", c.FulfillmentType, c.ManualStockTotal)
	}
	// 商品级人工库存 = 人工 SKU 汇总（只有 B）
	if repo.product.ManualStockTotal != 5 {
		t.Fatalf("商品级人工库存应为人工 SKU 汇总 5，实际 %d", repo.product.ManualStockTotal)
	}
	skuA, skuB, skuC := byCode["A"], byCode["B"], byCode["C"]
	if productdomain.EffectiveFulfillmentType(repo.product, &skuA) != constants.FulfillmentTypeAuto ||
		productdomain.EffectiveFulfillmentType(repo.product, &skuB) != constants.FulfillmentTypeManual ||
		productdomain.EffectiveFulfillmentType(repo.product, &skuC) != constants.FulfillmentTypeContent {
		t.Fatalf("有效交付方式应为 A=auto B=manual C=content")
	}
}

// 商品级默认自动发卡，但某个 SKU 单独走人工：商品级人工表单必须保留（人工 SKU 下单要采集），
// 且该人工 SKU 上残留的 agiso 绑定要清空。
func TestUpdateKeepsManualFormWhenOnlySKUIsManual(t *testing.T) {
	skuRepo := &memorySKURepository{}
	if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: "A", PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true}); err != nil {
		t.Fatalf("seed sku A: %v", err)
	}
	if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: "B", PriceAmount: money.FromDecimal(decimal.NewFromInt(20)), IsActive: true, AgisoConnectionID: 3, AgisoCpkID: 2222, CardSource: constants.CardSourceAgiso}); err != nil {
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
		PriceAmount:      decimal.NewFromInt(1),
		FulfillmentType:  constants.FulfillmentTypeAuto,
		CardSource:       constants.CardSourceLocal,
		ManualFormTiming: constants.ManualFormTimingAfterPayment,
		ManualFormSchemaJSON: map[string]interface{}{
			"fields": []interface{}{
				map[string]interface{}{"key": "account", "type": "text", "required": true, "label": map[string]interface{}{"zh-CN": "账号"}},
			},
		},
		SKUs: []ProductSKUInput{
			{ID: 1, SKUCode: "A", PriceAmount: decimal.NewFromInt(10), IsActive: &active},
			{ID: 2, SKUCode: "B", PriceAmount: decimal.NewFromInt(20), IsActive: &active, FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: constants.ManualStockUnlimited, CardSource: constants.CardSourceAgiso, AgisoConnectionID: uintPtr(3), AgisoCpkID: int64Ptr(2222)},
		},
	}); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	if len(repo.product.ManualFormSchemaJSON) == 0 {
		t.Fatalf("有人工 SKU 时商品级人工表单应保留")
	}
	if repo.product.ManualFormTiming != constants.ManualFormTimingAfterPayment {
		t.Fatalf("表单采集时机应保留 after_payment，实际 %q", repo.product.ManualFormTiming)
	}
	if repo.product.ManualStockTotal != constants.ManualStockUnlimited {
		t.Fatalf("人工 SKU 无限库存时商品级应为无限，实际 %d", repo.product.ManualStockTotal)
	}
	rows, _ := skuRepo.ListByProduct(7, false)
	for _, row := range rows {
		if row.SKUCode != "B" {
			continue
		}
		if row.AgisoConnectionID != 0 || row.AgisoCpkID != 0 || row.CardSource != "" {
			t.Fatalf("人工 SKU 上的 agiso 绑定与卡密来源应清空，实际 conn=%d cpk=%d source=%q", row.AgisoConnectionID, row.AgisoCpkID, row.CardSource)
		}
	}
}

// 非对接商品的 SKU 不允许指定 upstream：货不在上游站，下单后找不到映射会卡单。
func TestUpdateRejectsUpstreamSKUOnUnmappedProduct(t *testing.T) {
	skuRepo := &memorySKURepository{}
	if err := skuRepo.Create(&productdomain.ProductSKU{ProductID: 7, SKUCode: "A", PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true}); err != nil {
		t.Fatalf("seed sku A: %v", err)
	}
	repo := &cardSourceProductRepo{product: &productdomain.Product{ID: 7, FulfillmentType: constants.FulfillmentTypeManual}}
	service := NewWriteService(Options{
		Products:     repo,
		SKUs:         skuRepo,
		Transactions: cardSourceUoW{products: repo, skus: skuRepo},
	})
	active := true
	_, err := service.Update("7", CreateProductInput{
		PriceAmount:     decimal.NewFromInt(1),
		FulfillmentType: constants.FulfillmentTypeManual,
		SKUs: []ProductSKUInput{
			{ID: 1, SKUCode: "A", PriceAmount: decimal.NewFromInt(10), IsActive: &active, FulfillmentType: constants.FulfillmentTypeUpstream},
		},
	})
	if !errors.Is(err, productcontract.ErrFulfillmentInvalid) {
		t.Fatalf("非对接商品的 upstream SKU 应报交付方式非法，实际 %v", err)
	}
}

// 商品级自动发卡下，人工 SKU 不参与卡种绑定完整性校验；只有自发+agiso 的 SKU 才要求绑定。
func TestValidateAgisoSKUBindingsSkipsNonAutoSKUs(t *testing.T) {
	skus := []normalizedProductSKU{
		{IsActive: true, FulfillmentType: constants.FulfillmentTypeManual}, // 人工，无绑定也行
		{IsActive: true, FulfillmentType: "", CardSource: constants.CardSourceAgiso, AgisoConnectionID: 3, AgisoCpkID: 1111},
	}
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceLocal, 0, 0, skus); err != nil {
		t.Fatalf("人工 SKU 不应参与卡种校验: %v", err)
	}
	skus[1].AgisoCpkID = 0
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeAuto, constants.CardSourceLocal, 0, 0, skus); !errors.Is(err, productdomain.ErrCardSourceIncomplete) {
		t.Fatalf("自发+agiso SKU 缺卡种应报不完整，实际 %v", err)
	}
	// 商品级人工、SKU 单独自发+agiso：同样要求绑定完整
	skus = []normalizedProductSKU{
		{IsActive: true, FulfillmentType: constants.FulfillmentTypeAuto, CardSource: constants.CardSourceAgiso},
	}
	if err := validateAgisoSKUBindings(constants.FulfillmentTypeManual, constants.CardSourceLocal, 0, 0, skus); !errors.Is(err, productdomain.ErrCardSourceIncomplete) {
		t.Fatalf("商品级人工但 SKU 自发+agiso 缺绑定应报不完整，实际 %v", err)
	}
}
