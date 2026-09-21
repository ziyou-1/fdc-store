package application

import (
	"strconv"
	"strings"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
)

type ServiceOptions struct {
	Secrets      cardsecretcontract.Repository
	Batches      cardsecretcontract.BatchRepository
	Transactions cardsecretcontract.UnitOfWork
	Products     cardsecretcontract.ProductRepository
	ProductSKUs  cardsecretcontract.ProductSKURepository
}

// Service 卡密库存服务。
type Service struct {
	secretRepo     cardsecretcontract.Repository
	batchRepo      cardsecretcontract.BatchRepository
	transactions   cardsecretcontract.UnitOfWork
	productRepo    cardsecretcontract.ProductRepository
	productSKURepo cardsecretcontract.ProductSKURepository
}

func NewService(options ServiceOptions) *Service {
	return &Service{
		secretRepo:     options.Secrets,
		batchRepo:      options.Batches,
		transactions:   options.Transactions,
		productRepo:    options.Products,
		productSKURepo: options.ProductSKUs,
	}
}

func (s *Service) resolveCardSecretSKU(productID, rawSKUID uint) (*productdomain.ProductSKU, error) {
	if productID == 0 || s.productSKURepo == nil {
		return nil, ErrProductSKUInvalid
	}
	product, err := s.productRepo.GetByID(strings.TrimSpace(strconv.FormatUint(uint64(productID), 10)))
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrProductNotFound
	}
	skus, err := s.productSKURepo.ListByProduct(productID, false)
	if err != nil {
		return nil, err
	}
	// 交付方式已下沉到 SKU 级：只有实际走自动发卡的活跃 SKU 才是卡密的合法归属。
	activeAutoSKUs := make([]productdomain.ProductSKU, 0, len(skus))
	hasAutoSKU := false
	for _, sku := range skus {
		skuCopy := sku
		if productdomain.EffectiveFulfillmentType(product, &skuCopy) != constants.FulfillmentTypeAuto {
			continue
		}
		hasAutoSKU = true
		if !sku.IsActive {
			continue
		}
		activeAutoSKUs = append(activeAutoSKUs, sku)
	}
	if rawSKUID > 0 {
		sku, err := s.productSKURepo.GetByID(rawSKUID)
		if err != nil {
			return nil, err
		}
		if sku == nil || sku.ProductID != productID {
			return nil, ErrProductSKUInvalid
		}
		// 卡密只能归属实际走自动发卡的规格：录到人工/内容规格上永远不会被发出去，
		// 运营会误以为有货。停用的自动发卡规格同样不收。
		if productdomain.EffectiveFulfillmentType(product, sku) != constants.FulfillmentTypeAuto || !sku.IsActive {
			return nil, ErrProductSKUInvalid
		}
		return sku, nil
	}

	if hasAutoSKU || strings.TrimSpace(product.FulfillmentType) == constants.FulfillmentTypeAuto {
		switch len(activeAutoSKUs) {
		case 0:
		case 1:
			return &activeAutoSKUs[0], nil
		default:
			return nil, ErrProductSKURequired
		}
	}

	defaultSKU, err := s.productSKURepo.GetByProductAndCode(productID, productdomain.DefaultSKUCode)
	if err != nil {
		return nil, err
	}
	if defaultSKU != nil {
		return defaultSKU, nil
	}
	if len(skus) == 1 {
		return &skus[0], nil
	}
	return nil, ErrProductSKURequired
}

func normalizeCardSecretIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
