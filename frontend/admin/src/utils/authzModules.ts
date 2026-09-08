// 权限目录里的「模块」是后端按接口路径推导出来的（/admin/api-credentials/... -> api-credentials），
// 直接展示对运营不友好。这里给每个模块配人话名称，未收录的键回落原样显示，
// 新增模块不会因为漏配而变成空白。

type ModuleLabelMap = Record<string, string>

const ZH_CN: ModuleLabelMap = {
  '2fa': '两步验证',
  affiliates: '推广返利',
  'api-credentials': '开放接口凭证',
  authz: '权限管理',
  banners: '轮播图',
  'card-secrets': '卡密库存',
  categories: '商品分类',
  'channel-clients': '渠道客户端',
  compliance: '合规声明',
  coupons: '优惠券',
  dashboard: '仪表盘',
  'free-claim': '免费领取',
  fulfillments: '订单履约',
  'gift-cards': '礼品卡',
  media: '媒体素材',
  'member-level-prices': '会员等级价',
  'member-levels': '会员等级',
  'membership-plans': '会员套餐',
  'order-refunds': '订单退款',
  orders: '订单管理',
  password: '密码',
  'payment-channels': '支付渠道',
  payments: '支付记录',
  points: '积分',
  'post-categories': '文章分类',
  posts: '文章内容',
  'procurement-orders': '采购单',
  'product-mappings': '商品映射',
  products: '商品管理',
  promotions: '活动价',
  reconciliation: '对账',
  resellers: '分销商',
  settings: '系统设置',
  'site-connections': '站点互联',
  support: '客服工单',
  system: '系统信息',
  'telegram-bot': 'Telegram 机器人',
  'token-stations': 'Token 额度',
  upload: '文件上传',
  'upstream-categories': '上游分类',
  'upstream-products': '上游商品',
  'user-login-logs': '用户登录日志',
  users: '用户管理',
  wallet: '钱包',
}

const ZH_TW: ModuleLabelMap = {
  '2fa': '兩步驗證',
  affiliates: '推廣返利',
  'api-credentials': '開放介面憑證',
  authz: '權限管理',
  banners: '輪播圖',
  'card-secrets': '卡密庫存',
  categories: '商品分類',
  'channel-clients': '渠道客戶端',
  compliance: '合規聲明',
  coupons: '優惠券',
  dashboard: '儀表板',
  'free-claim': '免費領取',
  fulfillments: '訂單履約',
  'gift-cards': '禮品卡',
  media: '媒體素材',
  'member-level-prices': '會員等級價',
  'member-levels': '會員等級',
  'membership-plans': '會員套餐',
  'order-refunds': '訂單退款',
  orders: '訂單管理',
  password: '密碼',
  'payment-channels': '支付渠道',
  payments: '支付記錄',
  points: '積分',
  'post-categories': '文章分類',
  posts: '文章內容',
  'procurement-orders': '採購單',
  'product-mappings': '商品映射',
  products: '商品管理',
  promotions: '活動價',
  reconciliation: '對帳',
  resellers: '分銷商',
  settings: '系統設定',
  'site-connections': '站點互聯',
  support: '客服工單',
  system: '系統資訊',
  'telegram-bot': 'Telegram 機器人',
  'token-stations': 'Token 額度',
  upload: '檔案上傳',
  'upstream-categories': '上游分類',
  'upstream-products': '上游商品',
  'user-login-logs': '用戶登入日誌',
  users: '用戶管理',
  wallet: '錢包',
}

const EN_US: ModuleLabelMap = {
  '2fa': 'Two-Factor Auth',
  affiliates: 'Affiliate',
  'api-credentials': 'API Credentials',
  authz: 'Authorization',
  banners: 'Banners',
  'card-secrets': 'Card Secrets',
  categories: 'Categories',
  'channel-clients': 'Channel Clients',
  compliance: 'Compliance',
  coupons: 'Coupons',
  dashboard: 'Dashboard',
  'free-claim': 'Free Claim',
  fulfillments: 'Fulfillment',
  'gift-cards': 'Gift Cards',
  media: 'Media',
  'member-level-prices': 'Member Level Prices',
  'member-levels': 'Member Levels',
  'membership-plans': 'Membership Plans',
  'order-refunds': 'Order Refunds',
  orders: 'Orders',
  password: 'Password',
  'payment-channels': 'Payment Channels',
  payments: 'Payments',
  points: 'Points',
  'post-categories': 'Post Categories',
  posts: 'Posts',
  'procurement-orders': 'Procurement Orders',
  'product-mappings': 'Product Mappings',
  products: 'Products',
  promotions: 'Promotions',
  reconciliation: 'Reconciliation',
  resellers: 'Resellers',
  settings: 'Settings',
  'site-connections': 'Site Connections',
  support: 'Support',
  system: 'System',
  'telegram-bot': 'Telegram Bot',
  'token-stations': 'Token Stations',
  upload: 'Upload',
  'upstream-categories': 'Upstream Categories',
  'upstream-products': 'Upstream Products',
  'user-login-logs': 'User Login Logs',
  users: 'Users',
  wallet: 'Wallet',
}

const BY_LOCALE: Record<string, ModuleLabelMap> = {
  'zh-CN': ZH_CN,
  'zh-TW': ZH_TW,
  'en-US': EN_US,
}

/** 已收录的模块键，供测试断言覆盖面。 */
export const knownAuthzModuleKeys = Object.keys(ZH_CN)

/**
 * 取模块的展示名。未收录的键原样返回——后端新增接口时宁可显示英文原键，
 * 也不要显示空白让人以为权限丢了。
 */
export const authzModuleLabel = (module: string, locale: string): string => {
  const key = String(module || '').trim()
  if (!key) return ''
  const map = BY_LOCALE[locale] || ZH_CN
  return map[key] || key
}

/** HTTP 方法的中文说明，用于让「动作」列不只是 GET/POST。 */
const METHOD_HINT: Record<string, Record<string, string>> = {
  'zh-CN': { GET: '查看', POST: '新增/操作', PUT: '修改', PATCH: '修改', DELETE: '删除', '*': '全部' },
  'zh-TW': { GET: '查看', POST: '新增/操作', PUT: '修改', PATCH: '修改', DELETE: '刪除', '*': '全部' },
  'en-US': { GET: 'Read', POST: 'Create/Act', PUT: 'Update', PATCH: 'Update', DELETE: 'Delete', '*': 'All' },
}

export const authzMethodHint = (method: string, locale: string): string => {
  const key = String(method || '').trim().toUpperCase()
  const map = METHOD_HINT[locale] || METHOD_HINT['zh-CN']
  return (map && map[key]) || ''
}
