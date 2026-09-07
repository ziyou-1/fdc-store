package contract

// StationInput 后台填写别人中转站的名称和网页链接。
type StationInput struct {
	Name    string
	BaseURL string
}

// PublicStation 首页展示的别人中转站链接。
type PublicStation struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Catalog 公开展示的别人中转站链接。
type Catalog struct {
	Stations []PublicStation `json:"stations"`
}
