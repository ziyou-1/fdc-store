package container

import (
	"errors"
	"mime/multipart"
	"strconv"
	"time"

	"github.com/dujiao-next/internal/constants"
	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	freeclaimcontract "github.com/dujiao-next/internal/modules/freeclaim/contract"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	pointscontract "github.com/dujiao-next/internal/modules/points/contract"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	supportapp "github.com/dujiao-next/internal/modules/support/application"
	uploadapp "github.com/dujiao-next/internal/modules/upload/application"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"

	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
)

// wireServiceDependencies 收口构造后才能建立的双向或延迟依赖。
func (c *Container) wireServiceDependencies() {
	c.UserAuthService.SetMemberLevelService(c.MemberLevelService)
	c.OrderRefundService.SetResellerAccounting(c.ResellerAccountingLedger)
	c.PaymentService.SetMemberLevelService(c.MemberLevelService)
	c.PaymentService.SetProcurementService(c.ProcurementOrderService)
	c.PaymentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.PaymentService.SetContentFulfillService(contentFulfiller{fulfillments: c.FulfillmentService})
	c.FulfillmentService.SetDownstreamCallbackService(c.DownstreamCallbackService)
	c.PointsService.SetRechargeCreator(pointsRechargeCreator{payments: c.PaymentService, wallets: c.WalletService})
	c.PointsService.SetInviteIssuer(c.AffiliateService)
	c.MemberLevelService.SetPurchaseCreator(membershipPurchaseCreator{payments: c.PaymentService})
	c.SupportService.SetUserReader(supportUserReader{users: c.UserStore})
	if translator := supportapp.NewDeepLTranslator(c.Config.DeepL.AuthKey); translator != nil {
		c.SupportService.SetUserTextTranslator(translator.Translate)
	}
	c.UserAuthService.SetReferralServices(inviterResolver{affiliates: c.AffiliateRepo, users: c.UserStore}, c.PointsService)
	c.FreeClaimService.SetCollaborators(
		freeClaimOrderCreator{orders: c.OrderService, payments: c.PaymentService},
		memberLevelReader{users: c.UserStore},
	)
	c.FreeClaimService.SetProductReader(freeClaimProductReader{
		products: c.ProductRepo,
		skus:     c.ProductSKURepo,
	})
}

// freeClaimProductReader 把商品仓储适配成免费领取需要的"商品展示快照"端口。
// 池条目只存 ID，首页卡片需要名称、图片和 slug 才能渲染与跳转。
type freeClaimProductReader struct {
	products *productgormstore.ProductStore
	skus     *productgormstore.SKUStore
}

// ProductViewForSKU 取展示用商品快照。价格优先用 SKU 价，取不到再回落商品价。
// 商品不存在或已下架时返回 nil，让卡片自己决定如何降级。
func (a freeClaimProductReader) ProductViewForSKU(
	productID, skuID uint,
) (*freeclaimcontract.ProductView, error) {
	if a.products == nil || productID == 0 {
		return nil, nil
	}
	product, err := a.products.GetByID(strconv.FormatUint(uint64(productID), 10))
	if err != nil {
		return nil, err
	}
	if product == nil || !product.IsActive {
		return nil, nil
	}

	view := &freeclaimcontract.ProductView{
		ID:    product.ID,
		Slug:  product.Slug,
		Title: map[string]string{},
		Price: product.PriceAmount.String(),
	}
	for key, value := range product.TitleJSON {
		if text, ok := value.(string); ok {
			view.Title[key] = text
		}
	}
	if len(product.Images) > 0 {
		view.Image = product.Images[0]
	}
	if a.skus != nil && skuID > 0 {
		if sku, skuErr := a.skus.GetByID(skuID); skuErr == nil && sku != nil {
			view.Price = sku.PriceAmount.String()
		}
	}
	return view, nil
}

// contentFulfiller 把交付用例适配成支付模块声明的 ContentFulfiller 端口：
// 支付侧只关心是否成功，不需要交付记录本身，也就不必依赖交付模块的类型。
type contentFulfiller struct {
	fulfillments *fulfillmentapp.Service
}

type autoFulfiller struct {
	fulfillments *fulfillmentapp.Service
}

