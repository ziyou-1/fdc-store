package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/logger"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

// MarkupReapplier 在连接定价配置（汇率/加价/取整）变更后，按新配置重算该连接已映射商品的本地售价。
// 由 Catalog Mapping 应用服务实现，通过 setter 注入以避免与本服务的循环依赖。
type MarkupReapplier interface {
	ReapplyMarkup(connectionID uint) (int, error)
}

// Service 对接连接服务。
type Service struct {
	connRepo        siteconnectioncontract.Repository
	encryptKey      []byte
	uploadsDir      string
	markupReapplier MarkupReapplier
}

// NewService 创建连接服务。
func NewService(connRepo siteconnectioncontract.Repository, appSecretKey, uploadsDir string) *Service {
	return &Service{
		connRepo:   connRepo,
		encryptKey: crypto.DeriveKey(appSecretKey),
		uploadsDir: uploadsDir,
	}
}

// SetMarkupReapplier 注入定价重算器（容器装配时调用）。
func (s *Service) SetMarkupReapplier(r MarkupReapplier) {
	s.markupReapplier = r
}

// Create 创建连接
func (s *Service) Create(input CreateInput) (*siteconnectiondomain.Connection, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.BaseURL) == "" {
		return nil, siteconnectioncontract.ErrInvalid
	}
	if strings.TrimSpace(input.ApiKey) == "" || strings.TrimSpace(input.ApiSecret) == "" {
		return nil, siteconnectioncontract.ErrInvalid
	}

	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = constants.ConnectionProtocolDujiaoNext
	}
	if !isSupportedProtocol(protocol) {
		// 不做白名单的话，协议名打错会静默存下来，直到测试连接时才报
		// "unsupported protocol"，排查成本高。
		return nil, siteconnectioncontract.ErrInvalid
	}

	secretPayload, err := buildSecretPayload(protocol, input.ApiSecret, input.AccessToken)
	if err != nil {
		return nil, err
	}
	encryptedSecret, err := crypto.Encrypt(s.encryptKey, secretPayload)
	if err != nil {
		return nil, err
	}

	retryMax := input.RetryMax
	if retryMax <= 0 {
		retryMax = 5
	}
	retryIntervals := strings.TrimSpace(input.RetryIntervals)
	if retryIntervals == "" {
		retryIntervals = "[30,60,300]"
	}

	roundingMode := strings.TrimSpace(input.PriceRoundingMode)
	if roundingMode == "" {
		roundingMode = "none"
	}

	conn := &siteconnectiondomain.Connection{
		Name:               strings.TrimSpace(input.Name),
		BaseURL:            strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"),
		ApiKey:             strings.TrimSpace(input.ApiKey),
		ApiSecret:          encryptedSecret,
		Protocol:           protocol,
		CallbackURL:        strings.TrimSpace(input.CallbackURL),
		Status:             constants.ConnectionStatusPending,
		RetryMax:           retryMax,
		RetryIntervals:     retryIntervals,
		ExchangeRate:       s.normalizeExchangeRate(input.ExchangeRate),
		PriceMarkupPercent: decimal.NewFromFloat(input.PriceMarkupPercent),
		PriceRoundingMode:  roundingMode,
		AutoSyncPrice:      input.AutoSyncPrice,
	}

	if err := s.connRepo.Create(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// Update 更新连接
func (s *Service) Update(id uint, input UpdateInput) (*siteconnectiondomain.Connection, error) {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}

	// 记录定价配置旧值，用于判断本次保存是否需要重算已映射商品的本地售价。
	prevExchangeRate := conn.ExchangeRate
	prevMarkupPercent := conn.PriceMarkupPercent
	prevRoundingMode := conn.PriceRoundingMode

	if strings.TrimSpace(input.Name) != "" {
		conn.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.BaseURL) != "" {
		conn.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	}
	if strings.TrimSpace(input.ApiKey) != "" {
		conn.ApiKey = strings.TrimSpace(input.ApiKey)
	}
	if strings.TrimSpace(input.Protocol) != "" {
		next := strings.TrimSpace(input.Protocol)
		if !isSupportedProtocol(next) {
			return nil, siteconnectioncontract.ErrInvalid
		}
		conn.Protocol = next
	}
	// 密钥要在协议确定之后再处理：agiso 需要把两个密钥打包，
	// 而打包与否取决于本次更新后的协议值。
	if strings.TrimSpace(input.ApiSecret) != "" || strings.TrimSpace(input.AccessToken) != "" {
		payload, err := buildSecretPayload(conn.Protocol, input.ApiSecret, input.AccessToken)
		if err != nil {
			return nil, err
		}
		encrypted, err := crypto.Encrypt(s.encryptKey, payload)
		if err != nil {
			return nil, err
		}
		conn.ApiSecret = encrypted
	}
	if input.CallbackURL != "" {
		conn.CallbackURL = strings.TrimSpace(input.CallbackURL)
	}
	if input.RetryMax > 0 {
		conn.RetryMax = input.RetryMax
	}
	if strings.TrimSpace(input.RetryIntervals) != "" {
		conn.RetryIntervals = strings.TrimSpace(input.RetryIntervals)
	}
	if input.ExchangeRate != nil {
		conn.ExchangeRate = s.normalizeExchangeRate(*input.ExchangeRate)
	}
	if input.PriceMarkupPercent != nil {
		conn.PriceMarkupPercent = decimal.NewFromFloat(*input.PriceMarkupPercent)
	}
	if input.PriceRoundingMode != nil {
		mode := strings.TrimSpace(*input.PriceRoundingMode)
		if mode == "" {
			mode = "none"
		}
		conn.PriceRoundingMode = mode
	}
	if input.AutoSyncPrice != nil {
		conn.AutoSyncPrice = *input.AutoSyncPrice
	}

	if err := s.connRepo.Update(conn); err != nil {
		return nil, err
	}

	// 定价配置（汇率/加价/取整）发生实际变化时，自动重算该连接已映射商品的本地售价，
	// 避免「改了汇率但已有商品价格不联动」。重算为尽力而为：失败不影响连接保存本身，
	// 仅记录告警，用户仍可通过「重新应用加价」手动补救。
	priceConfigChanged := !conn.ExchangeRate.Equal(prevExchangeRate) ||
		!conn.PriceMarkupPercent.Equal(prevMarkupPercent) ||
		conn.PriceRoundingMode != prevRoundingMode
	if priceConfigChanged && s.markupReapplier != nil {
		if _, err := s.markupReapplier.ReapplyMarkup(conn.ID); err != nil {
			logger.Warnw("reapply_markup_after_connection_update_failed",
				"connection_id", conn.ID, "error", err)
		}
	}

	return conn, nil
}

