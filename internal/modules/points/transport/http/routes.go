package pointshttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册用户积分端点。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("points user routes: required dependency is nil")
	}
	user.GET("/points", handler.GetPoints)
	user.GET("/points/config", handler.GetConfig)
	user.GET("/points/invite", handler.GetInvite)
	user.GET("/points/transactions", handler.GetTransactions)
	user.GET("/points/checkin", handler.GetCheckinStatus)
	user.POST("/points/checkin", handler.Checkin)
	user.POST("/points/recharge", handler.CreateRecharge)
	user.GET("/points/recharges", handler.ListRecharges)
}

// RegisterAdminRoutes 注册后台积分端点。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("points admin routes: required dependency is nil")
	}
	admin.GET("/users/:id/points", handler.GetUserPoints)
	admin.GET("/users/:id/points/transactions", handler.GetUserTransactions)
	admin.POST("/users/:id/points/adjust", handler.AdjustUserPoints)
	admin.GET("/points/config", handler.GetConfig)
	admin.PUT("/points/config", handler.UpdateConfig)
}
