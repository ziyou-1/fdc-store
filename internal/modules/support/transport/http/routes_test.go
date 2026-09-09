package supporthttp

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// 买家路由里 /support/messages/read（静态段）与 /support/messages/:id/revoke
// （参数段）在同一层。gin 的路由树对这种组合历史上会 panic，且是注册时
// 才炸——也就是进程一起来就崩，没有测试就只能靠上线才发现。
func TestUserRouteTreeAllowsRevokeBesideRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("gin 拒绝静态段与参数段共存: %v", r)
		}
	}()
	r := gin.New()
	r.GET("/support/messages", func(*gin.Context) {})
	r.POST("/support/messages", func(*gin.Context) {})
	r.POST("/support/messages/read", func(*gin.Context) {})
	r.POST("/support/messages/:id/revoke", func(*gin.Context) {})
}
