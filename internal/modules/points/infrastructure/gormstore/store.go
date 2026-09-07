package gormstore

import (
	"errors"
	"strings"
	"time"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct{ db *gorm.DB }

var (
	_ pointscontract.Repository = (*Store)(nil)
	_ pointscontract.UnitOfWork = (*Store)(nil)
)

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) Bind(tx *gorm.DB) *Store {
	if tx == nil {
		return s
	}
	return New(tx)
}

type transaction struct{ points *Store }

func (tx transaction) Points() pointscontract.Repository { return tx.points }

// UseTransaction 把一个已开启的 gorm 事务包装成积分模块的事务视图。
func UseTransaction(tx *gorm.DB) pointscontract.Transaction {
	if tx == nil {
		return nil
	}
	return transaction{points: New(tx)}
}

func (s *Store) WithinTransaction(fn func(pointscontract.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(UseTransaction(tx))
	})
}

func (s *Store) GetAccountByUserID(userID uint) (*pointsdomain.Account, error) {
	if userID == 0 {
		return nil, nil
	}
	var account pointsdomain.Account
	if err := s.db.Where("user_id = ?", userID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (s *Store) GetAccountByUserIDForUpdate(userID uint) (*pointsdomain.Account, error) {
	if userID == 0 {
		return nil, nil
	}
	var account pointsdomain.Account
	if err := s.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &account, nil
}

func (s *Store) GetAccountsByUserIDs(userIDs []uint) ([]pointsdomain.Account, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var accounts []pointsdomain.Account
	if err := s.db.Where("user_id IN ?", userIDs).Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *Store) CreateAccount(account *pointsdomain.Account) error {
	if account == nil {
		return pointscontract.ErrAccountCreateFailed
	}
	return s.db.Create(account).Error
}

func (s *Store) UpdateAccount(account *pointsdomain.Account) error {
	if account == nil || account.ID == 0 {
		return pointscontract.ErrAccountUpdateFailed
	}
	return s.db.Model(&pointsdomain.Account{}).
		Where("id = ?", account.ID).
		Updates(map[string]interface{}{
			"balance":      account.Balance,
			"total_earned": account.TotalEarned,
			"total_spent":  account.TotalSpent,
			"updated_at":   account.UpdatedAt,
		}).Error
}

func (s *Store) CreateTransaction(record *pointsdomain.Transaction) error {
	if record == nil {
		return pointscontract.ErrTransactionCreateFailed
	}
	if err := s.db.Create(record).Error; err != nil {
		// reference 唯一索引是并发下真正生效的那层防线：SQLite 上
		// clause.Locking{Strength: "UPDATE"} 是 no-op，应用层的先查后写挡不住并发。
		if isDuplicateKey(err) {
			return pointscontract.ErrDuplicateReference
		}
		return err
	}
	return nil
}

func (s *Store) GetTransactionByReference(reference string) (*pointsdomain.Transaction, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, nil
	}
	var record pointsdomain.Transaction
	if err := s.db.Where("reference = ?", reference).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *Store) ListTransactions(filter pointscontract.TransactionListFilter) ([]pointsdomain.Transaction, int64, error) {
	page, pageSize := normalizePagination(filter.Page, filter.PageSize)
	query := s.db.Model(&pointsdomain.Transaction{})
	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if value := strings.TrimSpace(filter.Type); value != "" {
		query = query.Where("type = ?", value)
	}
	if value := strings.TrimSpace(filter.Direction); value != "" {
		query = query.Where("direction = ?", value)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []pointsdomain.Transaction
	if err := query.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// SumPointsByTypeSince 统计某用户自 since 起某类流水的积分合计。
func (s *Store) SumPointsByTypeSince(userID uint, transactionType string, since time.Time) (int64, error) {
	if userID == 0 || strings.TrimSpace(transactionType) == "" {
		return 0, nil
	}
	var total int64
	if err := s.db.Model(&pointsdomain.Transaction{}).
		Where("user_id = ? AND type = ? AND created_at >= ?", userID, transactionType, since).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) CreateCheckin(record *pointsdomain.Checkin) error {
	if record == nil {
		return pointscontract.ErrAlreadyCheckedIn
	}
	if err := s.db.Create(record).Error; err != nil {
		if isDuplicateKey(err) {
			return pointscontract.ErrAlreadyCheckedIn
		}
		return err
	}
	return nil
}

func (s *Store) GetCheckinByDate(userID uint, date string) (*pointsdomain.Checkin, error) {
	if userID == 0 || strings.TrimSpace(date) == "" {
		return nil, nil
	}
	var record pointsdomain.Checkin
	if err := s.db.Where("user_id = ? AND checkin_date = ?", userID, date).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (s *Store) GetConfig() (*pointsdomain.Config, error) {
	var config pointsdomain.Config
	if err := s.db.Where("id = ?", pointsdomain.ConfigSingletonID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (s *Store) SaveConfig(config *pointsdomain.Config) error {
	if config == nil {
		return pointscontract.ErrConfigUpdateFailed
	}
	config.ID = pointsdomain.ConfigSingletonID
	return s.db.Save(config).Error
}

// isDuplicateKey 覆盖 gorm 已翻译的重复键错误与 SQLite/MySQL 的原始报文。
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

func normalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = 20
	case pageSize > 100:
		pageSize = 100
	}
	return page, pageSize
}
