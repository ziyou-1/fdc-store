package productapplication

import (
	"errors"
	"reflect"
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
)

type productRepositoryStub struct {
	filter  productcontract.ListFilter
	rows    []productdomain.Product
	total   int64
	bySlug  *productdomain.Product
	byAdmin *productdomain.Product
}

func (stub *productRepositoryStub) List(filter productcontract.ListFilter) ([]productdomain.Product, int64, error) {
	stub.filter = filter
	return stub.rows, stub.total, nil
}

func (stub *productRepositoryStub) GetBySlug(string, bool) (*productdomain.Product, error) {
	return stub.bySlug, nil
}

func (stub *productRepositoryStub) GetAdminByID(string) (*productdomain.Product, error) {
	return stub.byAdmin, nil
}

type categoryRepositoryStub struct {
	byID map[string]*categorydomain.Category
	rows []categorydomain.Category
}

func (stub categoryRepositoryStub) GetByID(id string) (*categorydomain.Category, error) {
	return stub.byID[id], nil
}

func (stub categoryRepositoryStub) List() ([]categorydomain.Category, error) {
	return stub.rows, nil
}

type hiddenProductRepositoryStub struct {
	ids []uint
}

func (stub hiddenProductRepositoryStub) ListHiddenProductIDs(uint) ([]uint, error) {
	return stub.ids, nil
}

type stockCounterStub struct {
	counts []cardsecretcontract.SKUStockCount
}

func (stub stockCounterStub) CountStockByProductIDs([]uint) ([]cardsecretcontract.SKUStockCount, error) {
	return stub.counts, nil
}

func TestListPublicForTenantBuildsVisibilityFilterBeforePagination(t *testing.T) {
	products := &productRepositoryStub{total: 2}
	categories := categoryRepositoryStub{
		byID: map[string]*categorydomain.Category{
			"10": {ID: 10, IsActive: true},
		},
		rows: []categorydomain.Category{
			{ID: 10, IsActive: true},
			{ID: 11, ParentID: 10, IsActive: true},
			{ID: 12, ParentID: 10, IsActive: false},
		},
	}
	service := NewService(Options{Products: products, Categories: categories})
	resellerID := uint(9)

	_, total, err := service.ListPublicForTenant(
		resellercontract.TenantContext{ResellerID: &resellerID},
		hiddenProductRepositoryStub{ids: []uint{7, 8}},
		"10",
		"keyword",
		2,
		20,
	)
	if err != nil {
		t.Fatalf("ListPublicForTenant returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total want 2 got %d", total)
	}
	if !reflect.DeepEqual(products.filter.CategoryIDs, []uint{10, 11}) {
		t.Fatalf("category ids want [10 11] got %v", products.filter.CategoryIDs)
	}
	if !reflect.DeepEqual(products.filter.ExcludeProductIDs, []uint{7, 8}) {
		t.Fatalf("excluded product ids want [7 8] got %v", products.filter.ExcludeProductIDs)
	}
	if products.filter.Page != 2 || products.filter.PageSize != 20 || !products.filter.OnlyActive || !products.filter.WithCategory {
		t.Fatalf("unexpected public filter: %#v", products.filter)
	}
}

func TestQueryServiceUsesProductContractErrors(t *testing.T) {
	service := NewService(Options{
		Products: &productRepositoryStub{},
	})

	if _, err := service.GetPublicBySlug("missing"); !errors.Is(err, productcontract.ErrNotFound) {
		t.Fatalf("GetPublicBySlug want contract not-found error, got %v", err)
	}
	resellerID := uint(9)
	if _, _, err := service.ListPublicForTenant(resellercontract.TenantContext{ResellerID: &resellerID}, nil, "", "", 1, 20); !errors.Is(err, productcontract.ErrResellerProductNotListed) {
		t.Fatalf("ListPublicForTenant want contract not-listed error, got %v", err)
	}
}

func TestApplyAutoStockCountsAssignsLegacyStockToOneSKU(t *testing.T) {
	service := NewService(Options{Stock: stockCounterStub{counts: []cardsecretcontract.SKUStockCount{
		{ProductID: 30, SKUID: 0, Status: cardsecretdomain.StatusAvailable, Total: 2},
		{ProductID: 30, SKUID: 101, Status: cardsecretdomain.StatusAvailable, Total: 3},
		{ProductID: 30, SKUID: 102, Status: cardsecretdomain.StatusAvailable, Total: 4},
	}}})
	products := []productdomain.Product{{
		ID:              30,
		FulfillmentType: constants.FulfillmentTypeAuto,
		SKUs: []productdomain.ProductSKU{
			{ID: 102, SKUCode: "SECOND", IsActive: true},
			{ID: 101, SKUCode: productdomain.DefaultSKUCode, IsActive: true},
		},
	}}

	if err := service.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("ApplyAutoStockCounts returned error: %v", err)
	}
	if products[0].AutoStockAvailable != 9 {
		t.Fatalf("product available want 9 got %d", products[0].AutoStockAvailable)
	}
	if products[0].SKUs[0].AutoStockAvailable != 4 {
		t.Fatalf("secondary SKU available want 4 got %d", products[0].SKUs[0].AutoStockAvailable)
	}
	if products[0].SKUs[1].AutoStockAvailable != 5 {
		t.Fatalf("DEFAULT SKU available want 5 got %d", products[0].SKUs[1].AutoStockAvailable)
	}
}

