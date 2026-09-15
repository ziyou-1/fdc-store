package settingshttp

import (
	"errors"

	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// DeepLAdminService 是后台 DeepL 翻译设置端口。
type DeepLAdminService interface {
	GetDeepLSetting() (settingssecurity.DeepLSetting, error)
	PatchDeepLSetting(patch settingssecurity.DeepLSettingPatch) (settingssecurity.DeepLSetting, error)
	ApplyRuntime(setting settingssecurity.DeepLSetting)
	// VerifyAuthKey 用给定密钥试译一次。空串表示用当前已保存的密钥。
	VerifyAuthKey(authKey string) error
}

// DeepLHandler 处理后台 DeepL 翻译设置请求。
type DeepLHandler struct {
	deepl DeepLAdminService
}

func NewDeepLHandler(deepl DeepLAdminService) *DeepLHandler {
	if deepl == nil {
		panic("settings deepl handler: deepl is nil")
	}
	return &DeepLHandler{deepl: deepl}
}

// GetDeepL 获取 DeepL 配置（密钥脱敏）。
func (h *DeepLHandler) GetDeepL(c *gin.Context) {
	setting, err := h.deepl.GetDeepLSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingssecurity.MaskDeepLSettingForAdmin(setting))
}

// UpdateDeepL 更新 DeepL 配置。
func (h *DeepLHandler) UpdateDeepL(c *gin.Context) {
	var req settingssecurity.DeepLSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.deepl.PatchDeepLSetting(req)
	if err != nil {
		switch {
		case errors.Is(err, settingssecurity.ErrDeepLConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}

	h.deepl.ApplyRuntime(setting)
	response.Success(c, settingssecurity.MaskDeepLSettingForAdmin(setting))
}

type deepLTestRequest struct {
	// AuthKey 留空表示用当前已保存的密钥自测。
	// 前端表单里密钥框默认是空的（读接口不回显），留空即"测已存的那把"。
	AuthKey string `json:"auth_key"`
}

// TestDeepL 试译一次，用来在保存前验证密钥是否可用。
//
// 翻译在业务侧是 fail-closed 的：译不出来买家就发不出客服文本消息。
// 密钥一旦存错，代价是整站客服文本不可用，所以给运营一个先验后存的入口。
func (h *DeepLHandler) TestDeepL(c *gin.Context) {
	var req deepLTestRequest
	// 允许空 body：不带 JSON 时就是"测当前已保存的密钥"。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			ginutil.RespondBindError(c, err)
			return
		}
	}

	if err := h.deepl.VerifyAuthKey(req.AuthKey); err != nil {
		// 这里回显 DeepL 的原始错误（403 密钥无效 / 456 配额耗尽），
		// 让运营一眼看出是密钥打错了还是额度用光了。
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