// CreateAuto 执行自动发卡。重复回调命中已存在的交付记录时视为成功，保持幂等。
func (a autoFulfiller) CreateAuto(orderID uint) error {
	if a.fulfillments == nil {
		return nil
	}
	_, err := a.fulfillments.CreateAuto(orderID)
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, fulfillmentapp.ErrFulfillmentExists),
		errors.Is(err, fulfillmentapp.ErrFulfillmentNotAuto),
		errors.Is(err, fulfillmentapp.ErrOrderStatusInvalid):
		return nil
	default:
		return err
	}
}

// CreateContent 执行内容交付。重复回调命中已存在的交付记录时视为成功，保持幂等。
func (a contentFulfiller) CreateContent(orderID uint) error {
	if a.fulfillments == nil {
		return nil
	}
	_, err := a.fulfillments.CreateContent(orderID)
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, fulfillmentapp.ErrFulfillmentExists),
		errors.Is(err, fulfillmentapp.ErrFulfillmentNotContent),
		errors.Is(err, fulfillmentapp.ErrOrderStatusInvalid):
		return nil
	default:
		return err
	}
}

// pointsRechargeCreator 把支付模块的充值建单能力适配成 points 声明的端口。
// 放在装配层而不是任一模块内：points/application 不能 import payment/application
// （后者已 import pointsapp），反向引用即编译环。
type membershipPurchaseCreator struct {
	payments *paymentapp.PaymentService
}

func (a membershipPurchaseCreator) CreateMembershipPurchase(
	input memberlevelcontract.PurchasePaymentInput,
) (*memberlevelcontract.PurchaseResult, error) {
	if a.payments == nil {
		return nil, memberlevelcontract.ErrPurchaseDisabled
	}
	result, err := a.payments.CreateWalletRechargePayment(paymentapp.CreateWalletRechargePaymentInput{
		UserID:                 input.UserID,
		ChannelID:              input.ChannelID,
		Amount:                 input.Amount,
		Currency:               input.Currency,
		Remark:                 input.Remark,
		ClientIP:               input.ClientIP,
		Context:                input.Context,
		RequestScheme:          input.RequestScheme,
		CreditTarget:           constants.RechargeCreditTargetMembership,
		MembershipPeriod:       input.Period,
		MembershipDurationDays: input.DurationDays,
		MembershipDiscountRate: input.DiscountRate,
	})
	if err != nil {
		return nil, err
	}
	if result == nil || result.Recharge == nil || result.Payment == nil {
		return nil, memberlevelcontract.ErrPurchaseDisabled
	}
	return &memberlevelcontract.PurchaseResult{
		RechargeNo:      result.Recharge.RechargeNo,
		PaymentID:       result.Payment.ID,
		PayableAmount:   result.Recharge.PayableAmount,
		Currency:        result.Recharge.Currency,
		Period:          result.Recharge.MembershipPeriod,
		RedirectURL:     result.Payment.PayURL,
		QRCode:          result.Payment.QRCode,
		InteractionMode: result.Payment.InteractionMode,
	}, nil
}

type pointsRechargeCreator struct {
	payments *paymentapp.PaymentService
	wallets  *walletapp.Service
}

