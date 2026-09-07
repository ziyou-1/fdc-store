package contract

import (
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
)

// Repository 免费领取聚合的持久化端口。
type Repository interface {
	ListActivePoolItems() ([]freeclaimdomain.PoolItem, error)
	ListPoolItems() ([]freeclaimdomain.PoolItem, error)
	CreatePoolItem(item *freeclaimdomain.PoolItem) error
	UpdatePoolItem(item *freeclaimdomain.PoolItem) error
	DeletePoolItem(id uint) error
	GetPoolItemByID(id uint) (*freeclaimdomain.PoolItem, error)

	// CreateClaim 写入领取记录。撞 (user_id, claim_date) 唯一索引时
	// 必须返回 ErrAlreadyClaimed —— 这是"每天一次"在并发下唯一真正生效的防线。
	CreateClaim(claim *freeclaimdomain.Claim) error
	GetClaimByDate(userID uint, date string) (*freeclaimdomain.Claim, error)
	ListClaimsByUser(userID uint, page, pageSize int) ([]freeclaimdomain.Claim, int64, error)

	GetConfig() (*freeclaimdomain.Config, error)
	SaveConfig(config *freeclaimdomain.Config) error
}
