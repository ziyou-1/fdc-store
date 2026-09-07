package memberlevelhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/member-levels", handler.GetAdminMemberLevels)
	admin.POST("/member-levels", handler.CreateMemberLevel)
	admin.PUT("/member-levels/:id", handler.UpdateMemberLevel)
	admin.DELETE("/member-levels/:id", handler.DeleteMemberLevel)
	admin.GET("/member-level-prices", handler.GetMemberLevelPrices)
	admin.POST("/member-level-prices/batch", handler.BatchUpsertMemberLevelPrices)
	admin.DELETE("/member-level-prices/:id", handler.DeleteMemberLevelPrice)
	admin.POST("/member-levels/backfill", handler.BackfillMemberLevels)
	admin.PUT("/users/:id/member-level", handler.SetUserMemberLevel)
	admin.GET("/membership-plans", handler.GetMembershipPlans)
	admin.PUT("/membership-plans/:period", handler.UpdateMembershipPlan)
}

func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	public.GET("/member-levels", handler.List)
	public.GET("/membership-plans", handler.ListPlans)
}

func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	user.POST("/membership/purchase", handler.CreatePurchase)
}

func RegisterChannelRoutes(channel gin.IRoutes, handler *ChannelHandler) {
	channel.GET("/member-levels", handler.List)
}
