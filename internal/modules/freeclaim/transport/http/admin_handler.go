package freeclaimhttp

import (
	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminFreeClaimService 是后台免费领取所需的最小端口。
type AdminFreeClaimService interface {
	GetConfig() (*freeclaimdomain.Config, error)
	UpdateConfig(input freeclaimcontract.ConfigInput) (*freeclaimdomain.Config, error)
	ListPoolItems() ([]freeclaimdomain.PoolItem, error)
	AddPoolItem(input freeclaimcontract.PoolItemInput) (*freeclaimdomain.PoolItem, error)
	UpdatePoolItem(id uint, input freeclaimcontract.PoolItemInput) (*freeclaimdomain.PoolItem, error)
	DeletePoolItem(id uint) error
}

// AdminHandler 处理后台免费领取请求。
type AdminHandler struct {
	claims AdminFreeClaimService
}

// NewAdminHandler 构造后台免费领取处理器。
func NewAdminHandler(claims AdminFreeClaimService) *AdminHandler {
	if claims == nil {
		panic("free claim admin handler: required dependency is nil")
	}
	return &AdminHandler{claims: claims}
}

type updateConfigRequest struct {
	Enabled          bool `json:"enabled"`
	MemberOnly       bool `json:"member_only"`
	MinMemberLevelID uint `json:"min_member_level_id"`
}

type poolItemRequest struct {
	ProductID uint `json:"product_id" binding:"required"`
	SKUID     uint `json:"sku_id" binding:"required"`
	IsActive  bool `json:"is_active"`
	SortOrder int  `json:"sort_order"`
}

// GetConfig 返回免费领取配置。
func (h *AdminHandler) GetConfig(c *gin.Context) {
	config, err := h.claims.GetConfig()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.Success(c, config)
}

// UpdateConfig 保存免费领取配置。
func (h *AdminHandler) UpdateConfig(c *gin.Context) {
	var request updateConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	config, err := h.claims.UpdateConfig(freeclaimcontract.ConfigInput{
		Enabled:          request.Enabled,
		MemberOnly:       request.MemberOnly,
		MinMemberLevelID: request.MinMemberLevelID,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.Success(c, config)
}

// ListPoolItems 返回商品池。
func (h *AdminHandler) ListPoolItems(c *gin.Context) {
	items, err := h.claims.ListPoolItems()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", err)
		return
	}
	response.Success(c, items)
}

// CreatePoolItem 新增商品池条目。
func (h *AdminHandler) CreatePoolItem(c *gin.Context) {
	var request poolItemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	item, err := h.claims.AddPoolItem(freeclaimcontract.PoolItemInput{
		ProductID: request.ProductID,
		SKUID:     request.SKUID,
		IsActive:  request.IsActive,
		SortOrder: request.SortOrder,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	response.Success(c, item)
}

// UpdatePoolItem 修改商品池条目。
func (h *AdminHandler) UpdatePoolItem(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var request poolItemRequest
	if bindErr := c.ShouldBindJSON(&request); bindErr != nil {
		ginutil.RespondBindError(c, bindErr)
		return
	}
	item, updateErr := h.claims.UpdatePoolItem(id, freeclaimcontract.PoolItemInput{
		ProductID: request.ProductID,
		SKUID:     request.SKUID,
		IsActive:  request.IsActive,
		SortOrder: request.SortOrder,
	})
	if updateErr != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", updateErr)
		return
	}
	response.Success(c, item)
}

// DeletePoolItem 删除商品池条目。
func (h *AdminHandler) DeletePoolItem(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if deleteErr := h.claims.DeletePoolItem(id); deleteErr != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.bad_request", deleteErr)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
