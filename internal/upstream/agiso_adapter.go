package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream/agiso"
)

// AgisoCredentials 是 agiso 协议在 Connection.ApiSecret 里存放的凭据。
//
// Agiso 需要两个密钥：AppSecret（开发者应用的签名密钥）与 AccessToken（商家授权令牌），
// 两者都是敏感值。Connection 上只有 ApiSecret 一个加密字段（ApiKey 会明文出现在后台响应里），
// 因此把两者打包成 JSON 存进 ApiSecret，整体走 AES 加密。
// ApiKey 则留给 AppId —— 它只是应用标识、不是密钥，明文展示便于排查。
type AgisoCredentials struct {
	AppSecret   string `json:"app_secret"`
	AccessToken string `json:"access_token"`
}

// ParseAgisoCredentials 从解密后的 ApiSecret 中取出两个密钥。
// 兼容只填了一个裸串的旧数据：那种情况视为只有 AppSecret，会在校验时报缺 AccessToken，
// 而不是抛出难以理解的 JSON 解析错误。
func ParseAgisoCredentials(decrypted string) (AgisoCredentials, error) {
	s := strings.TrimSpace(decrypted)
	if s == "" {
		return AgisoCredentials{}, fmt.Errorf("agiso: credentials empty")
	}
	var c AgisoCredentials
	if strings.HasPrefix(s, "{") {
		if err := json.Unmarshal([]byte(s), &c); err != nil {
			return AgisoCredentials{}, fmt.Errorf("agiso: parse credentials: %w", err)
		}
	} else {
		c.AppSecret = s
	}
	c.AppSecret = strings.TrimSpace(c.AppSecret)
	c.AccessToken = strings.TrimSpace(c.AccessToken)
	if c.AppSecret == "" {
		return c, fmt.Errorf("agiso: app_secret required")
	}
	if c.AccessToken == "" {
		return c, fmt.Errorf("agiso: access_token required")
	}
	return c, nil
}

