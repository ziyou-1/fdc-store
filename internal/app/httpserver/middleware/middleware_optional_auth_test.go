package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/identity/jwttoken"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/modules/identity/userauth/challenge"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// runOptionalMiddleware 跑一遍 OptionalUserJWTMiddleware，返回 HTTP 状态码
// 与 handler 观察到的 user_id（0 表示中间件未写入身份，即按匿名放行）。
func runOptionalMiddleware(t *testing.T, repo usercontract.Store, token string) (int, uint) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalUserJWTMiddleware(middlewareTestSecret, repo))
	r.GET("/public/ping", func(c *gin.Context) {
		var uid uint
		if raw, exists := c.Get("user_id"); exists {
			if value, ok := raw.(uint); ok {
				uid = value
			}
		}
		c.JSON(http.StatusOK, gin.H{"user_id": uid})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/ping", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	uid, _ := resp["user_id"].(float64)
	return w.Code, uint(uid)
}

func validUserClaims(userID uint, email string, tokenVersion uint64) userauthapp.UserJWTClaims {
	return userauthapp.UserJWTClaims{
		UserID:       userID,
		Email:        email,
		TokenVersion: tokenVersion,
		Typ:          jwttoken.TypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
}

func TestOptionalAuthAllowsAnonymous(t *testing.T) {
	repo, _ := setupUserMiddlewareTestRepo(t)
	status, uid := runOptionalMiddleware(t, repo, "")
	if status != http.StatusOK {
		t.Fatalf("游客必须放行，got HTTP %d", status)
	}
	if uid != 0 {
		t.Fatalf("游客不该带出 user_id，got %d", uid)
	}
}

func TestOptionalAuthPopulatesIdentityForValidToken(t *testing.T) {
	repo, user := setupUserMiddlewareTestRepo(t)
	token := signToken(t, validUserClaims(user.ID, user.Email, user.TokenVersion))
	status, uid := runOptionalMiddleware(t, repo, token)
	if status != http.StatusOK {
		t.Fatalf("有效凭证必须放行，got HTTP %d", status)
	}
	if uid != user.ID {
		t.Fatalf("有效凭证必须写入 user_id，want %d got %d", user.ID, uid)
	}
}

// 公开页面不该因为一个坏掉的 token 就打不开：一律降级为匿名，不 401。
func TestOptionalAuthFallsBackToAnonymousOnBadToken(t *testing.T) {
	repo, user := setupUserMiddlewareTestRepo(t)

	expired := validUserClaims(user.ID, user.Email, user.TokenVersion)
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))

	staleVersion := validUserClaims(user.ID, user.Email, user.TokenVersion+1)

	unknownUser := validUserClaims(user.ID+9999, "ghost@example.com", 0)

	challengeClaims := userauthapp.UserChallengeClaims{
		UserID:  user.ID,
		JTI:     "challenge-jti",
		Purpose: challenge.PurposeTwoFactor,
		Typ:     jwttoken.TypeTwoFactorChallenge,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "challenge-jti",
		},
	}

	wrongSecretToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		validUserClaims(user.ID, user.Email, user.TokenVersion),
	).SignedString([]byte("a-different-secret"))
	if err != nil {
		t.Fatalf("sign with wrong secret: %v", err)
	}

	cases := []struct {
		name  string
		token string
	}{
		{"格式错误", "not-a-jwt"},
		{"签名错误", wrongSecretToken},
		{"已过期", signToken(t, expired)},
		{"token 版本已失效", signToken(t, staleVersion)},
		{"用户不存在", signToken(t, unknownUser)},
		{"二次验证挑战 token", signToken(t, challengeClaims)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, uid := runOptionalMiddleware(t, repo, tc.token)
			if status != http.StatusOK {
				t.Fatalf("坏凭证必须按匿名放行而不是拦截，got HTTP %d", status)
			}
			if uid != 0 {
				t.Fatalf("坏凭证不该写入身份，got user_id=%d", uid)
			}
		})
	}
}

// 缺少密钥属于配置问题，可选鉴权同样按匿名放行，不能让公开页面整体不可用。
func TestOptionalAuthAnonymousWhenMisconfigured(t *testing.T) {
	repo, user := setupUserMiddlewareTestRepo(t)
	token := signToken(t, validUserClaims(user.ID, user.Email, user.TokenVersion))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalUserJWTMiddleware("", repo))
	r.GET("/public/ping", func(c *gin.Context) {
		_, exists := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{"has_user": exists})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("密钥缺失时公开端点仍必须可用，got HTTP %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if has, _ := resp["has_user"].(bool); has {
		t.Fatal("密钥缺失时不得写入任何身份")
	}
}
