package application

import (
	"strings"

	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
)

// GetInvite 返回当前用户的推荐入口。关闭时不签发邀请码。
func (s *Service) GetInvite(userID uint) (*pointscontract.Invite, error) {
	if userID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	config, err := s.GetConfig()
	if err != nil {
		return nil, err
	}
	return buildInvite(userID, config, s.inviteIssuer)
}

func buildInvite(userID uint, config *pointsdomain.Config, issuer pointscontract.InviteCodeIssuer) (*pointscontract.Invite, error) {
	if config == nil {
		return nil, pointscontract.ErrInvalidConfig
	}
	if !config.ReferralEnabled {
		return &pointscontract.Invite{}, nil
	}
	if issuer == nil {
		return nil, pointscontract.ErrInviteIssuerRequired
	}
	code, err := issuer.EnsureInviteCode(userID)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return nil, pointscontract.ErrInviteCodeEmpty
	}
	return &pointscontract.Invite{
		Enabled:          true,
		Code:             trimmed,
		RegisterPoints:   config.ReferralRegisterPoints,
		FirstOrderPoints: config.ReferralFirstOrderPoints,
	}, nil
}
