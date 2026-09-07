package gormstore

import (
	"errors"
	"strings"

	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"

	"gorm.io/gorm"
)

type Store struct{ db *gorm.DB }

var _ freeclaimcontract.Repository = (*Store)(nil)

func New(db *gorm.DB) *Store { return &Store{db: db} }

// Bind 返回绑定在给定事务上的仓储，供领取写入与订单写入落在同一事务。
func (s *Store) Bind(tx *gorm.DB) *Store {
	if tx == nil {
		return s
	}
	return New(tx)
}

func (s *Store) ListActivePoolItems() ([]freeclaimdomain.PoolItem, error) {
	var items []freeclaimdomain.PoolItem
	if err := s.db.Where("is_active = ?", true).
		Order("sort_order DESC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListPoolItems() ([]freeclaimdomain.PoolItem, error) {
	var items []freeclaimdomain.PoolItem
	if err := s.db.Order("sort_order DESC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CreatePoolItem(item *freeclaimdomain.PoolItem) error {
	if item == nil {
		return freeclaimcontract.ErrPoolItemInvalid
	}
	return s.db.Create(item).Error
}

func (s *Store) UpdatePoolItem(item *freeclaimdomain.PoolItem) error {
	if item == nil || item.ID == 0 {
		return freeclaimcontract.ErrPoolItemInvalid
	}
	return s.db.Model(&freeclaimdomain.PoolItem{}).
		Where("id = ?", item.ID).
		Updates(map[string]interface{}{
			"product_id": item.ProductID,
			"sku_id":     item.SKUID,
			"is_active":  item.IsActive,
			"sort_order": item.SortOrder,
			"updated_at": item.UpdatedAt,
		}).Error
}

func (s *Store) DeletePoolItem(id uint) error {
	if id == 0 {
		return nil
	}
	return s.db.Where("id = ?", id).Delete(&freeclaimdomain.PoolItem{}).Error
}

func (s *Store) GetPoolItemByID(id uint) (*freeclaimdomain.PoolItem, error) {
	if id == 0 {
		return nil, nil
	}
	var item freeclaimdomain.PoolItem
	if err := s.db.Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) CreateClaim(claim *freeclaimdomain.Claim) error {
	if claim == nil {
		return freeclaimcontract.ErrClaimCreateFailed
	}
	if err := s.db.Create(claim).Error; err != nil {
		if isDuplicateKey(err) {
			return freeclaimcontract.ErrAlreadyClaimed
		}
		return err
	}
	return nil
}

func (s *Store) GetClaimByDate(userID uint, date string) (*freeclaimdomain.Claim, error) {
	if userID == 0 || strings.TrimSpace(date) == "" {
		return nil, nil
	}
	var claim freeclaimdomain.Claim
	if err := s.db.Where("user_id = ? AND claim_date = ?", userID, date).First(&claim).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &claim, nil
}

func (s *Store) ListClaimsByUser(userID uint, page, pageSize int) ([]freeclaimdomain.Claim, int64, error) {
	if userID == 0 {
		return nil, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	query := s.db.Model(&freeclaimdomain.Claim{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var claims []freeclaimdomain.Claim
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&claims).Error; err != nil {
		return nil, 0, err
	}
	return claims, total, nil
}

func (s *Store) GetConfig() (*freeclaimdomain.Config, error) {
	var config freeclaimdomain.Config
	if err := s.db.Where("id = ?", freeclaimdomain.ConfigSingletonID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (s *Store) SaveConfig(config *freeclaimdomain.Config) error {
	if config == nil {
		return freeclaimcontract.ErrConfigUpdateFailed
	}
	config.ID = freeclaimdomain.ConfigSingletonID
	return s.db.Save(config).Error
}

// isDuplicateKey 与 points 侧同款：gorm 未开启 TranslateError，
// errors.Is(err, gorm.ErrDuplicatedKey) 恒为假，只能按驱动报文匹配。
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key")
}
