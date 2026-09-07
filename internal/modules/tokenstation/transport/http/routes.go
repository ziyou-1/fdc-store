package tokenstationhttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册首页别人中转站链接。
func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	if public == nil || handler == nil {
		panic("token station public routes: required dependency is nil")
	}
	public.GET("/token-stations", handler.Catalog)
}

// RegisterAdminRoutes 注册后台别人中转站链接。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("token station admin routes: required dependency is nil")
	}
	admin.GET("/token-stations", handler.ListStations)
	admin.POST("/token-stations", handler.CreateStation)
	admin.PUT("/token-stations/:id", handler.UpdateStation)
	admin.DELETE("/token-stations/:id", handler.DeleteStation)
}
