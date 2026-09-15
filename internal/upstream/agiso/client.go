package agiso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL 是 Agiso 开放平台网关。
const DefaultBaseURL = "https://gw-api.agiso.com"

// 接口路径。
const (
	pathGetList    = "/acpr/CardPwd/GetList"
	pathGetDetails = "/acpr/CardPwdKind/GetDetails"
	pathHandPick   = "/acpr/CardPwd/HandPick"
	// 余额查询在 /open 命名空间下，不是 /acpr——它查的是开发者账户余额，
	// 与具体商家的卡券库无关，因此鉴权方式也不同（见 QueryDeposit）。
	pathQueryDeposit = "/open/Bankroll/QueryDeposit"
)

// Config 是一个商家授权所需的全部凭据。
//
// AppSecret 属于开发者（我方应用），AccessToken 属于被授权的商家，两者缺一不可：
// 前者用来签名，后者用来标识操作哪个商家的卡券库。
type Config struct {
	BaseURL   string
	AppSecret string
	// AccessToken 是商家授权令牌，卡券相关接口用它标识操作哪个商家的卡券库。
	AccessToken string
	// AppID 仅余额查询接口需要：那个接口不走 Authorization 头，
	// 而是把 appId 作为公共参数参与签名。
	AppID string
}

func (c Config) endpoint(path string) string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return base + path
}

func (c Config) validate() error { return c.validateFor(true) }

// validateFor 按是否需要商家授权校验凭据。
// 余额查询只用 AppSecret + AppID 签名，不需要 AccessToken。
func (c Config) validateFor(needToken bool) error {
	if strings.TrimSpace(c.AppSecret) == "" {
		return fmt.Errorf("%w: app_secret required", ErrBusiness)
	}
	if needToken && strings.TrimSpace(c.AccessToken) == "" {
		return fmt.Errorf("%w: access_token required", ErrBusiness)
	}
	return nil
}

// Client 是 Agiso 卡券开放平台的最小客户端。
// 只实现本集成用得到的四个接口，不做无用的全量封装。
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time // 可注入，测试里用来固定时间戳
}

// NewClient 创建客户端。httpClient 传 nil 时使用带 20s 超时的默认客户端。
func NewClient(cfg Config, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{cfg: cfg, http: httpClient, now: time.Now}
}

