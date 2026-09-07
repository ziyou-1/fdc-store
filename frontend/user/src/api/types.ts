export interface UserProfileData {
    id: number
    email: string
    nickname: string
    email_verified_at?: string | null
    locale: string
    member_level_id?: number
    membership_period?: string
    membership_discount_rate?: number | string
    membership_expires_at?: string | null
    total_recharged?: number | string
    total_spent?: number | string
    email_change_mode?: 'bind_only' | 'change_with_old_and_new'
    password_change_mode?: 'set_without_old' | 'change_with_old'
}

export interface PublicMemberLevel {
    id: number
    name: Record<string, string>
    slug: string
    icon: string
    discount_rate: number
    recharge_threshold: number
    spend_threshold: number
    is_default: boolean
    sort_order: number
}

export interface MembershipPlan {
    period: 'month' | 'quarter' | 'year' | string
    price: number | string
    discount_rate: number
    duration_days: number
    name: Record<string, string>
    description: Record<string, string>
    is_active: boolean
}

export interface MembershipPurchasePayload {
    period: string
    channel_id: number
    currency?: string
    remark?: string
}

export interface MembershipPurchaseResult {
    recharge_no: string
    payment_id: number
    payable_amount: number | string
    currency: string
    period: string
    redirect_url?: string
    qr_code?: string
    interaction_mode?: string
}

export interface UpdateUserProfilePayload {
    nickname?: string
    locale?: string
}

export interface UserLoginLogItem {
    id: number
    user_id: number
    email: string
    status: string
    fail_reason?: string
    client_ip?: string
    user_agent?: string
    login_source?: string
    created_at?: string
}

export interface SendChangeEmailCodePayload {
    kind: 'old' | 'new'
    new_email?: string
}

export interface ChangeEmailPayload {
    new_email: string
    old_code?: string
    new_code: string
}

export interface ChangeUserPasswordPayload {
    old_password?: string
    new_password: string
}

export interface TelegramAuthPayload {
    id: number
    first_name?: string
    last_name?: string
    username?: string
    photo_url?: string
    auth_date: number
    hash: string
    invite_code?: string
}

export interface TelegramMiniAppAuthPayload {
    init_data: string
    invite_code?: string
}

export interface GoogleCredentialPayload {
    credential: string
    invite_code?: string
}

export interface TelegramBindingData {
    bound: boolean
    provider?: string
    provider_user_id?: string
    username?: string
    avatar_url?: string
    auth_at?: string | null
    updated_at?: string | null
    can_unbind?: boolean
}

export interface GoogleBindingData extends TelegramBindingData {
    email?: string
    display_name?: string
}

export interface WalletAccountData {
    balance: string
}

export interface WalletTransactionData {
    id: number
    type: string
    direction: string
    amount: string
    balance_after: string
    remark: string
    created_at: string
}

export interface WalletRechargePayload {
    amount: string
    channel_id: number
    currency?: string
    remark?: string
}

export interface WalletRechargeOrderData {
    id: number
    recharge_no: string
    amount: string
    payable_amount: string
    fee_amount: string
    currency: string
    status: string
    remark: string
    paid_at?: string
    created_at: string
}

export interface WalletRechargeResult {
    recharge?: WalletRechargeOrderData
    recharge_no?: string
    recharge_status?: string
    account?: WalletAccountData
    payment_id?: number
    provider_type?: string
    channel_type?: string
    interaction_mode?: string
    pay_url?: string
    qr_code?: string
    wallet_address?: string
    chain_amount?: string
    chain?: string
    token_id?: string
    expires_at?: string
    status?: string
}

export interface GiftCardData {
    id: number
    name: string
    code: string
    amount: string
    currency: string
    status: string
    redeemed_at?: string
}

export interface GiftCardRedeemResult {
    gift_card: GiftCardData
    wallet: WalletAccountData
    transaction: WalletTransactionData
    wallet_delta: string
}