// Delete 删除连接
func (s *Service) Delete(id uint) error {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return err
	}
	if conn == nil {
		return siteconnectioncontract.ErrNotFound
	}
	return s.connRepo.Delete(id)
}

// GetByID 获取连接
func (s *Service) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	return s.connRepo.GetByID(id)
}

// List 列表查询
func (s *Service) List(filter siteconnectioncontract.ListFilter) ([]siteconnectiondomain.Connection, int64, error) {
	return s.connRepo.List(filter)
}

// SetStatus 设置连接状态
func (s *Service) SetStatus(id uint, status string) error {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return err
	}
	if conn == nil {
		return siteconnectioncontract.ErrNotFound
	}
	conn.Status = status
	return s.connRepo.Update(conn)
}

// Ping 测试连接
func (s *Service) Ping(id uint) (*PingResult, error) {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}

	// 解密 secret
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}

	adapter, err := upstream.NewAdapter(&siteconnectiondomain.Connection{
		BaseURL:   conn.BaseURL,
		ApiKey:    conn.ApiKey,
		ApiSecret: decrypted,
		Protocol:  conn.Protocol,
	}, s.uploadsDir)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, pingErr := adapter.Ping(ctx)
	now := time.Now()
	conn.LastPingAt = &now
	conn.LastPingOK = pingErr == nil

	if pingErr == nil && conn.Status == constants.ConnectionStatusPending {
		conn.Status = constants.ConnectionStatusActive
	}

	// 更新连接状态（不管 ping 是否成功）
	_ = s.connRepo.Update(conn)

	if pingErr != nil {
		return nil, pingErr
	}
	if result == nil {
		return nil, nil
	}
	return &PingResult{
		SiteName:        result.SiteName,
		ProtocolVersion: result.ProtocolVersion,
		UserID:          result.UserID,
		Balance:         result.Balance,
		Currency:        result.Currency,
		MemberLevel:     result.MemberLevel,
	}, nil
}

// GetAdapter 获取连接的适配器（解密 secret 后构建）
func (s *Service) GetAdapter(conn *siteconnectiondomain.Connection) (upstream.Adapter, error) {
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}

	return upstream.NewAdapter(&siteconnectiondomain.Connection{
		BaseURL:   conn.BaseURL,
		ApiKey:    conn.ApiKey,
		ApiSecret: decrypted,
		Protocol:  conn.Protocol,
	}, s.uploadsDir)
}

func (s *Service) decryptSecret(conn *siteconnectiondomain.Connection) (string, error) {
	return crypto.Decrypt(s.encryptKey, conn.ApiSecret)
}

// DecryptSecret 解密加密后的 api_secret（公开方法，用于回调签名验证）
func (s *Service) DecryptSecret(encrypted string) (string, error) {
	return crypto.Decrypt(s.encryptKey, encrypted)
}

// normalizeExchangeRate 规范化汇率值，<=0 时返回 1
func (s *Service) normalizeExchangeRate(rate float64) decimal.Decimal {
	if rate <= 0 {
		return decimal.NewFromInt(1)
	}
	return decimal.NewFromFloat(rate)
}

// isSupportedProtocol 报告协议是否受支持。
func isSupportedProtocol(p string) bool {
	switch p {
	case constants.ConnectionProtocolDujiaoNext, constants.ConnectionProtocolAgiso:
		return true
	default:
		return false
	}
}

// buildSecretPayload 生成待加密的密钥载荷。
//
// dujiao-next 只有一个密钥，原样返回；agiso 需要 AppSecret 与 AccessToken 两个，
// 打包成 JSON 后整体加密——Connection 上只有 ApiSecret 一个加密字段，
// 而 ApiKey 会明文出现在后台响应里，不能拿来放 AccessToken。
func buildSecretPayload(protocol, apiSecret, accessToken string) (string, error) {
	if protocol != constants.ConnectionProtocolAgiso {
		return apiSecret, nil
	}
	packed, err := upstream.EncodeAgisoCredentials(apiSecret, accessToken)
	if err != nil {
		return "", err
	}
	// 保存时就校验两个密钥齐全。只在读取时校验的话，缺 AccessToken 的连接能存进去，
	// 要等到点「测试连接」才报错，而且报的是底层的凭据错误，运营看不懂。
	if _, err := upstream.ParseAgisoCredentials(packed); err != nil {
		// 归到 ErrInvalid：handler 只把它映射成 400，其余错误一律 500。
		// 少填一个密钥属于用户输入问题，不该报成服务器错误。
		return "", fmt.Errorf("%w: %v", siteconnectioncontract.ErrInvalid, err)
	}
	return packed, nil
}
