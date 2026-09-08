package productadmin

import (
	"strconv"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
)

// Delete 删除商品，并级联清理关联数据。
//
// 有库存、有成交记录都照删——这是运营明确要求的行为。原先这里挡了两道
// （可用/锁定卡密、订单条目引用），实践中反而让下架多年的商品永远清不掉。
//
// 之所以敢放开，是因为历史数据本来就自足：order_items 存的是 TitleJSON
// 标题快照，商品没了订单详情照样显示得出商品名；商品本身也是软删除
// （置 deleted_at），行还在库里可追溯。
//
// 代价是明确的，运营需要知道：
//  1. 按 product_id 聚合的销量报表会丢掉这些商品的关联；
//  2. 连带删除的卡密里若有「已锁定」的（对应在途订单），那笔订单将无法
//     再自动发货，只能人工处理。
func (s *AdminService) Delete(id string) error {
	product, err := s.products.GetByID(id)
	if err != nil {
		return err
	}
	if product == nil {
		return productcontract.ErrNotFound
	}

	return s.transactions.WithinTransaction(func(repositories DeleteRepositories) error {
		if err := repositories.CardSecrets.DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := repositories.CardSecretBatches.DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := repositories.SKUs.DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := repositories.MemberLevelPrices.DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := repositories.Carts.DeleteByProduct(product.ID); err != nil {
			return err
		}
		if err := repositories.ProductMappings.DeleteByLocalProduct(product.ID); err != nil {
			return err
		}
		return repositories.Products.Delete(id)
	})
}

// QuickUpdate 快速更新商品部分字段（如 is_active、sort_order）。
func (s *AdminService) QuickUpdate(id string, fields map[string]interface{}) (*productdomain.Product, error) {
	product, err := s.products.GetByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, productcontract.ErrNotFound
	}
	if isActivatingProduct(fields) {
		categoryID := product.CategoryID
		if rawCategoryID, ok := fields["category_id"]; ok {
			parsedCategoryID, parseErr := categoryIDFromValue(rawCategoryID, productcontract.ErrProductCategoryInvalid)
			if parseErr != nil {
				return nil, parseErr
			}
			categoryID = parsedCategoryID
		}
		if err := validateActivationCategory(s.categories, categoryID, productcontract.ErrProductCategoryInvalid); err != nil {
			return nil, err
		}
	}
	if err := s.products.QuickUpdate(id, fields); err != nil {
		return nil, err
	}
	return s.products.GetByID(id)
}

// UpdateWholesalePrices 更新商品批发价阶梯，不修改商品其他字段。
func (s *AdminService) UpdateWholesalePrices(id string, inputs []productdomain.WholesalePriceInput) (*productdomain.Product, error) {
	product, err := s.products.GetAdminByID(id)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, productcontract.ErrNotFound
	}

	wholesalePrices, err := productdomain.NormalizeWholesalePricesForSKUs(inputs, product.SKUs)
	if err != nil {
		return nil, err
	}
	if err := s.products.QuickUpdate(id, map[string]interface{}{"wholesale_prices": wholesalePrices}); err != nil {
		return nil, err
	}
	return s.products.GetAdminByID(id)
}

func isActivatingProduct(fields map[string]interface{}) bool {
	raw, ok := fields["is_active"]
	if !ok {
		return false
	}
	value, ok := raw.(bool)
	return ok && value
}

func categoryIDFromValue(value interface{}, invalidError error) (uint, error) {
	switch typed := value.(type) {
	case uint:
		return typed, nil
	case uint64:
		return uint(typed), nil
	case uint32:
		return uint(typed), nil
	case int:
		if typed < 0 {
			return 0, invalidError
		}
		return uint(typed), nil
	case int64:
		if typed < 0 {
			return 0, invalidError
		}
		return uint(typed), nil
	case int32:
		if typed < 0 {
			return 0, invalidError
		}
		return uint(typed), nil
	case float64:
		if typed < 0 || typed != float64(uint(typed)) {
			return 0, invalidError
		}
		return uint(typed), nil
	default:
		return 0, invalidError
	}
}

func validateActivationCategory(categoryRepo CategoryRepository, categoryID uint, invalidError error) error {
	if categoryID == 0 || categoryRepo == nil {
		return invalidError
	}

	categoryIDText := strconv.FormatUint(uint64(categoryID), 10)
	category, err := categoryRepo.GetByID(categoryIDText)
	if err != nil {
		return err
	}
	if category == nil || !category.IsActive {
		return invalidError
	}

	childCount, err := categoryRepo.CountChildren(categoryIDText)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return invalidError
	}
	return nil
}
