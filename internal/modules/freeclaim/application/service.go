package application

import (
	"hash/fnv"
	"time"

	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
)

// claimDateLayout 领取按站点本地时区的自然日计算，与签到口径一致。
const claimDateLayout = "2006-01-02"

// Service 实现免费领取用例。
type Service struct {
	repo     freeclaimcontract.Repository
	orders   ClaimOrderCreator
	members  MemberLevelReader
	products ProductReader
}

// ClaimOrderCreator 是"下单并以免费领取 tender 结算"的端口。
// freeclaim 不能直接依赖 order/payment 的 application 包（会形成编译环），
// 因此由装配层注入适配器。
type ClaimOrderCreator interface {
	CreateFreeClaimOrder(userID, productID, skuID uint, claimDate string) (string, uint, error)
}

// ProductReader 读取商品展示信息。池条目只存 ID，
// 首页卡片要显示名称/图片/跳转链接就必须解析出来；
// freeclaim 不直接依赖 catalog，由装配层注入适配器。
type ProductReader interface {
	ProductViewForSKU(productID, skuID uint) (*freeclaimcontract.ProductView, error)
}

// MemberLevelReader 读取付费会员资格与可选的等级门槛。
// "会员专享"看的是已购买且未过期的付费会员，不是注册时分配的默认等级。
type MemberLevelReader interface {
	CurrentMemberLevelID(userID uint) (uint, error)
	HasPaidMembership(userID uint) (bool, error)
}

func membershipAllowsClaim(memberOnly bool, minLevelID uint, paid bool, levelID uint) bool {
	if memberOnly && !paid {
		return false
	}
	if minLevelID > 0 && levelID < minLevelID {
		return false
	}
	return true
}

// NewService 构造免费领取服务。
func NewService(repo freeclaimcontract.Repository) *Service {
	return &Service{repo: repo}
}

// SetCollaborators 注入下单端口与会员等级读取端口。
func (s *Service) SetCollaborators(orders ClaimOrderCreator, members MemberLevelReader) {
	if s == nil {
		return
	}
	s.orders = orders
	s.members = members
}

// SetProductReader 注入商品展示读取端口，供首页卡片渲染商品名称与图片。
func (s *Service) SetProductReader(products ProductReader) {
	if s == nil {
		return
	}
	s.products = products
}

// GetConfig 读取配置，首次访问时落入默认值。
func (s *Service) GetConfig() (*freeclaimdomain.Config, error) {
	config, err := s.repo.GetConfig()
	if err != nil {
		return nil, err
	}
	if config != nil {
		return config, nil
	}
	config = freeclaimdomain.DefaultConfig(time.Now())
	if err := s.repo.SaveConfig(config); err != nil {
		return nil, freeclaimcontract.ErrConfigUpdateFailed
	}
	return config, nil
}

// UpdateConfig 保存配置。
func (s *Service) UpdateConfig(input freeclaimcontract.ConfigInput) (*freeclaimdomain.Config, error) {
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	config.Enabled = input.Enabled
	config.MemberOnly = input.MemberOnly
	config.MinMemberLevelID = input.MinMemberLevelID
	config.UpdatedAt = time.Now()
	if err := s.repo.SaveConfig(config); err != nil {
		return nil, freeclaimcontract.ErrConfigUpdateFailed
	}
	return config, nil
}

// pickForDate 按日期从池中确定性地选出一个条目。
//
// 用日期哈希取模而不是随机数：同一天内所有用户、所有请求、服务重启前后
// 看到的都必须是同一个商品，否则用户刷新页面就能"换一个"，
// 而领取校验又要求所领即所示，会直接变成可挑选。
func pickForDate(items []freeclaimdomain.PoolItem, date string) *freeclaimdomain.PoolItem {
	if len(items) == 0 {
		return nil
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(date))
	index := int(hasher.Sum32() % uint32(len(items)))
	return &items[index]
}

// TodayItem 返回今日可领取的商品与当前用户的领取状态。
func (s *Service) TodayItem(userID uint) (*freeclaimcontract.TodayItem, error) {
	today := time.Now().Format(claimDateLayout)
	result := &freeclaimcontract.TodayItem{Date: today}

	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	result.Enabled = config.Enabled
	result.MemberOnly = config.MemberOnly
	if !config.Enabled {
		return result, nil
	}

	items, err := s.repo.ListActivePoolItems()
	if err != nil {
		return nil, err
	}
	item := pickForDate(items, today)
	if item == nil {
		return result, nil
	}
	result.Item = item
	result.Available = true

	// 商品信息解析失败时降级：仍返回 available，只是卡片没有名称/图片。
	// 首页不该因为一次商品查询失败就整体报错。
	if s.products != nil {
		if view, viewErr := s.products.ProductViewForSKU(item.ProductID, item.SKUID); viewErr == nil {
			result.Product = view
		}
	}

	if userID > 0 {
		claim, claimErr := s.repo.GetClaimByDate(userID, today)
		if claimErr != nil {
			return nil, claimErr
		}
		result.Claimed = claim != nil
		paid, levelID, memberErr := s.readMembership(userID, config.MemberOnly, config.MinMemberLevelID)
		if memberErr != nil {
			return nil, memberErr
		}
		result.Eligible = membershipAllowsClaim(config.MemberOnly, config.MinMemberLevelID, paid, levelID)
	}
	return result, nil
}

