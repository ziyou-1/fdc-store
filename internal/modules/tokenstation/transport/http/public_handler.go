package tokenstationhttp

import (
	tokenstationapp "github.com/dujiao-next/internal/modules/tokenstation/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// PublicHandler 处理前台别人中转站链接。
type PublicHandler struct {
	stations *tokenstationapp.Service
}

func NewPublicHandler(stations *tokenstationapp.Service) *PublicHandler {
	if stations == nil {
		panic("token station public handler: required dependency is nil")
	}
	return &PublicHandler{stations: stations}
}

// Catalog 返回别人中转站的网页链接。没有链接时返回空数组，首页据此隐藏整块。
func (h *PublicHandler) Catalog(c *gin.Context) {
	catalog, err := h.stations.PublicCatalog()
	if err != nil {
		respondTokenStationError(c, err)
		return
	}
	response.Success(c, catalog)
}

func respondTokenStationError(c *gin.Context, err error) {
	code, key := classifyTokenStationError(err)
	ginutil.RespondError(c, code, key, err)
}