// 卡券库商品的卡不在本地卡密表里，按本地聚合永远是 0、前台会显示售罄。
// 库存必须来自定时任务刷新的 Agiso 缓存字段。
func TestApplyAutoStockCountsUsesAgisoCacheForCardLibraryProducts(t *testing.T) {
	service := NewService(Options{Stock: stockCounterStub{counts: []cardsecretcontract.SKUStockCount{
		// 本地卡密表里恰好有这个商品的残留数据，不能被采信
		{ProductID: 40, SKUID: 0, Status: cardsecretdomain.StatusAvailable, Total: 7},
	}}})
	products := []productdomain.Product{{
		ID:               40,
		FulfillmentType:  constants.FulfillmentTypeAuto,
		CardSource:       constants.CardSourceAgiso,
		AgisoStockCached: 13,
	}}

	if err := service.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("ApplyAutoStockCounts: %v", err)
	}
	if products[0].AutoStockAvailable != 13 {
		t.Fatalf("应取 Agiso 缓存 13，实际 %d", products[0].AutoStockAvailable)
	}
	if products[0].AutoStockTotal != 13 {
		t.Fatalf("总量应等于可用量，实际 %d", products[0].AutoStockTotal)
	}
	// 卡券库没有「本地锁定/已售」的概念，残留统计不能漏进来
	if products[0].AutoStockLocked != 0 || products[0].AutoStockSold != 0 {
		t.Fatalf("锁定与已售应为 0，实际 %d/%d", products[0].AutoStockLocked, products[0].AutoStockSold)
	}
}

// 前台按 SKU 累加算库存，只填商品级字段的话前台仍会显示售罄。
// 同时多个 SKU 共用一个卡池，逐个填会让累加结果翻倍。
func TestApplyAutoStockCountsFillsSingleSKUForAgiso(t *testing.T) {
	service := NewService(Options{Stock: stockCounterStub{}})
	products := []productdomain.Product{{
		ID:               43,
		FulfillmentType:  constants.FulfillmentTypeAuto,
		CardSource:       constants.CardSourceAgiso,
		AgisoStockCached: 11,
		SKUs: []productdomain.ProductSKU{
			{ID: 201, SKUCode: productdomain.DefaultSKUCode, IsActive: true},
			{ID: 202, SKUCode: "SECOND", IsActive: true},
		},
	}}
	if err := service.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("ApplyAutoStockCounts: %v", err)
	}
	var sum int64
	for _, sku := range products[0].SKUs {
		sum += sku.AutoStockAvailable
	}
	if sum != 11 {
		t.Fatalf("SKU 累加应等于卡池 11（不能翻倍），实际 %d", sum)
	}
	if products[0].AutoStockAvailable != 11 {
		t.Fatalf("商品级应为 11，实际 %d", products[0].AutoStockAvailable)
	}
}

// 本地来源的商品不受影响，仍按卡密表聚合。
func TestApplyAutoStockCountsKeepsLocalSourceUnchanged(t *testing.T) {
	service := NewService(Options{Stock: stockCounterStub{counts: []cardsecretcontract.SKUStockCount{
		{ProductID: 41, SKUID: 0, Status: cardsecretdomain.StatusAvailable, Total: 5},
	}}})
	products := []productdomain.Product{{
		ID:               41,
		FulfillmentType:  constants.FulfillmentTypeAuto,
		CardSource:       constants.CardSourceLocal,
		AgisoStockCached: 999, // 即便有残留缓存也不该被使用
	}}
	if err := service.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("ApplyAutoStockCounts: %v", err)
	}
	if products[0].AutoStockAvailable != 5 {
		t.Fatalf("本地来源应取卡密表的 5，实际 %d", products[0].AutoStockAvailable)
	}
}

// 缓存为负（异常数据）时按 0 处理，不能出现负库存。
func TestApplyAutoStockCountsClampsNegativeAgisoCache(t *testing.T) {
	service := NewService(Options{Stock: stockCounterStub{}})
	products := []productdomain.Product{{
		ID:               42,
		FulfillmentType:  constants.FulfillmentTypeAuto,
		CardSource:       constants.CardSourceAgiso,
		AgisoStockCached: -3,
	}}
	if err := service.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("ApplyAutoStockCounts: %v", err)
	}
	if products[0].AutoStockAvailable != 0 {
		t.Fatalf("负缓存应夹到 0，实际 %d", products[0].AutoStockAvailable)
	}
}
