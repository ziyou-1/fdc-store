package container

import (
	"context"
	"fmt"
	"strconv"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	"github.com/dujiao-next/internal/upstream"
	"github.com/dujiao-next/internal/upstream/agiso"
)

// agisoCardPicker 把「商品配置 → Agiso 提卡」这条链路接起来。
//
// 它住在 container 里而不是任何一个业务模块内，是因为要同时用到商品仓储与
// 站点连接服务；放进任一模块都会制造模块间的编译依赖。
// 发货侧通过 fulfillment 的 AgisoCardPicker 端口调用它。
type agisoCardPicker struct {
	products    *productgormstore.ProductStore
	connections *siteconnectionapp.Service
}

// IsAgisoItem 报告该订单项（商品+SKU）是否从 Agiso 卡券库取卡。
// 来源按 SKU 生效：SKU 自己指定了来源就用 SKU 的，否则回落商品级，
// 因此同一商品下的不同 SKU 可以分属本地 / 卡券库。
func (p agisoCardPicker) IsAgisoItem(productID, skuID uint) (bool, error) {
	prod, err := p.products.GetAdminByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil {
		return false, err
	}
	if prod == nil {
		return false, nil
	}
	var sku *productdomain.ProductSKU
	for i := range prod.SKUs {
		if prod.SKUs[i].ID == skuID {
			sku = &prod.SKUs[i]
			break
		}
	}
	return productdomain.EffectiveCardSource(prod, sku) == constants.CardSourceAgiso, nil
}

// PickForOrder 按订单项对应 SKU 的有效绑定提卡，返回渲染好的卡密文本。
//
// 卡种绑定已下沉到 SKU 级：SKU 自己配了卡种就用 SKU 的，没配则回落到商品级，
// 因此同一商品的不同 SKU 可以取自不同卡种。
//
// orderNo 直接作为 Agiso 的 handPickOrderId，是这条链路的幂等键：
// 同一个 orderNo 重复调用返回同一批卡，不会重复扣库存，
// 因此调用方可以在写库失败后安全重试。
func (p agisoCardPicker) PickForOrder(ctx context.Context, productID, skuID uint, quantity int, orderNo string) ([]string, error) {
	prod, err := p.products.GetAdminByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil {
		return nil, err
	}
	if prod == nil {
		return nil, fmt.Errorf("agiso pick: product %d not found", productID)
	}
	var sku *productdomain.ProductSKU
	for i := range prod.SKUs {
		if prod.SKUs[i].ID == skuID {
			sku = &prod.SKUs[i]
			break
		}
	}
	connID, cpkID := productdomain.EffectiveAgisoBinding(prod, sku)
	if connID == 0 || cpkID <= 0 {
		return nil, fmt.Errorf("agiso pick: product %d sku %d 未配置卡券库连接或卡种", productID, skuID)
	}

	conn, err := p.connections.GetByID(connID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("agiso pick: 连接 %d 不存在", connID)
	}
	decrypted, err := p.connections.DecryptSecret(conn.ApiSecret)
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

	// 先取卡种详情：展示模板、卡号/密码前缀与分隔符都在这里，
	// 少了它拼出来的卡密格式是错的，而买家拿到手才会发现。
	detail, err := client.GetCardKind(ctx, cpkID)
	if err != nil {
		return nil, err
	}
	res, err := client.HandPick(ctx, agiso.HandPickInput{
		CpkID:   cpkID,
		Num:     quantity,
		OrderNo: orderNo,
	})
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(res.Cards))
	for _, c := range res.Cards {
		lines = append(lines, agiso.Render(detail, c))
	}
	return lines, nil
}