export interface AffiliateDashboardData {
    opened: boolean
    affiliate_code: string
    promotion_path: string
    click_count: number
    valid_order_count: number
    conversion_rate: number
    pending_commission: string
    available_commission: string
    withdrawn_commission: string
}

export interface AffiliateCommissionData {
    id: number
    commission_type: string
    commission_amount: string
    status: string
    confirm_at?: string
    available_at?: string
    created_at: string
}

export interface AffiliateWithdrawData {
    id: number
    amount: string
    channel: string
    account: string
    status: string
    reject_reason?: string
    created_at: string
}

export interface AffiliateWithdrawApplyPayload {
    amount: string
    channel: string
    account: string
}

export interface ResellerProfileSummaryData {
    id: number
    status: string
    settlement_status: string
    created_at: string
}

export interface ResellerManagementProfileData {
    id: number
    status: string
    apply_reason?: string
    reject_reason?: string
    default_markup_percent: string
    max_markup_percent: string
    settlement_status: string
    reviewed_at?: string
    created_at: string
    updated_at: string
}

export interface ResellerDomainData {
    id: number
    domain: string
    type: string
    verification_token?: string
    verification_status: string
    status: string
    is_primary: boolean
    verified_at?: string
    created_at: string
    updated_at: string
}

export interface ResellerManagementSnapshotData {
    opened: boolean
    can_apply: boolean
    profile?: ResellerManagementProfileData
    domains: ResellerDomainData[]
}

export interface ResellerApplyPayload {
    reason?: string
}

export interface ResellerCustomDomainPayload {
    domain: string
}

export interface ResellerLocalizedText {
    'zh-CN': string
    'zh-TW': string
    'en-US': string
    'id-ID': string
}

export interface ResellerSiteConfigPayload {
    site_name: string
    logo?: string
    favicon?: string
    announcement?: {
        enabled: boolean
        type: string
        title: ResellerLocalizedText
        content: ResellerLocalizedText
    }
    support?: {
        telegram?: string
        whatsapp?: string
        email?: string
        support_url?: string
    }
    seo?: {
        title: ResellerLocalizedText
        keywords: ResellerLocalizedText
        description: ResellerLocalizedText
        default_og_image?: string
    }
    footer_links?: Array<{ name: ResellerLocalizedText; url: string }>
    nav_config?: {
        builtin: Record<string, boolean>
        custom_items: Array<{ name: ResellerLocalizedText; url: string }>
    }
}

export interface ResellerSiteConfigData extends ResellerSiteConfigPayload {
    id: number
    updated_at: string
}

export interface ResellerSiteConfigSnapshotData {
    opened: boolean
    can_edit: boolean
    config?: ResellerSiteConfigData
}

export type ResellerPricingMode = 'inherit' | 'markup_percent' | 'fixed_markup' | 'fixed_price'

export interface ResellerProductSettingData {
    id?: number
    product_id: number
    sku_id: number
    is_listed: boolean
    pricing_mode: ResellerPricingMode | string
    markup_percent: string
    fixed_markup_amount: string
    fixed_price_amount: string
    effective_price_amount?: string
    rule_source?: string
    sort_order: number
    updated_at?: string
}

export interface ResellerProductSettingSKUData {
    id: number
    sku_code: string
    spec_values: Record<string, unknown>
    base_price_amount: string
    is_active: boolean
    setting?: ResellerProductSettingData
    effective_price_amount?: string
}

export interface ResellerProductSettingProductData {
    id: number
    slug: string
    title: Record<string, string>
    price_amount: string
    is_active: boolean
}

export interface ResellerProductSettingDetailData {
    product: ResellerProductSettingProductData
    product_setting?: ResellerProductSettingData
    skus: ResellerProductSettingSKUData[]
}

export interface ResellerProductSettingPayloadItem {
    sku_id: number
    is_listed: boolean
    pricing_mode: ResellerPricingMode | string
    markup_percent: string
    fixed_markup_amount: string
    fixed_price_amount: string
    sort_order: number
}

export interface ResellerProductSettingUpdatePayload {
    settings: ResellerProductSettingPayloadItem[]
}

