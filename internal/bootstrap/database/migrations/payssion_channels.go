package migrations

import (
	"github.com/dujiao-next/internal/constants"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type payssionChannelSeed struct {
	Name        string
	ChannelType string
	FeeRate     string
	SortOrder   int
}

func payssionChannelSeeds() []payssionChannelSeed {
	return []payssionChannelSeed{
		{Name: "支付宝支付", ChannelType: "alipay_cn", SortOrder: 10},
		{Name: "Touch 'n Go-MY", ChannelType: "touchngo_my", SortOrder: 20},
		{Name: "Doku-ID", ChannelType: "doku_id", FeeRate: "3.50", SortOrder: 30},
		{Name: "Gcash-PH", ChannelType: "gcash_ph", SortOrder: 40},
		{Name: "Promptpay-TH", ChannelType: "promptpay_th", SortOrder: 50},
		{Name: "Picpay-BR", ChannelType: "picpay_br", SortOrder: 60},
		{Name: "Qris-ID", ChannelType: "qris_id", FeeRate: "2.50", SortOrder: 70},
		{Name: "OVO-ID", ChannelType: "ovo_id", FeeRate: "4.50", SortOrder: 80},
		{Name: "Dana-ID", ChannelType: "dana_id", FeeRate: "3.50", SortOrder: 90},
		{Name: "LinkAja-ID", ChannelType: "linkaja_id", FeeRate: "3.50", SortOrder: 100},
		{Name: "ShopeePay-ID", ChannelType: "shopeepay_id", FeeRate: "3.50", SortOrder: 110},
		{Name: "ATM-ID", ChannelType: "atm_id", FeeRate: "2.50", SortOrder: 120},
		{Name: "Akulaku-ID", ChannelType: "akulaku_id", FeeRate: "3.75", SortOrder: 130},
		{Name: "Credit Card-ID", ChannelType: "creditcard_id", FeeRate: "4.25", SortOrder: 140},
		{Name: "Alfamart-ID", ChannelType: "alfamart_id", FeeRate: "2.50", SortOrder: 150},
	}
}

func ensurePayssionPaymentChannels(db *gorm.DB) error {
	for _, seed := range payssionChannelSeeds() {
		// 刻意不带 deleted_at IS NULL：软删除的行也算「已存在」。
		// 管理员在后台删掉的渠道不能被这里复活——原先带了这个条件，被删的渠道会在
		// 下次重启时以「启用」状态重新建出来，运营以为删干净了，买家结算页却又能选到它。
		// （PaymentChannel.DeletedAt 是 *time.Time 而非 gorm.DeletedAt，
		//   GORM 不做自动软删除，所以这里也用不上 Unscoped。）
		var count int64
		if err := db.Model(&paymentdomain.PaymentChannel{}).
			Where("provider_type = ? AND channel_type = ?", constants.PaymentProviderPayssion, seed.ChannelType).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		feeRate := "0"
		if seed.FeeRate != "" {
			feeRate = seed.FeeRate
		}
		channel := paymentdomain.PaymentChannel{
			Name:            seed.Name,
			ProviderType:    constants.PaymentProviderPayssion,
			ChannelType:     seed.ChannelType,
			InteractionMode: constants.PaymentInteractionRedirect,
			FeeRate:         money.FromDecimal(decimal.RequireFromString(feeRate)),
			FixedFee:        money.FromDecimal(decimal.Zero),
			MinAmount:       money.FromDecimal(decimal.Zero),
			MaxAmount:       money.FromDecimal(decimal.Zero),
			PaymentTypes:    jsonslice.Strings{"order", "wallet"},
			ConfigJSON:      jsonmap.JSON{"live_mode": true},
			IsActive:        true,
			SortOrder:       seed.SortOrder,
		}
		if err := db.Create(&channel).Error; err != nil {
			return err
		}
	}
	return nil
}
