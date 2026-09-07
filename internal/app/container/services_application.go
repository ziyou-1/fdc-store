package container

import (
	"context"
	"github.com/dujiao-next/internal/constants"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"

	"github.com/dujiao-next/internal/logger"
	cardsecretapp "github.com/dujiao-next/internal/modules/cardsecret/application"
	cartapp "github.com/dujiao-next/internal/modules/cart/application"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	couponapp "github.com/dujiao-next/internal/modules/coupon/application"
	freeclaimapp "github.com/dujiao-next/internal/modules/freeclaim/application"
	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentqueue "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/queueadapter"
	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcardsettingscurrency "github.com/dujiao-next/internal/modules/giftcard/infrastructure/settingscurrency"
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderrefund "github.com/dujiao-next/internal/modules/order/application/refund"
	orderqueue "github.com/dujiao-next/internal/modules/order/infrastructure/queueadapter"
	orderriskapp "github.com/dujiao-next/internal/modules/orderrisk/application"
	orderrisklimiter "github.com/dujiao-next/internal/modules/orderrisk/infrastructure/redislimiter"
	pointsapp "github.com/dujiao-next/internal/modules/points/application"
	promotionapp "github.com/dujiao-next/internal/modules/promotion/application"
	sitemapapp "github.com/dujiao-next/internal/modules/sitemap/application"
	sitemapcontract "github.com/dujiao-next/internal/modules/sitemap/contract"
	sitemapcache "github.com/dujiao-next/internal/modules/sitemap/infrastructure/cacheadapter"
	sitemapcatalog "github.com/dujiao-next/internal/modules/sitemap/infrastructure/catalogreader"
	supportapp "github.com/dujiao-next/internal/modules/support/application"
	tokenstationapp "github.com/dujiao-next/internal/modules/tokenstation/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	giftcardredeemgormuow "github.com/dujiao-next/internal/workflows/giftcardredeem/infrastructure/gormuow"
)