// CreatePointsRecharge 复用钱包充值建单链路，只把到账目标切成积分。
// 渠道白名单、金额与币种校验、超时链路、回调幂等全部原样复用。
func (a pointsRechargeCreator) CreatePointsRecharge(
	input pointscontract.RechargePaymentInput,
) (*pointscontract.RechargeResult, error) {
	if a.payments == nil {
		return nil, pointscontract.ErrRechargeDisabled
	}
	result, err := a.payments.CreateWalletRechargePayment(paymentapp.CreateWalletRechargePaymentInput{
		UserID:        input.UserID,
		ChannelID:     input.ChannelID,
		Amount:        input.Amount,
		Currency:      input.Currency,
		Remark:        input.Remark,
		ClientIP:      input.ClientIP,
		Context:       input.Context,
		RequestScheme: input.RequestScheme,
		CreditTarget:  constants.RechargeCreditTargetPoints,
		PointsAmount:  input.PointsAmount,
		PointsPerUnit: input.PointsPerUnit,
	})
	if err != nil {
		// 渠道类错误翻译成 points 语义，避免前台看到"创建支付失败"这种无从下手的提示。
		switch {
		case errors.Is(err, paymentapp.ErrPaymentChannelNotFound),
			errors.Is(err, paymentapp.ErrPaymentChannelInactive),
			errors.Is(err, paymentapp.ErrPaymentChannelNotAllowedForRecharge),
			errors.Is(err, paymentapp.ErrPaymentChannelConfigInvalid):
			return nil, pointscontract.ErrRechargeChannelInvalid
		default:
			return nil, err
		}
	}
	if result == nil || result.Recharge == nil || result.Payment == nil {
		return nil, pointscontract.ErrRechargeNotFound
	}
	return &pointscontract.RechargeResult{
		RechargeNo:      result.Recharge.RechargeNo,
		PaymentID:       result.Payment.ID,
		PayableAmount:   result.Recharge.PayableAmount,
		Currency:        result.Recharge.Currency,
		PointsAmount:    result.Recharge.PointsAmount,
		RedirectURL:     result.Payment.PayURL,
		QRCode:          result.Payment.QRCode,
		InteractionMode: result.Payment.InteractionMode,
	}, nil
}

// ListPointsRecharges 复用钱包充值单查询，按 credit_target = points 过滤。
func (a pointsRechargeCreator) ListPointsRecharges(
	userID uint, page, pageSize int, status string,
) ([]pointscontract.RechargeRecord, int64, error) {
	if a.wallets == nil {
		return nil, 0, pointscontract.ErrRechargeDisabled
	}
	orders, total, err := a.wallets.ListRechargeOrdersAdmin(walletcontract.RechargeListFilter{
		Page:         page,
		PageSize:     pageSize,
		UserID:       userID,
		Status:       status,
		CreditTarget: constants.RechargeCreditTargetPoints,
	})
	if err != nil {
		return nil, 0, err
	}
	records := make([]pointscontract.RechargeRecord, 0, len(orders))
	for index := range orders {
		row := &orders[index]
		records = append(records, pointscontract.RechargeRecord{
			RechargeNo:    row.RechargeNo,
			Status:        row.Status,
			Amount:        row.Amount,
			PayableAmount: row.PayableAmount,
			Currency:      row.Currency,
			PointsAmount:  row.PointsAmount,
			PointsPerUnit: row.PointsPerUnit,
			PaidAt:        row.PaidAt,
			CreatedAt:     row.CreatedAt,
		})
	}
	return records, total, nil
}

// inviterResolver 把邀请码解析成推荐人用户 ID。
// 复用 affiliate_profiles.affiliate_code：用户手上只有一套码，
// 既能拿现金佣金也能带来积分奖励，不必再发明第二套邀请码。
type inviterResolver struct {
	affiliates affiliatecontract.Store
	users      usercontract.Store
}

func (r inviterResolver) ResolveInviterUserID(code string) (uint, error) {
	if r.affiliates == nil {
		return 0, nil
	}
	profile, err := r.affiliates.GetProfileByCode(affiliatedomain.NormalizeCode(code))
	if err != nil || profile == nil {
		return 0, err
	}
	// 停用的推广档案不再带来新的推荐关系。
	if profile.Status != constants.AffiliateProfileStatusActive {
		return 0, nil
	}
	// 用户状态与推广档案状态是两张表，后台封禁用户不会联动改档案。
	// 只看档案状态会导致"封了号还在收推荐奖励"，这里必须再查一次用户。
	if r.users != nil {
		inviter, err := r.users.GetByID(profile.UserID)
		if err != nil || inviter == nil {
			return 0, err
		}
		if inviter.Status != constants.UserStatusActive {
			return 0, nil
		}
	}
	return profile.UserID, nil
}

// freeClaimOrderCreator 把"下单 + 以免费领取 tender 结算"这一对动作适配成
// freeclaim 模块声明的端口。freeclaim 不能 import order/payment 的 application 包，
// 否则与订单事务里引用 freeclaim 仓储的方向构成编译环。
type freeClaimOrderCreator struct {
	orders   *orderapp.OrderService
	payments *paymentapp.PaymentService
}