export interface ResellerProductSettingPreviewItem {
    sku_id: number
    is_listed: boolean
    base_price_amount: string
    effective_price_amount: string
    valid: boolean
    error_code?: string
}

export interface ResellerProductSettingPreviewData {
    items: ResellerProductSettingPreviewItem[]
}

export interface ResellerBalanceData {
    id: number
    currency: string
    status: string
    available_amount: string
    locked_amount: string
    negative_amount: string
    updated_at: string
}

export interface ResellerLedgerData {
    id: number
    order_id?: number
    type: string
    amount: string
    currency: string
    status: string
    available_at?: string
    withdraw_request_id?: number
    created_at: string
}

export interface ResellerWithdrawData {
    id: number
    amount: string
    currency: string
    channel: string
    account: string
    status: string
    reject_reason?: string
    processed_at?: string
    created_at: string
}

export interface ResellerDashboardData {
    opened: boolean
    profile?: ResellerProfileSummaryData
    balances?: ResellerBalanceData[]
    withdraw_enabled: boolean
    withdraw_disabled_reason?: string
}

export interface ResellerOrderListParams {
    page?: number
    page_size?: number
    status?: string
    order_no?: string
    created_from?: string
    created_to?: string
    paid_from?: string
    paid_to?: string
}

export type ResellerOrderStatsParams = ResellerOrderListParams

export interface ResellerOrderData {
    order_no: string
    status: string
    currency: string
    total_amount: string
    base_amount: string
    profit_amount: string
    profit_status: 'credited' | 'pending' | 'unavailable' | string
    domain: string
    buyer_label: string
    items_count: number
    created_at: string
    paid_at?: string | null
}

export interface ResellerOrderItemData {
    title: Record<string, string>
    sku_snapshot: Record<string, unknown>
    quantity: number
    unit_price: string
    total_price: string
    base_unit_amount?: string
    reseller_unit_amount?: string
    base_total_amount?: string
    reseller_total_amount?: string
    profit_amount?: string
}

export interface ResellerOrderDetailData extends ResellerOrderData {
    items: ResellerOrderItemData[]
}

export interface ResellerOrderStatsData {
    total: number
    by_status: Record<string, number>
    by_currency: Record<string, number>
}

export interface ResellerWithdrawApplyPayload {
    amount: string
    currency: string
    channel: string
    account: string
}

export interface CreatePaymentPayload {
    order_no: string
    channel_id?: number
    use_balance?: boolean
    use_points?: boolean
}

export interface PaymentCreateResult {
    order_paid?: boolean
    wallet_paid_amount?: string
    online_pay_amount?: string
    payable_amount?: string
    // currency 是 payable_amount 的实际币种；换汇渠道下与订单币种不同，
    // 展示 payable_amount 必须搭配这个字段，不能直接套用订单币种。
    currency?: string
    fee_amount?: string
    fee_policy?: string
    payment_id?: number
    order_no?: string
    channel_id?: number
    provider_type?: string
    channel_type?: string
    interaction_mode?: string
    pay_url?: string
    qr_code?: string
    wallet_address?: string
    chain_amount?: string
    chain?: string
    token_id?: string
    expires_at?: string
}

export interface CaptchaPayload {
    captcha_id?: string
    captcha_code?: string
    turnstile_token?: string
}

// ===== 在线客服 =====

// SupportConversationData 每个买家恒定一条会话。unread_for_user 是买家侧未读，
// unread_for_agent 是客服侧未读，两者互不影响。
export interface SupportConversationData {
    id: number
    user_id: number
    status: 'open' | 'closed'
    last_message_at: string | null
    last_message_preview: string
    unread_for_agent: number
    unread_for_user: number
    created_at: string
    updated_at: string
}

export type SupportContentType = 'text' | 'image' | 'video'

export interface SupportMessageData {
    id: number
    conversation_id: number
    // sender_type 为 'user'（买家）或 'agent'（客服），决定气泡左右。
    sender_type: 'user' | 'agent'
    sender_id: number
    content_type: SupportContentType
    content: string
    media_url: string
    created_at: string
}

