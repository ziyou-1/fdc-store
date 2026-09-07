package freeclaimhttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开的今日免费商品端点。
// 首页需要在未登录时也能展示"今日免费领取"，登录态只影响 claimed 字段。
func RegisterPublicRoutes(public gin.IRoutes, handler *UserHandler) {
	if public == nil || handler == nil {
		panic("free claim public routes: required dependency is nil")
	}
	public.GET("/free-claim/today", handler.GetToday)
}

// RegisterUserRoutes 注册用户免费领取端点。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("free claim user routes: required dependency is nil")
	}
	user.POST("/free-claim", handler.Claim)
	user.GET("/free-claim/records", handler.ListClaims)
}

// RegisterAdminRoutes 注册后台免费领取端点。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("free claim admin routes: required dependency is nil")
	}
	admin.GET("/free-claim/config", handler.GetConfig)
	admin.PUT("/free-claim/config", handler.UpdateConfig)
	admin.GET("/free-claim/pool", handler.ListPoolItems)
	admin.POST("/free-claim/pool", handler.CreatePoolItem)
	admin.PUT("/free-claim/pool/:id", handler.UpdatePoolItem)
	admin.DELETE("/free-claim/pool/:id", handler.DeletePoolItem)
}