// CreateFreeClaimOrder 按名义价正常下单，再用免费领取 tender 整单结算。
// 走完整下单链路而不是直接造 paid 订单，是为了白拿既有的交付、通知与查单能力。
func (a freeClaimOrderCreator) CreateFreeClaimOrder(
	userID, productID, skuID uint, claimDate string,
) (string, uint, error) {
	if a.orders == nil || a.payments == nil {
		return "", 0, freeclaimcontract.ErrDisabled
	}
	order, err := a.orders.CreateOrder(orderapp.CreateOrderInput{
		UserID: userID,
		Tenant: resellercontract.MainTenantContext(""),
		Items: []orderapp.CreateOrderItem{{
			ProductID: productID,
			SKUID:     skuID,
			Quantity:  1,
		}},
	})
	if err != nil {
		return "", 0, err
	}
	if order == nil {
		return "", 0, freeclaimcontract.ErrClaimCreateFailed
	}
	if _, payErr := a.payments.CreatePayment(paymentapp.CreatePaymentInput{
		OrderID: order.ID,
		UseFreeClaim: &orderapp.FreeClaimIntent{
			ClaimDate: claimDate,
			ProductID: productID,
			SKUID:     skuID,
		},
	}); payErr != nil {
		// 领取记录与订单结算在同一事务里，失败时不会留下"已领"标记，
		// 只会留下一张未支付订单，由既有的超时取消回收。
		return "", 0, payErr
	}
	return order.OrderNo, order.ID, nil
}

// memberLevelReader 读取付费会员资格与等级，供免费领取判定。
type memberLevelReader struct {
	users usercontract.Store
}

func (r memberLevelReader) CurrentMemberLevelID(userID uint) (uint, error) {
	if r.users == nil || userID == 0 {
		return 0, nil
	}
	user, err := r.users.GetByID(userID)
	if err != nil || user == nil {
		return 0, err
	}
	return user.MemberLevelID, nil
}

func (r memberLevelReader) HasPaidMembership(userID uint) (bool, error) {
	if r.users == nil || userID == 0 {
		return false, nil
	}
	user, err := r.users.GetByID(userID)
	if err != nil || user == nil {
		return false, err
	}
	return memberleveldomain.MembershipActive(user, time.Now()), nil
}

// supportUserReader 把用户仓储适配成客服工作台需要的"批量取邮箱"端口。
// support 模块不直接依赖 identity/user，避免为了一列展示字段把两个模块绑死。
type supportUserReader struct {
	users usercontract.Store
}

// GetEmailsByIDs 批量取邮箱；取不到的 ID 直接缺省，不阻断会话列表。
func (a supportUserReader) GetEmailsByIDs(ids []uint) (map[uint]string, error) {
	result := map[uint]string{}
	if a.users == nil || len(ids) == 0 {
		return result, nil
	}
	users, err := a.users.ListByIDs(ids)
	if err != nil {
		return result, err
	}
	for index := range users {
		result[users[index].ID] = users[index].Email
	}
	return result, nil
}

// supportAttachmentUploader 把独立策略的上传服务适配成客服附件端口。
// 用的是 SupportUploadService 而不是后台的 UploadService：
// 客服场景要放开视频与更大体积，绝不能顺手把后台素材库的白名单一起放开。
type SupportAttachmentUploader struct {
	uploads *uploadapp.Service
}

// NewSupportAttachmentUploader 供路由装配构造客服附件上传端口。
func NewSupportAttachmentUploader(uploads *uploadapp.Service) SupportAttachmentUploader {
	return SupportAttachmentUploader{uploads: uploads}
}

// SaveSupportAttachment 保存买家发来的图片或视频，落在 support 场景目录下。
func (a SupportAttachmentUploader) SaveSupportAttachment(
	file *multipart.FileHeader,
) (string, string, error) {
	if a.uploads == nil {
		return "", "", errors.New("support attachment uploader is not configured")
	}
	result, err := a.uploads.SaveFileWithMeta(file, "support")
	if err != nil {
		return "", "", err
	}
	return result.URL, result.MimeType, nil
}