export interface SupportQuickReplyData {
    id: number
    title: string
    content: string
    is_active: boolean
    sort_order: number
    created_at: string
    updated_at: string
}

// SupportSendMessagePayload content_type 省略时后端按 text 处理；
// image/video 必须带 media_url，text 必须带非空 content。
export interface SupportSendMessagePayload {
    content_type?: SupportContentType
    content?: string
    media_url?: string
}

export interface SupportAttachmentResult {
    media_url: string
    mime_type: string
    content_type: 'image' | 'video'
}

export interface SupportMessageListResult {
    conversation: SupportConversationData
    messages: SupportMessageData[]
    total: number
}

// ===== 积分 =====

// PointsAccountData 积分余额与累计。积分是整数（后端 int64），
// 与金额一律用字符串的约定不同，不要套用 money 工具函数。
export interface PointsAccountData {
    balance: number
    total_earned: number
    total_spent: number
}

export type PointsTransactionType =
    | 'checkin'
    | 'referral_register'
    | 'referral_order'
    | 'recharge'
    | 'exchange'
    | 'exchange_refund'
    | 'admin_adjust'

export interface PointsTransactionData {
    id: number
    type: PointsTransactionType | string
    direction: 'in' | 'out'
    amount: number
    balance_after: number
    remark: string
    created_at: string
}

// PointsCheckinStatusData enabled 为 false 时表示站点未开启签到，整块 UI 应隐藏。
export interface PointsCheckinStatusData {
    checked_in: boolean
    date: string
    streak_days: number
    checkin_points: number
    enabled: boolean
    balance: number
}

export interface PointsCheckinResult {
    points: number
    streak_days: number
    balance: number
    date: string
}

export interface PointsRechargePayload {
    channel_id: number
    amount: string
    currency?: string
    remark?: string
}

// PointsRechargeResult interaction_mode 只会是 qr 或 redirect
// （payment_service_provider.go 限定），前台据此决定跳转还是渲染二维码。
export interface PointsRechargeResult {
    recharge_no: string
    payment_id: number
    payable_amount: string
    currency: string
    points_amount: number
    redirect_url: string
    qr_code: string
    interaction_mode: string
}

export interface PointsRechargeRecordData {
    recharge_no: string
    status: string
    amount: string
    payable_amount: string
    currency: string
    points_amount: number
    points_per_unit: number
    paid_at: string | null
    created_at: string
}

// PointsPublicConfigData 买家可见的积分规则子集（GET /points/config）。
// 充值前必须靠 recharge_points_per_unit 告诉买家能换多少积分。
export interface PointsPublicConfigData {
    checkin_enabled: boolean
    checkin_points: number
    recharge_enabled: boolean
    recharge_points_per_unit: number
    exchange_enabled: boolean
    referral_enabled: boolean
    referral_register_points: number
    referral_first_order_points: number
}

export interface PointsInviteData {
    enabled: boolean
    code: string
    register_points: number
    first_order_points: number
}

// ===== 每日免费领取 =====

// FreeClaimProductData 后端补齐的商品展示快照。
// 池条目本身只有 ID，卡片靠这个字段渲染名称/图片并跳转到商品页。
export interface FreeClaimProductData {
    id: number
    slug: string
    title: Record<string, string>
    image: string
    price: string
}

export interface FreeClaimPoolItemData {
    id: number
    product_id: number
    sku_id: number
    is_active: boolean
    sort_order: number
}

// FreeClaimTodayData claimed 依赖可选鉴权：带 token 才有意义，游客恒为 false。
export interface FreeClaimTodayData {
    date: string
    enabled: boolean
    available: boolean
    claimed: boolean
    member_only: boolean
    eligible: boolean
    item?: FreeClaimPoolItemData
    product?: FreeClaimProductData
}

export interface FreeClaimResult {
    date: string
    order_no: string
    product_id: number
    sku_id: number
}
