package migrations

import (
	"time"

	"github.com/dujiao-next/internal/constants"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"
	auditlogdomain "github.com/dujiao-next/internal/modules/auditlog/domain"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cartdomain "github.com/dujiao-next/internal/modules/cart/domain"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	channelclientdomain "github.com/dujiao-next/internal/modules/channelclient/domain"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	downstreamcallbackdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
	freeclaimdomain "github.com/dujiao-next/internal/modules/freeclaim/domain"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	notificationdomain "github.com/dujiao-next/internal/modules/notification/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderriskcontract "github.com/dujiao-next/internal/modules/orderrisk/contract"
	orderriskdomain "github.com/dujiao-next/internal/modules/orderrisk/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	pointsdomain "github.com/dujiao-next/internal/modules/points/domain"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
	resellerstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	supportdomain "github.com/dujiao-next/internal/modules/support/domain"
	broadcastdomain "github.com/dujiao-next/internal/modules/telegram/broadcast/domain"
	tokenstationdomain "github.com/dujiao-next/internal/modules/tokenstation/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/platform/database/gormdb"

	"gorm.io/gorm"
)

// AutoMigrate owns the application-wide schema registry and ordered data migrations.
func AutoMigrate() error {
	db := gormdb.DB
	if err := db.AutoMigrate(
		&admindomain.Admin{},
		&userdomain.User{},
		&externalidentitydomain.Identity{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Click{},
		&affiliatedomain.Commission{},
		&affiliatedomain.WithdrawRequest{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.RechargeOrder{},
		&pointsdomain.Account{},
		&pointsdomain.Transaction{},
		&pointsdomain.Checkin{},
		&pointsdomain.Config{},
		&freeclaimdomain.PoolItem{},
		&freeclaimdomain.Claim{},
		&freeclaimdomain.Config{},
		&tokenstationdomain.Station{},
		&supportdomain.Conversation{},
		&supportdomain.Message{},
		&supportdomain.QuickReply{},
		&auditlogdomain.UserLoginLog{},
		&auditlogdomain.AuthzAuditLog{},
		&notificationdomain.NotificationLog{},
		&auditlogdomain.AdminLoginLog{},
		&emailverificationdomain.Code{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&orderriskdomain.LockKey{},
		&cartdomain.Item{},
		&paymentdomain.PaymentChannel{},
		&paymentdomain.Payment{},
		&cardsecretdomain.Secret{},
		&cardsecretdomain.Batch{},
		&giftcarddomain.GiftCard{},
		&giftcarddomain.GiftCardBatch{},
		&fulfillmentdomain.Fulfillment{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&contentdomain.Post{},
		&contentdomain.PostProduct{},
		&contentdomain.PostCategory{},
		&contentdomain.Banner{},
		&settingsstore.SettingRecord{},
		&apicredentialdomain.ApiCredential{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
		&procurementdomain.Order{},
		&downstreamcallbackdomain.OrderRef{},
		&reconciliationdomain.Job{},
		&reconciliationdomain.Item{},
		&channelclientdomain.Client{},
		&broadcastdomain.Broadcast{},
		&memberleveldomain.MemberLevel{},
		&memberleveldomain.MemberLevelPrice{},
		&memberleveldomain.Plan{},
		&contentdomain.Media{},
	); err != nil {
		return err
	}

	if err := ensureUserOAuthIdentityUserProviderUniqueIndex(); err != nil {
		return err
	}
	if err := backfillPendingOrderRiskIPs(db); err != nil {
		return err
	}
	if err := resellerstore.Migrate(db); err != nil {
		return err
	}
	if err := migrateCartSKUUniqueIndex(); err != nil {
		return err
	}
	if err := ensureProductSKUMigration(); err != nil {
		return err
	}
	if err := ensureManualStockRemainingMigration(); err != nil {
		return err
	}
	if err := ensureCategoryParentMigration(); err != nil {
		return err
	}
	if err := ensurePaymentProviderBepusdtRenameMigration(); err != nil {
		return err
	}
	if err := ensurePaymentChannelBepusdtConfigMigration(); err != nil {
		return err
	}
	if err := ensurePaymentFeePolicyMigration(); err != nil {
		return err
	}
	if err := ensureOrderRefundPaymentFeeMigration(); err != nil {
		return err
	}
	if err := ensureOrderItemOriginalPriceMigration(); err != nil {
		return err
	}
	if err := ensureCartForeignKeyConstraints(); err != nil {
		return err
	}
	if err := ensureProcurementOrderForeignKeyConstraint(); err != nil {
		return err
	}
	if db.Migrator().HasColumn(&productdomain.Product{}, "price_currency") {
		if err := db.Migrator().DropColumn(&productdomain.Product{}, "price_currency"); err != nil {
			return err
		}
	}
	if err := ensureTokenStationLinkOnlyMigration(db); err != nil {
		return err
	}
	if err := ensurePayssionPaymentChannels(db); err != nil {
		return err
	}
	return nil
}

type tokenStationLegacyColumns struct {
	AdminEmail       string     `gorm:"column:admin_email"`
	AdminPasswordEnc string     `gorm:"column:admin_password_enc"`
	Status           string     `gorm:"column:status"`
	LastPingOK       bool       `gorm:"column:last_ping_ok"`
	LastPingAt       *time.Time `gorm:"column:last_ping_at"`
	LastError        string     `gorm:"column:last_error"`
}

func (tokenStationLegacyColumns) TableName() string { return "token_stations" }

func ensureTokenStationLinkOnlyMigration(db *gorm.DB) error {
	if db.Migrator().HasTable("token_station_offers") {
		if err := db.Migrator().DropTable("token_station_offers"); err != nil {
			return err
		}
	}
	legacy := &tokenStationLegacyColumns{}
	for _, col := range []string{"admin_email", "admin_password_enc", "status", "last_ping_ok", "last_ping_at", "last_error"} {
		if db.Migrator().HasColumn(legacy, col) {
			if err := db.Migrator().DropColumn(legacy, col); err != nil {
				return err
			}
		}
	}
	if !db.Migrator().HasTable("token_stations") {
		return nil
	}
	return db.Exec("DELETE FROM token_stations WHERE base_url IN (?, ?)", "https://example.com", "https://example.com/").Error
}

// backfillPendingOrderRiskIPs 只迁移仍可能占用库存的在途订单，避免启动时扫描全部历史订单。
func backfillPendingOrderRiskIPs(db *gorm.DB) error {
	type orderIPRow struct {
		ID       uint
		ClientIP string
	}
	var rows []orderIPRow
	if err := db.Model(&orderdomain.Order{}).
		Select("id", "client_ip").
		Where("deleted_at IS NULL AND status = ? AND (risk_ip IS NULL OR risk_ip = '') AND client_ip <> ''", constants.OrderStatusPendingPayment).
		Find(&rows).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			riskIP := orderriskcontract.NormalizeRiskIP(row.ClientIP)
			if riskIP == "" {
				continue
			}
			if err := tx.Model(&orderdomain.Order{}).
				Where("id = ? AND (risk_ip IS NULL OR risk_ip = '')", row.ID).
				UpdateColumn("risk_ip", riskIP).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
