package application

import (
	"errors"
	"fmt"
	"testing"
	"time"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	promotiongormstore "github.com/dujiao-next/internal/modules/promotion/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// 交付方式已下沉到 SKU 级：同一商品下自动发卡 / 人工交付 / 纯内容三种规格一起下单，
// 每个订单项的 FulfillmentType 快照必须来自各自 SKU，而不是商品级默认值；
// 人工库存校验只对人工规格生效，人工表单快照只落在人工规格上。
func TestBuildOrderResultSnapshotsFulfillmentTypePerSKU(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_sku_fulfillment_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&promotiondomain.Promotion{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{Slug: "sku-fulfillment-category", NameJSON: jsonmap.JSON{"zh-CN": "测试分类"}, CreatedAt: now}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "sku-fulfillment-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "混合交付商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto, // 商品级默认自动发卡
		ManualFormSchemaJSON: jsonmap.JSON{
			"fields": []interface{}{
				map[string]interface{}{"key": "account", "type": "text", "required": true, "label": map[string]interface{}{"zh-CN": "账号"}},
			},
		},
		ManualFormTiming: constants.ManualFormTimingCheckout,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	skuAuto := productdomain.ProductSKU{ProductID: product.ID, SKUCode: "AUTO", PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true, CreatedAt: now, UpdatedAt: now}
	skuManual := productdomain.ProductSKU{ProductID: product.ID, SKUCode: "MANUAL", PriceAmount: money.FromDecimal(decimal.NewFromInt(20)), IsActive: true, FulfillmentType: constants.FulfillmentTypeManual, ManualStockTotal: 3, CreatedAt: now, UpdatedAt: now}
	skuContent := productdomain.ProductSKU{ProductID: product.ID, SKUCode: "CONTENT", PriceAmount: money.FromDecimal(decimal.NewFromInt(30)), IsActive: true, FulfillmentType: constants.FulfillmentTypeContent, CreatedAt: now, UpdatedAt: now}
	for _, sku := range []*productdomain.ProductSKU{&skuAuto, &skuManual, &skuContent} {
		if err := db.Create(sku).Error; err != nil {
			t.Fatalf("create sku %s failed: %v", sku.SKUCode, err)
		}
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductStore:    productgormstore.NewProductStore(db),
		ProductSKUStore: productgormstore.NewSKUStore(db),
		PromotionRepo:   promotiongormstore.New(db),
		ExpireMinutes:   15,
	})

	manualKey := fmt.Sprintf("%d:%d", product.ID, skuManual.ID)
	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{ProductID: product.ID, SKUID: skuAuto.ID, Quantity: 1},
			{ProductID: product.ID, SKUID: skuManual.ID, Quantity: 2},
			{ProductID: product.ID, SKUID: skuContent.ID, Quantity: 1},
		},
		ManualFormData: map[string]jsonmap.JSON{
			manualKey: {"account": "buyer@example.com"},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}
	if len(result.OrderItems) != 3 {
		t.Fatalf("expected 3 order items, got %d", len(result.OrderItems))
	}
	bySKU := map[uint]*childOrderPlan{}
	for i := range result.Plans {
		bySKU[result.Plans[i].SKU.ID] = &result.Plans[i]
	}
	if got := bySKU[skuAuto.ID].Item.FulfillmentType; got != constants.FulfillmentTypeAuto {
		t.Fatalf("auto sku item type = %q", got)
	}
	if got := bySKU[skuManual.ID].Item.FulfillmentType; got != constants.FulfillmentTypeManual {
		t.Fatalf("manual sku item type = %q", got)
	}
	if got := bySKU[skuContent.ID].Item.FulfillmentType; got != constants.FulfillmentTypeContent {
		t.Fatalf("content sku item type = %q", got)
	}
	// 人工表单快照只落在人工规格上
	if len(bySKU[skuManual.ID].Item.ManualFormSchemaSnapshotJSON) == 0 || len(bySKU[skuManual.ID].Item.ManualFormSubmissionJSON) == 0 {
		t.Fatalf("manual sku item should carry manual form snapshot and submission")
	}
	if len(bySKU[skuAuto.ID].Item.ManualFormSchemaSnapshotJSON) != 0 || len(bySKU[skuContent.ID].Item.ManualFormSchemaSnapshotJSON) != 0 {
		t.Fatalf("auto/content sku items must not carry manual form snapshot")
	}

	// 人工规格库存只有 3，买 4 件应被人工库存校验拦下；自动规格不受人工库存影响。
	_, err = svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{ProductID: product.ID, SKUID: skuManual.ID, Quantity: 4},
		},
		ManualFormData: map[string]jsonmap.JSON{manualKey: {"account": "buyer@example.com"}},
	})
	if !errors.Is(err, ErrManualStockInsufficient) {
		t.Fatalf("expected manual stock insufficient for manual sku, got %v", err)
	}
	if _, err := svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items:  []CreateOrderItem{{ProductID: product.ID, SKUID: skuAuto.ID, Quantity: 4}},
	}); err != nil {
		t.Fatalf("auto sku should not be limited by manual stock: %v", err)
	}
}