// ListCardKinds 拉取卡种列表。pageSize 上限 100，超出会被服务端截断，这里直接夹住。
func (c *Client) ListCardKinds(ctx context.Context, pageIndex, pageSize int, filterName string) (*CardKindPage, error) {
	if pageIndex < 1 {
		pageIndex = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 100
	}
	params := map[string]string{
		"pageIndex": strconv.Itoa(pageIndex),
		"pageSize":  strconv.Itoa(pageSize),
	}
	if s := strings.TrimSpace(filterName); s != "" {
		params["filterName"] = s
	}
	var out CardKindPage
	if err := c.call(ctx, pathGetList, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCardKind 获取卡种详情。展示模板与前缀分隔符都在这里，发货拼卡密时必须先取。
func (c *Client) GetCardKind(ctx context.Context, cpkID int64) (*CardKindDetail, error) {
	var out CardKindDetail
	if err := c.call(ctx, pathGetDetails, map[string]string{
		"cpkId": strconv.FormatInt(cpkID, 10),
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// HandPickInput 是一次提卡请求。
type HandPickInput struct {
	CpkID int64
	Num   int
	// OrderNo 是我方订单号，作为 handPickOrderId 传给 Agiso。
	// 这是整条链路的幂等键：同一订单号重试不会重复出卡，
	// 因此网络超时后可以安全重试，不必担心重复扣卡造成资损。
	OrderNo string
	Buyer   string
	// BuildCpd 为 true 时返回一个提取链接而非卡密明文。
	BuildCpd bool
}

// HandPick 提卡。
func (c *Client) HandPick(ctx context.Context, in HandPickInput) (*HandPickResult, error) {
	if in.CpkID <= 0 {
		return nil, fmt.Errorf("%w: cpk_id required", ErrBusiness)
	}
	if in.Num < 1 {
		return nil, fmt.Errorf("%w: num must be positive", ErrBusiness)
	}
	if strings.TrimSpace(in.OrderNo) == "" {
		// 没有幂等键就发起提卡，一次超时重试就可能重复扣卡，直接拒绝。
		return nil, fmt.Errorf("%w: order_no required as idempotency key", ErrBusiness)
	}
	params := map[string]string{
		"cpkId":           strconv.FormatInt(in.CpkID, 10),
		"num":             strconv.Itoa(in.Num),
		"handPickOrderId": strings.TrimSpace(in.OrderNo),
		"buildCpd":        strconv.FormatBool(in.BuildCpd),
	}
	if s := strings.TrimSpace(in.Buyer); s != "" {
		params["buyer"] = s
	}
	var out HandPickResult
	if err := c.call(ctx, pathHandPick, params, &out); err != nil {
		return nil, err
	}
	// 服务端说成功但一张卡都没给，属于协议异常；当成功处理会让订单显示已发货却没有内容。
	if !in.BuildCpd && len(out.Cards) == 0 {
		return nil, fmt.Errorf("%w: handpick returned no card", ErrBusiness)
	}
	if in.BuildCpd && strings.TrimSpace(out.CpdURL) == "" {
		return nil, fmt.Errorf("%w: handpick returned no cpd url", ErrBusiness)
	}
	return &out, nil
}

// QueryDeposit 查询开放平台预存款余额。
// 余额耗尽会触发错误码 16 并停掉所有提卡，需要定时巡检 + 阈值告警。
//
// 这个接口与其余三个都不一样，三处差异缺一不可：
//  1. 路径在 /open 而非 /acpr
//  2. 不带 Authorization 头，改用 appId 公共参数（且 appId 参与签名）
//  3. Data 直接是一个数字，不是对象
//
// 把它当成普通接口调会得到「未知的URL」或解码失败。
func (c *Client) QueryDeposit(ctx context.Context) (*Deposit, error) {
	if strings.TrimSpace(c.cfg.AppID) == "" {
		return nil, fmt.Errorf("%w: app_id required for deposit query", ErrBusiness)
	}
	raw, err := c.callRaw(ctx, pathQueryDeposit, map[string]string{"appId": strings.TrimSpace(c.cfg.AppID)}, false)
	if err != nil {
		return nil, err
	}
	var balance float64
	if err := json.Unmarshal(raw, &balance); err != nil {
		return nil, fmt.Errorf("%w: decode balance: %v", ErrBusiness, err)
	}
	return &Deposit{Balance: balance}, nil
}

// call 执行一次带签名的表单 POST，并把 Data 解进 out。
func (c *Client) call(ctx context.Context, path string, params map[string]string, out interface{}) error {
	raw, err := c.callRaw(ctx, path, params, true)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: decode data: %v", ErrBusiness, err)
	}
	return nil
}

// callRaw 执行请求并返回未解码的 Data 段。
// withAuth 为 false 时不发 Authorization 头——余额查询接口按 appId 鉴权。
func (c *Client) callRaw(ctx context.Context, path string, params map[string]string, withAuth bool) (json.RawMessage, error) {
	if err := c.cfg.validateFor(withAuth); err != nil {
		return nil, err
	}

	// timestamp 是公共参数，且必须参与签名。服务端允许 10 分钟误差。
	params["timestamp"] = strconv.FormatInt(c.now().Unix(), 10)
	params["sign"] = SignRequest(c.cfg.AppSecret, params)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.endpoint(path), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+c.cfg.AccessToken)
	}
	req.Header.Set("ApiVersion", "1")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: http %d", ErrTransient, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: http %d", ErrBusiness, resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("%w: decode envelope: %v", ErrBusiness, err)
	}
	if !env.IsSuccess || env.ErrorCode != 0 {
		return nil, &APIError{
			Code:       env.ErrorCode,
			Msg:        env.ErrorMsg,
			RequestID:  env.RequestID,
			AllowRetry: env.AllowRetry,
			kind:       classify(env.ErrorCode),
		}
	}

	// Data 的形状随接口而变（对象、数组，余额接口甚至是裸数字），
	// 成功后原样返回，由调用方决定怎么解。
	var payload struct {
		Data json.RawMessage `json:"Data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: decode data: %v", ErrBusiness, err)
	}
	if len(payload.Data) == 0 || string(payload.Data) == "null" {
		return nil, fmt.Errorf("%w: empty data", ErrNoData)
	}
	return payload.Data, nil
}