// initApplicationServices 装配内容、购物车、订单、履约和营销用例。
func (c *Container) initApplicationServices() {
	postStore := gormstore.NewPostStore(gormdb.DB)
	postCategoryStore := gormstore.NewPostCategoryStore(gormdb.DB)
	c.ContentPostService = contentapp.NewPostService(
		postStore,
		postStore,
		postCategoryStore,
		contentapp.SystemClock{},
	)
	c.ContentPostCategoryService = contentapp.NewPostCategoryService(postCategoryStore)
	c.CategoryService = categoryapp.NewService(c.CategoryRepo)
	if err := c.CategoryService.EnsureDefaultCategories(); err != nil {
		logger.Errorw("provider_ensure_default_categories_failed", "error", err)
		panic(err)
	}
	sitemapService, err := sitemapapp.NewService(
		sitemapcatalog.New(c.ProductRepo, c.CategoryRepo),
		sitemapcontract.PublishedPostReaderFunc(func(ctx context.Context, limit int) ([]sitemapcontract.PublishedPost, error) {
			// 只收录有 /posts/:slug 详情页的类型。常见问题在 /faq 页内联展开、
			// 没有独立详情页，若一起收录会生成语义错误的 /posts/{faq-slug}。
			result := make([]sitemapcontract.PublishedPost, 0, limit)
			for _, postType := range []string{constants.PostTypeNotice} {
				posts, _, listErr := c.ContentPostService.ListPublic(ctx, contentapp.PublicPostQuery{
					Type:     postType,
					Page:     1,
					PageSize: limit,
				})
				if listErr != nil {
					return nil, listErr
				}
				for _, post := range posts {
					result = append(result, sitemapcontract.PublishedPost{
						Slug:        post.Slug,
						CreatedAt:   post.CreatedAt,
						PublishedAt: post.PublishedAt,
					})
				}
			}
			return result, nil
		}),
		sitemapcache.New(),
	)
	if err != nil {
		logger.Errorw("provider_init_sitemap_failed", "error", err)
		panic(err)
	}
	c.SitemapService = sitemapService
	c.CartService = cartapp.NewService(c.CartRepo, c.ProductRepo, c.ProductSKURepo, c.PromotionRepo, c.SettingService)
	c.WalletService = walletapp.NewService(walletapp.Options{
		Repository: c.WalletRepo, Transactions: c.WalletRepo,
	})
	c.PointsService = pointsapp.NewService(c.PointsRepo, c.PointsRepo)
	c.FreeClaimService = freeclaimapp.NewService(c.FreeClaimRepo)
	c.TokenStationService = tokenstationapp.NewService(c.TokenStationRepo)
	c.SupportService = supportapp.NewService(c.SupportRepo)
	c.OrderRefundService = orderrefund.New(
		c.OrderStore,
		c.UserStore,
		c.AffiliateService,
		c.SettingService,
		c.WalletService,
		c.PaymentStore,
	)
	c.MemberLevelService = memberlevelapp.NewService(c.MemberLevelRepo, c.MemberLevelPriceRepo, c.MemberLevelUserRepo)
	c.MemberLevelService.SetPlanRepository(memberlevelgormstore.NewPlanStore(gormdb.DB))
	if err := c.MemberLevelService.EnsureDefaultPlans(); err != nil {
		logger.Errorw("provider_ensure_default_membership_plans_failed", "error", err)
		panic(err)
	}
	c.OrderRiskControlService = orderriskapp.NewService(orderriskapp.Options{
		Settings:    c.SettingService,
		RateLimiter: orderrisklimiter.New(),
	})
	orderQueue := orderqueue.New(c.QueueClient)
	c.OrderService = orderapp.NewOrderService(orderapp.OrderServiceOptions{
		OrderStore:              c.OrderStore,
		UserStore:               c.UserStore,
		ProductStore:            c.ProductRepo,
		ProductSKUStore:         c.ProductSKURepo,
		CouponStore:             c.CouponRepo,
		CouponUsageStore:        c.CouponUsageRepo,
		PromotionRepo:           c.PromotionRepo,
		Queue:                   orderQueue,
		SettingService:          c.SettingService,
		DefaultEmailConfig:      c.Config.Email,
		WalletService:           c.WalletService,
		AffiliateService:        c.AffiliateService,
		MemberLevelService:      c.MemberLevelService,
		ResellerPricingResolver: c.ResellerPricingResolver,
		ResellerAccounting:      c.ResellerAccountingLedger,
		RiskControlService:      c.OrderRiskControlService,
		ExpireMinutes:           c.Config.Order.PaymentExpireMinutes,
	})
	c.FulfillmentService = fulfillmentapp.New(fulfillmentapp.Options{
		OrderStore:            c.OrderStore,
		FulfillmentStore:      c.FulfillmentStore,
		OrderQueue:            orderQueue,
		BotNotifier:           fulfillmentqueue.NewBotNotifier(c.QueueClient),
		SettingService:        c.SettingService,
		DefaultEmailConfig:    c.Config.Email,
		ExternalIdentityStore: c.ExternalIdentityStore,
	})
	c.CardSecretService = cardsecretapp.NewService(cardsecretapp.ServiceOptions{
		Secrets:      c.CardSecretRepo,
		Batches:      c.CardSecretBatchRepo,
		Transactions: c.CardSecretRepo,
		Products:     c.ProductRepo,
		ProductSKUs:  c.ProductSKURepo,
	})
	c.GiftCardService = giftcardapp.NewService(giftcardapp.Options{
		Repo:     c.GiftCardRepo,
		Users:    c.UserStore,
		Currency: giftcardsettingscurrency.New(c.SettingService),
		Redeemer: giftcardredeemgormuow.New(c.GiftCardRepo, c.WalletService),
	})
	c.CouponAdminService = couponapp.NewAdminService(c.CouponRepo)
	c.PromotionAdminService = promotionapp.NewAdminService(c.PromotionRepo)
	c.ContentBannerService = contentapp.NewBannerService(
		gormstore.NewBannerStore(gormdb.DB),
		contentapp.SystemClock{},
	)
}
