package settingshttp

import (
	"errors"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// VerifyEmailTemplateAdminService 是后台验证码邮件模板设置端口。
type VerifyEmailTemplateAdminService interface {
	GetVerifyEmailTemplateSetting() (settingsmessaging.VerifyEmailTemplateSetting, error)
	PatchVerifyEmailTemplateSetting(patch settingsmessaging.VerifyEmailTemplateSettingPatch) (settingsmessaging.VerifyEmailTemplateSetting, error)
	ResetVerifyEmailTemplateSetting() (settingsmessaging.VerifyEmailTemplateSetting, error)
}

// VerifyEmailTemplateHandler 处理后台验证码邮件模板设置请求。
type VerifyEmailTemplateHandler struct {
	templates VerifyEmailTemplateAdminService
}

func NewVerifyEmailTemplateHandler(templates VerifyEmailTemplateAdminService) *VerifyEmailTemplateHandler {
	if templates == nil {
		panic("settings verify-email-template handler: templates is nil")
	}
	return &VerifyEmailTemplateHandler{templates: templates}
}

// verifyEmailTemplateResponse 附带场景与语言清单，前端据此渲染 Tab，
// 不必自己再写死一份场景/语言列表。
func verifyEmailTemplateResponse(setting settingsmessaging.VerifyEmailTemplateSetting) gin.H {
	return gin.H{
		"templates": setting.Templates,
		"scenes":    settingsmessaging.VerifyEmailScenes(),
		"locales":   append([]string(nil), constants.SupportedLocales...),
		"variables": []string{"code", "site_name", "site_url"},
	}
}

// GetVerifyEmailTemplate 获取验证码邮件模板配置。
func (h *VerifyEmailTemplateHandler) GetVerifyEmailTemplate(c *gin.Context) {
	setting, err := h.templates.GetVerifyEmailTemplateSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, verifyEmailTemplateResponse(setting))
}

// UpdateVerifyEmailTemplate 更新验证码邮件模板配置。
func (h *VerifyEmailTemplateHandler) UpdateVerifyEmailTemplate(c *gin.Context) {
	var req settingsmessaging.VerifyEmailTemplateSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.templates.PatchVerifyEmailTemplateSetting(req)
	if err != nil {
		switch {
		case errors.Is(err, settingsmessaging.ErrVerifyEmailTemplateConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}
	response.Success(c, verifyEmailTemplateResponse(setting))
}

// ResetVerifyEmailTemplate 把验证码邮件模板重置为内置默认文案。
func (h *VerifyEmailTemplateHandler) ResetVerifyEmailTemplate(c *gin.Context) {
	setting, err := h.templates.ResetVerifyEmailTemplateSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	response.Success(c, verifyEmailTemplateResponse(setting))
}
