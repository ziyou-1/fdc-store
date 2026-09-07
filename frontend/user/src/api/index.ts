// Re-export client instances and ApiResponse
export { api, userApi } from './client'
export type { ApiResponse } from './client'

// Re-export all types
export type {
    UserProfileData,
    PublicMemberLevel,
    MembershipPlan,
    MembershipPurchasePayload,
    MembershipPurchaseResult,
    UpdateUserProfilePayload,
    UserLoginLogItem,
    SendChangeEmailCodePayload,
    ChangeEmailPayload,
    ChangeUserPasswordPayload,
    TelegramAuthPayload,
    TelegramMiniAppAuthPayload,
    TelegramBindingData,
    GoogleCredentialPayload,
    GoogleBindingData,
    WalletAccountData,
    WalletTransactionData,
    WalletRechargePayload,
    WalletRechargeOrderData,
    WalletRechargeResult,
    GiftCardRedeemResult,
    AffiliateDashboardData,
    AffiliateCommissionData,
    AffiliateWithdrawData,
    AffiliateWithdrawApplyPayload,
    ResellerProfileSummaryData,
    ResellerManagementProfileData,
    ResellerDomainData,
    ResellerManagementSnapshotData,
    ResellerApplyPayload,
    ResellerCustomDomainPayload,
    ResellerLocalizedText,
    ResellerSiteConfigPayload,
    ResellerSiteConfigData,
    ResellerSiteConfigSnapshotData,
    ResellerBalanceData,
    ResellerLedgerData,
    ResellerWithdrawData,
    ResellerDashboardData,
    ResellerOrderListParams,
    ResellerOrderStatsParams,
    ResellerOrderData,
    ResellerOrderItemData,
    ResellerOrderDetailData,
    ResellerOrderStatsData,
    ResellerWithdrawApplyPayload,
    CreatePaymentPayload,
    PaymentCreateResult,
    CaptchaPayload,
    SupportConversationData,
    SupportContentType,
    SupportMessageData,
    SupportQuickReplyData,
    SupportSendMessagePayload,
    SupportAttachmentResult,
    SupportMessageListResult,
    PointsAccountData,
    PointsTransactionType,
    PointsTransactionData,
    PointsCheckinStatusData,
    PointsCheckinResult,
    PointsRechargePayload,
    PointsRechargeResult,
    PointsRechargeRecordData,
    PointsPublicConfigData,
    FreeClaimProductData,
    FreeClaimPoolItemData,
    FreeClaimTodayData,
    FreeClaimResult,
} from './types'

// Re-export domain APIs
export { productAPI, postAPI, bannerAPI, categoryAPI, memberLevelAPI } from './product'
export { userAuthAPI, captchaAPI, configAPI } from './auth'
export { userProfileAPI } from './user'
export { userOrderAPI, paymentAPI } from './order'
export { walletAPI, giftCardAPI } from './wallet'
export { affiliateAPI } from './affiliate'
export { resellerAPI } from './reseller'
export { apiCredentialAPI } from './credential'
export { supportAPI } from './support'
export { pointsAPI } from './points'
export { membershipAPI } from './membership'
export { freeClaimAPI } from './freeclaim'
export { tokenStationAPI } from './tokenstation'

// Default export for backward compatibility
export { default } from './client'
