package migrations

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPayssionSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:payssion_seed_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&paymentdomain.PaymentChannel{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func countChannels(t *testing.T, db *gorm.DB, channelType string, unscoped bool) int64 {
	t.Helper()
	q := db.Model(&paymentdomain.PaymentChannel{}).Where("deleted_at IS NULL")
	if unscoped {
		q = db.Model(&paymentdomain.PaymentChannel{})
	}
	var n int64
	if err := q.Where("channel_type = ?", channelType).Count(&n).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	return n
}

// 管理员在后台删掉的渠道不能被种子复活。
// 曾经的行为是：种子的存在性判断带 deleted_at IS NULL，于是被删的渠道在下次重启时
// 会以「启用」状态重新建出来——运营以为删干净了，买家结算页却又能选到它。
func TestPayssionSeedDoesNotResurrectDeletedChannels(t *testing.T) {
	db := setupPayssionSeedTestDB(t)

	if err := ensurePayssionPaymentChannels(db); err != nil {
		t.Fatalf("first seed failed: %v", err)
	}
	seeded := len(payssionChannelSeeds())
	if got := countChannels(t, db, "doku_id", false); got != 1 {
		t.Fatalf("首次播种后 doku_id 应有 1 条，实得 %d", got)
	}

	// 模拟管理员在后台删除。注意 PaymentChannel.DeletedAt 是 *time.Time 而不是
	// gorm.DeletedAt，GORM 不会自动软删除；应用层是手工置 deleted_at 的（见
	// ChannelStore.Delete），测试必须照同样的方式来，调 db.Delete 会变成硬删除。
	if err := db.Model(&paymentdomain.PaymentChannel{}).
		Where("channel_type = ?", "doku_id").
		Update("deleted_at", time.Now()).Error; err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}

	// 重启会再跑一次种子
	if err := ensurePayssionPaymentChannels(db); err != nil {
		t.Fatalf("second seed failed: %v", err)
	}
	if got := countChannels(t, db, "doku_id", false); got != 0 {
		t.Fatalf("被删除的渠道被种子复活了：doku_id 活跃行数 %d，应为 0", got)
	}
	if got := countChannels(t, db, "doku_id", true); got != 1 {
		t.Fatalf("不该产生重复行：doku_id 总行数 %d，应为 1（仅那条软删除的）", got)
	}

	var total int64
	if err := db.Model(&paymentdomain.PaymentChannel{}).Count(&total).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if int(total) != seeded {
		t.Fatalf("重复播种后总行数 %d，应保持 %d", total, seeded)
	}
}

// 正常情况下重复播种应当幂等。
func TestPayssionSeedIsIdempotent(t *testing.T) {
	db := setupPayssionSeedTestDB(t)
	for i := 0; i < 3; i++ {
		if err := ensurePayssionPaymentChannels(db); err != nil {
			t.Fatalf("seed %d failed: %v", i, err)
		}
	}
	var total int64
	db.Model(&paymentdomain.PaymentChannel{}).Count(&total)
	if int(total) != len(payssionChannelSeeds()) {
		t.Fatalf("三次播种后总行数 %d，应为 %d", total, len(payssionChannelSeeds()))
	}
	var one paymentdomain.PaymentChannel
	db.Where("channel_type = ?", "qris_id").First(&one)
	if one.ProviderType != constants.PaymentProviderPayssion {
		t.Fatalf("provider_type 异常: %s", one.ProviderType)
	}
}
