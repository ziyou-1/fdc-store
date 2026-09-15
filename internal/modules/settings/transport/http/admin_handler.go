package settingshttp

import (
	"strings"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台通用设置端口。
type AdminService interface {
	GetByKey(key string) (jsonmap.JSON, error)
	UpdateWithEffects(key string, value map[string]interface{}) (settingsapp.UpdateResult, error)
	InvalidateCallbackRoutesCache()
}

// AdminHandler 处理后台通用设置请求。
type AdminHandler struct {
	settings AdminService
}

func NewAdminHandler(settings AdminService) *AdminHandler {
	if settings == nil {
		panic("settings admin handler: settings is nil")
	}
	return &AdminHandler{settings: settings}
}

type updateRequest struct {
	Key   string                 `json:"key" binding:"required"`
	Value map[string]interface{} `json:"value" binding:"required"`
}

// dedicatedSecretSettingKeys 是含密钥、且已有专用脱敏接口的设置键。
//
// 通用接口原样返回 settings 表里的整块 JSON，不做任何脱敏，
// 于是 GET /admin/settings?key=smtp_config 能直接读出 SMTP 明文密码——
// 专用接口辛苦做的 has_password / has_secret 脱敏被整体绕过。
// 这些键一律改走各自的专用接口，那里才有 Mask*ForAdmin。
var dedicatedSecretSettingKeys = map[string]string{
	constants.SettingKeySMTPConfig:         "/admin/settings/smtp",
	constants.SettingKeyCaptchaConfig:      "/admin/settings/captcha",
	constants.SettingKeyTelegramAuthConfig: "/admin/settings/telegram-auth",
	constants.SettingKeyGoogleAuthConfig:   "/admin/settings/google-auth",
	constants.SettingKeyDeepLConfig:        "/admin/settings/deepl",
	// 通知中心里存着飞书 App Secret，Mask 里同样置空（notification.go 的
	// feishu["app_secret"] = ""），所以它也必须走专用接口。
	constants.SettingKeyNotificationCenterConfig: "/admin/settings/notification-center",
}

// Get 获取设置。
func (h *AdminHandler) Get(c *gin.Context) {
	key := c.DefaultQuery("key", constants.SettingKeySiteConfig)

	requestedKey := strings.TrimSpace(key)
	if dedicated, exists := dedicatedSecretSettingKeys[requestedKey]; exists {
		ginutil.RespondErrorWithMsg(
			c,
			response.CodeBadRequest,
			requestedKey+" must be read through "+dedicated,
			nil,
		)
		return
	}

	value, err := h.settings.GetByKey(key)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	if value == nil {
		response.Success(c, gin.H{})
		return
	}

	response.Success(c, value)
}

// Update 更新设置。
func (h *AdminHandler) Update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	// 写入侧同理：这些键有各自的校验与 ApplyRuntime（热生效），
	// 从通用接口直写会跳过校验、也不会推给正在跑的服务。
	// 原先这里只挡了 google_auth_config，其余四个都是敞开的。
	//
	// 这不是架构守卫要禁的"按 key 分发业务逻辑"——业务效果仍然全部由
	// UpdateWithEffects 返回的 registry effects 驱动；这里只是把几个
	// 有专用脱敏接口的键挡在通用入口之外，属于访问边界，不是分发。
	requestedKey := strings.TrimSpace(req.Key)
	if dedicated, exists := dedicatedSecretSettingKeys[requestedKey]; exists {
		ginutil.RespondErrorWithMsg(
			c,
			response.CodeBadRequest,
			requestedKey+" must be updated through "+dedicated,
			nil,
		)
		return
	}

	result, err := h.settings.UpdateWithEffects(req.Key, req.Value)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}

	if result.HasEffect(settingsapp.EffectInvalidatePublicConfigCache) {
		_ = cache.DelAllPublicConfig(c.Request.Context())
	}
	if result.HasEffect(settingsapp.EffectInvalidateCallbackRoutesCache) {
		h.settings.InvalidateCallbackRoutesCache()
	}
	response.Success(c, result.Value)
}
