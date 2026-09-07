import { computed, onMounted, ref, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Banknote, Coins, Crown, Home, ShoppingBag, Wallet, Gift, ShieldCheck, UserCircle, Megaphone, Key } from 'lucide-vue-next'
import { orderStatusLabel, orderStatusVariant } from '../utils/status'
import type { PageAlert } from '../utils/alerts'
import { useAppStore } from '../stores/app'
import { useUserProfileStore } from '../stores/userProfile'
import type { PublicMemberLevel } from '../api'
import { discountOffN, isMembershipActive } from '../utils/membership'
import { isDisabledPersonalSection } from '../utils/disabledFeatures'

export type PersonalSection = 'overview' | 'profile' | 'security' | 'orders' | 'wallet' | 'points' | 'membership' | 'giftCard' | 'affiliate' | 'reseller' | 'api'

export interface PersonalSectionItem {
  key: PersonalSection
  label: string
  icon: Component
}

/**
 * 个人中心壳逻辑（classic + vault 壳共用）：账户头部、侧栏导航、概览数据、初始化。
 */
export function usePersonalCenter(sectionGetter: () => PersonalSection) {
  const router = useRouter()
  const { t, locale } = useI18n()
  const appStore = useAppStore()
  const userProfileStore = useUserProfileStore()

  const sectionItems: PersonalSectionItem[] = [
    { key: 'overview', label: 'personalCenter.tabs.overview', icon: Home },
    { key: 'orders', label: 'personalCenter.tabs.orders', icon: ShoppingBag },
    { key: 'wallet', label: 'personalCenter.tabs.wallet', icon: Wallet },
    { key: 'points', label: 'personalCenter.tabs.points', icon: Coins },
    { key: 'membership', label: 'personalCenter.tabs.membership', icon: Crown },
    { key: 'affiliate', label: 'personalCenter.tabs.affiliate', icon: Megaphone },
    { key: 'reseller', label: 'personalCenter.tabs.reseller', icon: Banknote },
    { key: 'giftCard', label: 'personalCenter.tabs.giftCard', icon: Gift },
    { key: 'security', label: 'personalCenter.tabs.security', icon: ShieldCheck },
    { key: 'api', label: 'personalCenter.tabs.api', icon: Key },
    { key: 'profile', label: 'personalCenter.tabs.profile', icon: UserCircle },
  ]

  const sectionRouteMap: Record<PersonalSection, string> = {
    overview: '/me',
    profile: '/me/profile',
    security: '/me/security',
    orders: '/me/orders',
    wallet: '/me/wallet',
    points: '/me/points',
    membership: '/me/membership',
    affiliate: '/me/affiliate',
    reseller: '/me/reseller',
    giftCard: '/me/gift-cards',
    api: '/me/api',
  }

  const canAccessResellerConsole = computed(() => appStore.canAccessResellerConsole)
  const visibleSectionItems = computed(() => {
    return sectionItems.filter((item) => {
      if (isDisabledPersonalSection(item.key)) return false
      return item.key !== 'reseller' || canAccessResellerConsole.value
    })
  })
  const currentSection = computed<PersonalSection>(() => {
    const section = sectionGetter()
    if (isDisabledPersonalSection(section)) return 'overview'
    if (section === 'reseller' && !canAccessResellerConsole.value) {
      return 'overview'
    }
    return section
  })
  const globalAlert = ref<PageAlert | null>(null)

  const displayInitial = computed(() => {
    const name = userProfileStore.displayName || ''
    const normalized = name.trim()
    if (!normalized) return 'U'
    return normalized.slice(0, 1).toUpperCase()
  })

  const switchSection = (section: PersonalSection) => {
    router.push(sectionRouteMap[section])
  }

  const statusLabel = (status?: string) => orderStatusLabel(t, status)
  const statusVariant = (status?: string) => orderStatusVariant(status)

  // vault 模板：BadgeTone → vault pill 类名（classic 不使用）
  const statusPillClass = (status?: string) => {
    const map: Record<string, string> = {
      success: 'pill-done',
      warning: 'pill-low',
      danger: 'pill-sale',
      info: 'pill-stock',
      accent: 'pill-sale',
      neutral: 'pill-out',
    }
    return map[orderStatusVariant(status)] || 'pill-out'
  }

  const formatMoney = (amount?: string, currency?: string) => {
    if (!amount) return '-'
    if (!currency) return amount
    return `${amount} ${currency}`
  }

  const formatDate = (raw?: string) => {
    if (!raw) return '-'
    const date = new Date(raw)
    if (Number.isNaN(date.getTime())) return raw
    return date.toLocaleString()
  }

  const emailVerifiedLabel = computed(() => {
    if (userProfileStore.profile?.email_verified_at) {
      return t('personalCenter.overview.emailVerified')
    }
    return t('personalCenter.overview.emailUnverified')
  })

  const emailVerifiedVariant = computed<'success' | 'warning'>(() => {
    return userProfileStore.profile?.email_verified_at ? 'success' : 'warning'
  })

  const formatDiscountOff = (rate: number) => (
    t('personalCenter.memberLevel.discountOff', { n: discountOffN(rate, locale.value) })
  )

  const discountText = computed(() => {
    const profile = userProfileStore.profile
    if (isMembershipActive(profile)) {
      return formatDiscountOff(Number(profile?.membership_discount_rate))
    }
    const lvl = userProfileStore.currentLevel
    if (lvl && lvl.discount_rate < 100) {
      return formatDiscountOff(lvl.discount_rate)
    }
    return t('personalCenter.memberLevel.noDiscount')
  })

  const isImagePath = (icon: string | undefined | null) => icon?.startsWith('/uploads/') || icon?.startsWith('http')

  const levelName = (level: PublicMemberLevel | null | undefined) => {
    if (!level) return t('personalCenter.memberLevel.defaultLevel')
    const loc = locale.value as string
    return level.name[loc] || level.name['zh-CN'] || level.name['en'] || level.slug || t('personalCenter.memberLevel.defaultLevel')
  }

  const initialize = async () => {
    globalAlert.value = null
    const [profileOk] = await Promise.all([
      userProfileStore.loadProfile(),
      userProfileStore.loadRecentOrders(5),
      userProfileStore.loadMemberLevels(),
    ])
    if (!profileOk) {
      globalAlert.value = {
        level: 'error',
        message: userProfileStore.profileError || t('personalCenter.common.loadFailed'),
      }
    }
  }

  onMounted(() => {
    initialize()
  })

  return {
    userProfileStore,
    canAccessResellerConsole,
    visibleSectionItems,
    currentSection,
    globalAlert,
    displayInitial,
    switchSection,
    statusLabel,
    statusVariant,
    statusPillClass,
    formatMoney,
    formatDate,
    emailVerifiedLabel,
    emailVerifiedVariant,
    discountText,
    formatDiscountOff,
    isImagePath,
    levelName,
  }
}