// EncodeAgisoCredentials 把两个密钥打包成待加密的字符串。
func EncodeAgisoCredentials(appSecret, accessToken string) (string, error) {
	b, err := json.Marshal(AgisoCredentials{
		AppSecret:   strings.TrimSpace(appSecret),
		AccessToken: strings.TrimSpace(accessToken),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// agisoAdapter 把 Agiso 卡券库包装成 upstream.Adapter。
//
// 语义上它不是「另一个发卡站」而是一个卡密仓库，因此接口里有三个方法天然对不上：
// Agiso 的提卡是同步且不可撤销的，没有可轮询的订单，也没有商品图。
// 这三个方法明确返回不支持，而不是伪造出一个假的成功——采购流程据此就不会
// 把 agiso 连接接进「下单后轮询」的路径里。
type agisoAdapter struct {
	client *agiso.Client
	conn   *siteconnectiondomain.Connection
}

// NewAgisoAdapter 创建 Agiso 适配器。凭据来自已解密的 conn.ApiSecret。
func NewAgisoAdapter(conn *siteconnectiondomain.Connection) (Adapter, error) {
	cred, err := ParseAgisoCredentials(conn.ApiSecret)
	if err != nil {
		return nil, err
	}
	return &agisoAdapter{
		client: agiso.NewClient(agiso.Config{
			BaseURL:     conn.BaseURL,
			AppSecret:   cred.AppSecret,
			AccessToken: cred.AccessToken,
			AppID:       conn.ApiKey,
		}, nil),
		conn: conn,
	}, nil
}

// Ping 用查询预存款余额来验证凭据是否可用。
// 选它而不是卡种列表，是因为余额本身就是必须盯住的运营指标：
// 余额见底会触发错误码 16 并停掉全部提卡，测试连接时顺便把它显示出来最合适。
func (a *agisoAdapter) Ping(ctx context.Context) (*PingResult, error) {
	dep, err := a.client.QueryDeposit(ctx)
	if err != nil {
		return nil, err
	}
	// 顺带确认卡种列表可读——只有余额通不能说明授权范围足够。
	page, err := a.client.ListCardKinds(ctx, 1, 100, "")
	if err != nil {
		return nil, err
	}
	usable := 0
	for _, k := range page.List {
		if agiso.SupportedCardType(k.CardType) {
			usable++
		}
	}
	return &PingResult{
		SiteName:        fmt.Sprintf("Agiso 卡券库（卡种 %d 个，可接入 %d 个）", page.TotalCount, usable),
		ProtocolVersion: "acpr/1",
		Balance:         strconv.FormatFloat(dep.Balance, 'f', 4, 64),
		Currency:        "CNY",
	}, nil
}

// ListCategories Agiso 的卡种有 classId，但开放平台没有暴露分类列表接口。
// 返回 Supported=false，让映射流程跳过分类同步而不是当成"零个分类"。
func (a *agisoAdapter) ListCategories(ctx context.Context) (*CategoryListResult, error) {
	return &CategoryListResult{Supported: false}, nil
}

// ListProducts 把卡种映射成上游商品。
// 不支持的卡种（套卡/图片卡/循环卡）直接过滤掉，避免运营在后台选到之后才发错货。
func (a *agisoAdapter) ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	size := opts.PageSize
	if size < 1 || size > 100 {
		size = 100
	}
	res, err := a.client.ListCardKinds(ctx, page, size, "")
	if err != nil {
		return nil, err
	}
	items := make([]UpstreamProduct, 0, len(res.List))
	for _, k := range res.List {
		if !agiso.SupportedCardType(k.CardType) {
			continue
		}
		items = append(items, a.toProduct(k))
	}
	return &ProductListResult{
		Total: res.TotalCount,
		Items: items,
		// Agiso 不返回已删除的卡种，下游不能据此推断"缺失=已删除"
		IncludesInactive: false,
	}, nil
}

func (a *agisoAdapter) toProduct(k agiso.CardKind) UpstreamProduct {
	title := jsonmap.JSON{"zh-CN": k.Title, "zh-TW": k.Title, "en-US": k.Title}
	// 成本单价直接作为上游价；本地售价由连接上的汇率与加价规则算出。
	price := strconv.FormatFloat(k.PurPrice, 'f', 2, 64)
	stock := k.RemainingCount
	status := "in_stock"
	if stock <= 0 {
		status = "out_of_stock"
	}
	return UpstreamProduct{
		ID:              uint(k.IdNo),
		Title:           title,
		PriceAmount:     price,
		Currency:        "CNY",
		FulfillmentType: "auto", // 卡券库天然是自动发卡
		IsActive:        stock > 0,
		SKUs: []UpstreamSKU{{
			ID:            uint(k.IdNo),
			SKUCode:       "DEFAULT",
			PriceAmount:   price,
			StockStatus:   status,
			StockQuantity: stock,
			IsActive:      stock > 0,
		}},
	}
}

// GetProduct 取单个卡种。展示模板与前缀分隔符也一并带回，发货拼卡密时要用。
func (a *agisoAdapter) GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error) {
	d, err := a.client.GetCardKind(ctx, int64(productID))
	if err != nil {
		return nil, err
	}
	if !agiso.SupportedCardType(d.CardType) {
		return nil, fmt.Errorf("agiso: 卡种类型 %d 暂不支持自动发卡（目前仅支持唯一卡 800）", d.CardType)
	}
	p := a.toProduct(agiso.CardKind{
		IdNo:           d.IdNo,
		CardType:       d.CardType,
		Title:          d.Title,
		RemainingCount: d.TotalCount - d.UsedCount,
		UsedCount:      d.UsedCount,
		TotalCount:     d.TotalCount,
		PurPrice:       d.PurPrice,
	})
	return &p, nil
}

// CreateOrder 提卡。Agiso 是同步出卡，没有中间态，因此这里直接返回已完成。
//
// DownstreamOrderNo 作为 handPickOrderId 传给 Agiso，是整条链路的幂等键：
// 同一订单号重试不会重复扣卡，所以超时后可以安全重试。
func (a *agisoAdapter) CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error) {
	if strings.TrimSpace(req.DownstreamOrderNo) == "" {
		return nil, fmt.Errorf("agiso: downstream_order_no required as idempotency key")
	}
	res, err := a.client.HandPick(ctx, agiso.HandPickInput{
		CpkID:   int64(req.SKUID),
		Num:     req.Quantity,
		OrderNo: req.DownstreamOrderNo,
	})
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(res.Cards)
	return &CreateUpstreamOrderResp{
		OK:       true,
		OrderNo:  req.DownstreamOrderNo,
		Status:   "completed",
		Currency: "CNY",
		// 卡密明文放在 ErrorMessage 之外没有合适的槽位，调用方应改用 HandPick 直连；
		// 这里保留 order_no 让采购流程能对上账，真正的卡密由上层从 Payload 取。
		Amount: strconv.Itoa(len(res.Cards)),
		ErrorMessage: func() string {
			if len(payload) > 0 {
				return ""
			}
			return "no card"
		}(),
	}, nil
}

// GetOrder Agiso 没有「查询我的提卡单」接口。
// 提卡是同步的，结果在 CreateOrder 当场就拿到了，不存在需要轮询的中间态。
func (a *agisoAdapter) GetOrder(ctx context.Context, orderID uint) (*UpstreamOrderDetail, error) {
	return nil, fmt.Errorf("agiso: 不支持查询提卡单——提卡是同步接口，结果在下单时已返回")
}

// CancelOrder 卡密一经提出即不可撤销，Agiso 也没有对应接口。
// 明确报错而不是静默成功：静默成功会让上层误以为库存已归还。
func (a *agisoAdapter) CancelOrder(ctx context.Context, orderID uint) error {
	return fmt.Errorf("agiso: 不支持取消——卡密提出后无法退回，需人工处理")
}

// DownloadImage 卡券库没有商品图。
func (a *agisoAdapter) DownloadImage(ctx context.Context, imageURL string) (string, error) {
	return "", fmt.Errorf("agiso: 不提供商品图")
}

var _ = time.Now // 保持与其它适配器一致的导入集合
