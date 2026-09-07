import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { memberLevelAPI, membershipAPI, walletAPI } from '../api'
import type { MembershipPlan, MembershipPurchaseResult } from '../api/types'
import type { PageAlert } from '../utils/alerts'
import { isMembershipActive } from '../utils/membership'

export { isMembershipActive }

export function useMembership() {
  const { t, locale } = useI18n()
  const loading = ref(true)
  const purchasing = ref(false)
  const channelLoading = ref(false)
  const plans = ref<MembershipPlan[]>([])
  const channels = ref<any[]>([])
  const selectedPeriod = ref('')
  const channelId = ref(0)
  const alert = ref<PageAlert | null>(null)
  const qrResult = ref<MembershipPurchaseResult | null>(null)

  const selectedPlan = computed(() => plans.value.find((item) => item.period === selectedPeriod.value) || null)

  const localized = (value: Record<string, string> | undefined) => {
    if (!value) return ''
    return value[locale.value] || value['zh-CN'] || value['en-US'] || Object.values(value)[0] || ''
  }

  const loadPlans = async () => {
    loading.value = true
    try {
      const res = await memberLevelAPI.plans()
      plans.value = Array.isArray(res.data.data) ? res.data.data : []
      if (!selectedPeriod.value && plans.value[0]) {
        selectedPeriod.value = plans.value[0].period
      }
    } catch (err: any) {
      alert.value = { level: 'error', message: err?.message || t('personalCenter.membership.loadFailed') }
    } finally {
      loading.value = false
    }
  }

  const loadChannels = async () => {
    const amount = String(selectedPlan.value?.price || '').trim()
    if (!amount || Number(amount) <= 0) {
      channels.value = []
      return
    }
    channelLoading.value = true
    try {
      const res = await walletAPI.getPaymentChannels(amount)
      channels.value = res.data.data ?? []
      if (!channels.value.some((item: any) => item.id === channelId.value)) {
        channelId.value = Number(channels.value[0]?.id) || 0
      }
    } catch (err: any) {
      channels.value = []
      channelId.value = 0
      alert.value = { level: 'error', message: err?.message || t('personalCenter.membership.channelLoadFailed') }
    } finally {
      channelLoading.value = false
    }
  }

  const purchase = async () => {
    alert.value = null
    qrResult.value = null
    if (!selectedPeriod.value) return null
    if (channelId.value <= 0) {
      alert.value = { level: 'warning', message: t('personalCenter.membership.channelRequired') }
      return null
    }
    purchasing.value = true
    try {
      const res = await membershipAPI.purchase({
        period: selectedPeriod.value,
        channel_id: channelId.value,
      })
      const result = res.data.data || {}
      if (result.redirect_url) {
        window.location.href = result.redirect_url
        return null
      }
      if (result.qr_code) {
        qrResult.value = result
        return result
      }
      return result
    } catch (err: any) {
      alert.value = { level: 'error', message: err?.message || t('personalCenter.membership.purchaseFailed') }
      return null
    } finally {
      purchasing.value = false
    }
  }

  return {
    loading,
    purchasing,
    channelLoading,
    plans,
    channels,
    selectedPeriod,
    selectedPlan,
    channelId,
    alert,
    qrResult,
    localized,
    loadPlans,
    loadChannels,
    purchase,
  }
}
