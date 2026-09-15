package application

// CreateInput 创建对接连接输入。
type CreateInput struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	ApiKey    string `json:"api_key"`
	ApiSecret string `json:"api_secret"`
	// AccessToken 仅 agiso 协议使用：Agiso 需要 AppSecret 与 AccessToken 两个密钥，
	// 前者填在 api_secret，后者填在这里，服务层会打包后整体加密存入 ApiSecret。
	AccessToken        string  `json:"access_token"`
	Protocol           string  `json:"protocol"`
	CallbackURL        string  `json:"callback_url"`
	RetryMax           int     `json:"retry_max"`
	RetryIntervals     string  `json:"retry_intervals"`
	ExchangeRate       float64 `json:"exchange_rate"`
	PriceMarkupPercent float64 `json:"price_markup_percent"`
	PriceRoundingMode  string  `json:"price_rounding_mode"`
	AutoSyncPrice      bool    `json:"auto_sync_price"`
}

// UpdateInput 更新对接连接输入。
type UpdateInput struct {
	Name               string   `json:"name"`
	BaseURL            string   `json:"base_url"`
	ApiKey             string   `json:"api_key"`
	ApiSecret          string   `json:"api_secret"`   // 为空则不更新
	AccessToken        string   `json:"access_token"` // 仅 agiso 协议；为空则不更新
	Protocol           string   `json:"protocol"`
	CallbackURL        string   `json:"callback_url"`
	RetryMax           int      `json:"retry_max"`
	RetryIntervals     string   `json:"retry_intervals"`
	ExchangeRate       *float64 `json:"exchange_rate"`
	PriceMarkupPercent *float64 `json:"price_markup_percent"` // 指针类型，区分 0 和未传
	PriceRoundingMode  *string  `json:"price_rounding_mode"`
	AutoSyncPrice      *bool    `json:"auto_sync_price"`
}

// PingResult 连接测试结果。
type PingResult struct {
	SiteName        string                 `json:"site_name"`
	ProtocolVersion string                 `json:"protocol_version"`
	UserID          uint                   `json:"user_id"`
	Balance         string                 `json:"balance"`
	Currency        string                 `json:"currency"`
	MemberLevel     map[string]interface{} `json:"member_level,omitempty"`
}
