package siteconnectionhttp

import (
	"context"
	"strconv"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/upstream"
	"github.com/dujiao-next/internal/upstream/agiso"

	"github.com/gin-gonic/gin"
)

// AgisoCardKindItem 是给后台商品表单下拉框用的卡种条目。
type AgisoCardKindItem struct {
	CpkID     int64  `json:"cpk_id"`
	Title     string `json:"title"`
	CardType  int    `json:"card_type"`
	Remaining int    `json:"remaining"`
	PurPrice  string `json:"pur_price"`
	// Supported 为 false 表示卡种类型暂不支持接入（套卡/图片卡/循环卡），
	// 前端应置灰而不是直接隐藏——运营看到「为什么少了一个卡种」比看不到更难排查。
	Supported bool `json:"supported"`
}

// ListAgisoCardKinds GET /admin/site-connections/:id/agiso/card-kinds
// 拉取该连接下的卡种列表，供商品表单选择。
func (h *AdminHandler) ListAgisoCardKinds(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	conn, err := h.connections.GetByID(id)
	if err != nil || conn == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
		return
	}
	if conn.Protocol != constants.ConnectionProtocolAgiso {
		ginutil.RespondError(c, response.CodeBadRequest, "error.connection_invalid", nil)
		return
	}
	decrypted, err := h.connections.DecryptSecret(conn.ApiSecret)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.connection_fetch_failed", err)
		return
	}
	cred, err := upstream.ParseAgisoCredentials(decrypted)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.connection_invalid", nil)
		return
	}

	client := agiso.NewClient(agiso.Config{
		BaseURL:     conn.BaseURL,
		AppSecret:   cred.AppSecret,
		AccessToken: cred.AccessToken,
		AppID:       conn.ApiKey,
	}, nil)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	// 卡种通常几十个，一次拉满 100 条即可；真超过一页再说，
	// 现在做分页只会让前端下拉框变复杂。
	page, err := client.ListCardKinds(ctx, 1, 100, c.Query("keyword"))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.connection_ping_failed", err)
		return
	}
	items := make([]AgisoCardKindItem, 0, len(page.List))
	for _, k := range page.List {
		items = append(items, AgisoCardKindItem{
			CpkID:     k.IdNo,
			Title:     k.Title,
			CardType:  k.CardType,
			Remaining: k.RemainingCount,
			PurPrice:  strconv.FormatFloat(k.PurPrice, 'f', 2, 64),
			Supported: agiso.SupportedCardType(k.CardType),
		})
	}
	response.Success(c, gin.H{"items": items, "total": page.TotalCount})
}