// Claim 执行今日领取：校验资格 → 下单并以免费领取 tender 结算 → 写领取记录。
//
// "每天一次"的最终保证是 free_claim_records 的 (user_id, claim_date) 唯一索引，
// 由下单事务内的 CreateFreeClaimOrder 写入；这里的先查只是为了给出友好提示。
func (s *Service) Claim(userID uint) (*freeclaimcontract.ClaimResult, error) {
	if userID == 0 {
		return nil, freeclaimcontract.ErrGuestForbidden
	}
	if s.orders == nil {
		return nil, freeclaimcontract.ErrDisabled
	}

	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, freeclaimcontract.ErrDisabled
	}

	if config.MemberOnly || config.MinMemberLevelID > 0 {
		paid, levelID, memberErr := s.readMembership(userID, config.MemberOnly, config.MinMemberLevelID)
		if memberErr != nil {
			return nil, memberErr
		}
		if !membershipAllowsClaim(config.MemberOnly, config.MinMemberLevelID, paid, levelID) {
			return nil, freeclaimcontract.ErrMemberOnly
		}
	}

	today := time.Now().Format(claimDateLayout)
	if existing, claimErr := s.repo.GetClaimByDate(userID, today); claimErr != nil {
		return nil, claimErr
	} else if existing != nil {
		return nil, freeclaimcontract.ErrAlreadyClaimed
	}

	items, err := s.repo.ListActivePoolItems()
	if err != nil {
		return nil, err
	}
	item := pickForDate(items, today)
	if item == nil {
		return nil, freeclaimcontract.ErrPoolEmpty
	}

	orderNo, _, err := s.orders.CreateFreeClaimOrder(userID, item.ProductID, item.SKUID, today)
	if err != nil {
		return nil, err
	}
	return &freeclaimcontract.ClaimResult{
		Date:      today,
		OrderNo:   orderNo,
		ProductID: item.ProductID,
		SKUID:     item.SKUID,
	}, nil
}

func (s *Service) readMembership(userID uint, memberOnly bool, minLevelID uint) (bool, uint, error) {
	if !memberOnly && minLevelID == 0 {
		return false, 0, nil
	}
	if s.members == nil {
		return false, 0, freeclaimcontract.ErrMemberOnly
	}
	paid := false
	if memberOnly {
		hasPaid, err := s.members.HasPaidMembership(userID)
		if err != nil {
			return false, 0, err
		}
		paid = hasPaid
	}
	levelID := uint(0)
	if minLevelID > 0 {
		id, err := s.members.CurrentMemberLevelID(userID)
		if err != nil {
			return false, 0, err
		}
		levelID = id
	}
	return paid, levelID, nil
}

// ListPoolItems 后台读取完整商品池。
func (s *Service) ListPoolItems() ([]freeclaimdomain.PoolItem, error) {
	return s.repo.ListPoolItems()
}

// AddPoolItem 后台新增商品池条目。
func (s *Service) AddPoolItem(input freeclaimcontract.PoolItemInput) (*freeclaimdomain.PoolItem, error) {
	if input.ProductID == 0 || input.SKUID == 0 {
		return nil, freeclaimcontract.ErrPoolItemInvalid
	}
	now := time.Now()
	item := &freeclaimdomain.PoolItem{
		ProductID: input.ProductID,
		SKUID:     input.SKUID,
		IsActive:  input.IsActive,
		SortOrder: input.SortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreatePoolItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

// UpdatePoolItem 后台修改商品池条目。
func (s *Service) UpdatePoolItem(id uint, input freeclaimcontract.PoolItemInput) (*freeclaimdomain.PoolItem, error) {
	if id == 0 || input.ProductID == 0 || input.SKUID == 0 {
		return nil, freeclaimcontract.ErrPoolItemInvalid
	}
	item, err := s.repo.GetPoolItemByID(id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, freeclaimcontract.ErrPoolItemInvalid
	}
	item.ProductID = input.ProductID
	item.SKUID = input.SKUID
	item.IsActive = input.IsActive
	item.SortOrder = input.SortOrder
	item.UpdatedAt = time.Now()
	if err := s.repo.UpdatePoolItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

// DeletePoolItem 后台删除商品池条目。
func (s *Service) DeletePoolItem(id uint) error {
	return s.repo.DeletePoolItem(id)
}

// ListClaims 用户领取历史。
func (s *Service) ListClaims(userID uint, page, pageSize int) ([]freeclaimdomain.Claim, int64, error) {
	return s.repo.ListClaimsByUser(userID, page, pageSize)
}
