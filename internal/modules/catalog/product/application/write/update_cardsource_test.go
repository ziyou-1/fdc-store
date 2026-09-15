package productwrite

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/shopspring/decimal"
)

// cardSourceProductRepo 是只为「编辑商品保存卡密来源」这条链路准备的最小仓储桩。
// GetByID 返回同一个指针：Update 在这个指针上原地改写，事务结束后 update.go 会再
// GetByID 一次拿最终结果，返回同一指针才能反映出这次改写。
type cardSourceProductRepo struct {
	product *productdomain.Product
}

func (r *cardSourceProductRepo) GetByID(string) (*productdomain.Product, error) {
	return r.product, nil
}
func (r *cardSourceProductRepo) Create(*productdomain.Product) error { return nil }
func (r *cardSourceProductRepo) Update(item *productdomain.Product) error {
	r.product = item
	return nil
}
func (r *cardSourceProductRepo) CountBySlug(string, *string) (int64, error)       { return 0, nil }
func (r *cardSourceProductRepo) QuickUpdate(string, map[string]interface{}) error { return nil }

type cardSourceUoW struct {
	products *cardSourceProductRepo
	skus     SKURepository
}

func (u cardSourceUoW) WithinTransaction(fn func(TransactionRepositories) error) error {
	return fn(TransactionRepositories{Products: u.products, SKUs: u.skus})
}

func uintPtr(v uint) *uint    { return &v }
func int64Ptr(v int64) *int64 { return &v }

// 回归：编辑一个本地卡密商品、把卡密来源改成 Agiso 卡券库并保存，
// 保存后卡密来源必须真的变成 agiso 并带上连接与卡种。
// 修复前 update.go 与 admin_handler 都漏了这三个字段，保存被静默丢弃，
// 重新打开又显示「本地卡密库」。
func TestUpdatePersistsAgisoCardSource(t *testing.T) {
	repo := &cardSourceProductRepo{product: &productdomain.Product{
		ID:              7,
		FulfillmentType: constants.FulfillmentTypeManual,
		CardSource:      constants.CardSourceLocal,
	}}
	service := NewWriteService(Options{
		Products:     repo,
		SKUs:         &memorySKURepository{},
		Transactions: cardSourceUoW{products: repo, skus: &memorySKURepository{}},
	})

	got, err := service.Update("7", CreateProductInput{
		PriceAmount:       decimal.NewFromInt(1),
		FulfillmentType:   constants.FulfillmentTypeAuto,
		CardSource:        constants.CardSourceAgiso,
		AgisoConnectionID: uintPtr(3),
		AgisoCpkID:        int64Ptr(12345),
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if got.CardSource != constants.CardSourceAgiso {
		t.Fatalf("卡密来源应持久化为 agiso，实际 %q", got.CardSource)
	}
	if got.AgisoConnectionID != 3 {
		t.Fatalf("连接 ID 应为 3，实际 %d", got.AgisoConnectionID)
	}
	if got.AgisoCpkID != 12345 {
		t.Fatalf("卡种 ID 应为 12345，实际 %d", got.AgisoCpkID)
	}
}

// 反向：从 Agiso 切回本地时，连接与卡种必须一并清空，
// 否则残留的连接/卡种会让后续逻辑误以为还接着卡券库。
func TestUpdateClearsAgisoWhenSwitchingBackToLocal(t *testing.T) {
	repo := &cardSourceProductRepo{product: &productdomain.Product{
		ID:                7,
		FulfillmentType:   constants.FulfillmentTypeAuto,
		CardSource:        constants.CardSourceAgiso,
		AgisoConnectionID: 3,
		AgisoCpkID:        12345,
	}}
	service := NewWriteService(Options{
		Products:     repo,
		SKUs:         &memorySKURepository{},
		Transactions: cardSourceUoW{products: repo, skus: &memorySKURepository{}},
	})

	got, err := service.Update("7", CreateProductInput{
		PriceAmount:     decimal.NewFromInt(1),
		FulfillmentType: constants.FulfillmentTypeAuto,
		CardSource:      constants.CardSourceLocal,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if got.CardSource != constants.CardSourceLocal {
		t.Fatalf("卡密来源应为 local，实际 %q", got.CardSource)
	}
	if got.AgisoConnectionID != 0 || got.AgisoCpkID != 0 {
		t.Fatalf("切回本地后 Agiso 字段应清空，实际 conn=%d cpk=%d", got.AgisoConnectionID, got.AgisoCpkID)
	}
}
