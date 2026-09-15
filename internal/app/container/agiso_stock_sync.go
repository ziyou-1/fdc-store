package container

import (
	"context"
	"strconv"
	"time"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/upstream"
	"github.com/dujiao-next/internal/upstream/agiso"
)

// AgisoStockSyncer 定时把 Agiso 卡券库的库存刷进商品缓存字段。
//
// 为什么要缓存而不是实时查：前台商品列表每次渲染都会读库存，
// 实时调 Agiso 既慢（一次 HTTP 往返）又按调用次数计费。
// 卡券库的库存变化不快，定时刷新足够，代价是展示会有一个同步周期的延迟。
type AgisoStockSyncer struct {
	c *Container
}

// WithContainer 绑定容器。定时任务侧只拿得到 *Container，用它构造。
func (s AgisoStockSyncer) WithContainer(c *Container) AgisoStockSyncer {
	s.c = c
	return s
}

// SyncAll 遍历所有卡券库商品，按连接分组批量拉取卡种库存并写回缓存。
//
// 卡种绑定已下沉到 SKU 级：既刷商品级卡种（供未单独绑定的 SKU 回落使用），
// 也刷每个 SKU 自己绑定的卡种，写回各自的库存缓存字段。
// 按连接分组的意义在于：同一个连接下的多个卡种只需要一次 GetList 调用，
// 逐个查会把调用量放大到卡种数量级，既慢又费钱。
func (s AgisoStockSyncer) SyncAll() error {
	products, _, err := s.c.ProductRepo.List(productcontract.ListFilter{
		Page:     1,
		PageSize: 1000,
	})
	if err != nil {
		return err
	}

	// connID -> 需要拉取的 cpkID 集合（商品级 + SKU 级）
	byConn := map[uint]map[int64]struct{}{}
	addBinding := func(connID uint, cpkID int64) {
		if connID == 0 || cpkID <= 0 {
			return
		}
		if byConn[connID] == nil {
			byConn[connID] = map[int64]struct{}{}
		}
		byConn[connID][cpkID] = struct{}{}
	}
	for i := range products {
		p := &products[i]
		// 商品级卡种：仅当商品级来源为 agiso 时才需要（供回落 SKU 共用）。
		if productdomain.EffectiveCardSource(p, nil) == constants.CardSourceAgiso {
			addBinding(p.AgisoConnectionID, p.AgisoCpkID)
		}
		// SKU 级卡种：只要该 SKU 绑了卡种（cpk>0，即 agiso 来源）就同步，
		// 不受商品级来源限制——这样商品级为本地、个别 SKU 走卡券库的混合商品也能刷新。
		for j := range p.SKUs {
			if p.SKUs[j].AgisoCpkID <= 0 {
				continue
			}
			connID, cpkID := productdomain.EffectiveAgisoBinding(p, &p.SKUs[j])
			addBinding(connID, cpkID)
		}
	}
	if len(byConn) == 0 {
		return nil
	}

	// 先按连接各拉一次库存。单个连接失败不影响其它连接：一个商家的授权过期
	// 不应该让所有卡券库商品的库存一起停止更新；失败的连接不写回、保留旧缓存。
	stockByConn := map[uint]map[int64]int{}
	for connID := range byConn {
		stock, err := s.fetchStock(connID)
		if err != nil {
			continue
		}
		stockByConn[connID] = stock
	}
	if len(stockByConn) == 0 {
		return nil
	}

	now := time.Now()
	for i := range products {
		p := &products[i]
		// 商品级缓存：仅当商品级来源为 agiso 时刷新（供回落 SKU 共用）。
		if productdomain.EffectiveCardSource(p, nil) == constants.CardSourceAgiso &&
			p.AgisoConnectionID != 0 && p.AgisoCpkID > 0 {
			if stock, ok := stockByConn[p.AgisoConnectionID]; ok {
				// stock 里没有该卡种时取默认 0：卡种在上游已不存在应显示售罄，
				// 保留旧值会让前台一直显示有货、下单时才失败。
				_ = s.c.ProductRepo.QuickUpdate(strconv.FormatUint(uint64(p.ID), 10), map[string]interface{}{
					"agiso_stock_cached":    stock[p.AgisoCpkID],
					"agiso_stock_synced_at": now,
				})
			}
		}
		// SKU 级缓存：只刷单独绑定了卡种的 SKU。
		for j := range p.SKUs {
			if p.SKUs[j].AgisoCpkID <= 0 {
				continue
			}
			connID, cpkID := productdomain.EffectiveAgisoBinding(p, &p.SKUs[j])
			stock, ok := stockByConn[connID]
			if !ok {
				continue
			}
			_ = s.c.ProductSKURepo.QuickUpdate(p.SKUs[j].ID, map[string]interface{}{
				"agiso_stock_cached":    stock[cpkID],
				"agiso_stock_synced_at": now,
			})
		}
	}
	return nil
}

// fetchStock 拉取一个连接下全部卡种的剩余库存。
func (s AgisoStockSyncer) fetchStock(connID uint) (map[int64]int, error) {
	conn, err := s.c.SiteConnectionService.GetByID(connID)
	if err != nil || conn == nil {
		return nil, err
	}
	if conn.Protocol != constants.ConnectionProtocolAgiso {
		return nil, nil
	}
	decrypted, err := s.c.SiteConnectionService.DecryptSecret(conn.ApiSecret)
	if err != nil {
		return nil, err
	}
	cred, err := upstream.ParseAgisoCredentials(decrypted)
	if err != nil {
		return nil, err
	}
	client := agiso.NewClient(agiso.Config{
		BaseURL:     conn.BaseURL,
		AppSecret:   cred.AppSecret,
		AccessToken: cred.AccessToken,
		AppID:       conn.ApiKey,
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out := map[int64]int{}
	// 卡种一般不多，最多翻几页就够；设上限避免上游异常时无限翻页。
	for page := 1; page <= 10; page++ {
		res, err := client.ListCardKinds(ctx, page, 100, "")
		if err != nil {
			return nil, err
		}
		for _, k := range res.List {
			out[k.IdNo] = k.RemainingCount
		}
		if len(res.List) < 100 {
			break
		}
	}
	return out, nil
}
