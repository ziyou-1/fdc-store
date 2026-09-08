package application

import (
	"context"
	"time"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

// VerificationEmailSender defines the only email capability required by user
// authentication. Infrastructure implementations are supplied by composition.
type VerificationEmailSender interface {
	// tmpl 为 nil 时使用内置默认文案。刻意把模板做成参数而不是另开一个
	// SendVerifyCodeWithTemplate：留两条路径迟早会有人走到不读模板的那条。
	SendVerifyCode(toEmail, code, purpose, locale string, brand mailbrand.Brand, tmpl *settingsmessaging.VerifyEmailTemplateSetting) error
}

// AuthTransaction is the persistence surface required by cross-aggregate
// authentication mutations. Implementations must execute every method on the
// same database transaction.
type AuthTransaction interface {
	GetUserByEmail(email string) (*userdomain.User, error)
	GetUserByIDForUpdate(userID uint) (*userdomain.User, error)
	CreateUser(user *userdomain.User) error
	UpdateUser(user *userdomain.User) error
	GetIdentityByProviderUserID(provider, providerUserID string) (*externalidentitydomain.Identity, error)
	GetIdentityByUserProvider(userID uint, provider string) (*externalidentitydomain.Identity, error)
	ListIdentitiesByUserID(userID uint) ([]externalidentitydomain.Identity, error)
	CreateIdentity(identity *externalidentitydomain.Identity) error
	UpdateIdentity(identity *externalidentitydomain.Identity) error
	DeleteIdentityByID(identityID uint) error
}

// AuthUnitOfWork provides the atomic boundary shared by users and external
// identities. The callback is committed only when it returns nil.
type AuthUnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(AuthTransaction) error) error
}

// GoogleRedirectStore persists short-lived, single-use redirect state. Take
// operations must atomically read and delete the value.
type GoogleRedirectStore interface {
	PutIntent(ctx context.Context, state string, intent GoogleRedirectIntent, ttl time.Duration) error
	TakeIntent(ctx context.Context, state string) (*GoogleRedirectIntent, error)
	PutHandoff(ctx context.Context, handle string, handoff GoogleRedirectHandoff, ttl time.Duration) error
	TakeHandoff(ctx context.Context, handle string) (*GoogleRedirectHandoff, error)
}

// GoogleRedirectHandoff stores verified claims only. Raw Google credentials
// must never cross this application boundary into the redirect state store.
type GoogleRedirectHandoff struct {
	Flow      string
	UserID    uint
	Tenant    GoogleRedirectTenant
	Identity  googleauthapp.VerifiedIdentity
	CreatedAt time.Time
}

// InviterResolver 把邀请码解析成推荐人的用户 ID。
// 邀请码复用 affiliate_profiles.affiliate_code，避免用户手上有两套码。
type InviterResolver interface {
	ResolveInviterUserID(code string) (uint, error)
}

// ReferralRewarder 发放推荐注册奖励。奖励失败不影响注册本身。
type ReferralRewarder interface {
	AwardReferralRegister(inviterUserID, inviteeUserID uint) error
}
