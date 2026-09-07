import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { userOrderAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import {
  buildManualFormSubmissions,
  emptyManualFormData,
  normalizeManualFormSchema,
  validateManualForm,
  type ManualFormField,
  type ManualFormProduct,
} from '../utils/manualForm'
import { useConfirmDialog } from './useConfirmDialog'
import { toast } from './useToast'
import { useOrderDisplayHelpers } from './useOrderDisplayHelpers'

/**
 * 已登录用户订单详情逻辑（classic + vault 共用）。
 */
export function useOrderDetail() {
  const route = useRoute()
  const router = useRouter()
  const { confirm: showConfirm } = useConfirmDialog()
  const { t } = useI18n()

  const loading = ref(true)
  const order = ref<any>(null)
  const fulfillmentDownloading = ref(false)
  const pendingManualForms = ref<ManualFormProduct[]>([])
  const pendingManualFormData = ref<Record<string, Record<string, any>>>({})
  const pendingManualSubmitAttempted = ref(false)
  const pendingManualSubmitting = ref(false)
  const pendingManualSlaHours = ref(24)

  const helpers = useOrderDisplayHelpers(order)

  const findOrderItemTitle = (productId: number, skuId: number) => {
    const match = (items: any[] | undefined) => {
      if (!Array.isArray(items)) return null
      return items.find((item) => Number(item?.product_id) === productId && Number(item?.sku_id || 0) === skuId) || null
    }
    return match(order.value?.items)?.title || match(order.value?.children?.flatMap((child: any) => child?.items || []))?.title || {}
  }

  const loadPendingManualForms = async (orderNo: string) => {
    const status = String(order.value?.status || '')
    if (!orderNo || status === 'pending_payment' || status === 'canceled' || status === 'cancelled') {
      pendingManualForms.value = []
      pendingManualFormData.value = {}
      return
    }
    try {
      const response = await userOrderAPI.getPendingManualForm(orderNo)
      const payload = response.data?.data || {}
      const items = Array.isArray(payload.items) ? payload.items : []
      pendingManualSlaHours.value = Number(payload.sla_hours) > 0 ? Number(payload.sla_hours) : 24
      pendingManualForms.value = items.map((item: any) => {
        const productId = Number(item?.product_id)
        const skuId = Number(item?.sku_id || 0)
        const fields = normalizeManualFormSchema(item?.schema)
        return {
          itemKey: String(item?.item_key || '').trim() || `${productId}:${skuId}`,
          productId,
          title: findOrderItemTitle(productId, skuId),
          fields,
          skuCount: 1,
        } as ManualFormProduct
      }).filter((item: ManualFormProduct) => item.fields.length > 0)
      pendingManualFormData.value = emptyManualFormData(pendingManualForms.value, pendingManualFormData.value)
    } catch {
      pendingManualForms.value = []
    }
  }

  const handleDownloadFulfillment = async (orderNo: string) => {
    if (fulfillmentDownloading.value) return
    fulfillmentDownloading.value = true
    try {
      const res = await userOrderAPI.downloadFulfillment(orderNo)
      const blob = new Blob([res.data], { type: 'text/plain; charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `fulfillment-${orderNo}.txt`
      a.click()
      URL.revokeObjectURL(url)
    } catch {} finally {
      fulfillmentDownloading.value = false
    }
  }

  const loadOrder = async () => {
    loading.value = true
    try {
      const response = await userOrderAPI.detail(String(route.params.order_no || '').trim())
      order.value = response.data.data
      await loadPendingManualForms(String(order.value?.order_no || ''))
    } catch (error) {
      order.value = null
      pendingManualForms.value = []
    } finally {
      loading.value = false
    }
  }

  const debouncedLoadOrder = debounceAsync(loadOrder, 300)

  const cancelOrder = async () => {
    if (!order.value) return
    const confirmed = await showConfirm({
      title: t('orderDetail.cancel'),
      message: t('orderDetail.cancelConfirm'),
      confirmText: t('common.confirm'),
      cancelText: t('common.cancel'),
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await userOrderAPI.cancel(order.value.order_no)
      await debouncedLoadOrder()
    } catch {
      toast.error(t('orderDetail.cancelFailed'))
    }
  }

  const getPendingManualFieldLabel = (field: ManualFormField) => {
    const localized = helpers.getLocalizedText(field.label)
    return localized || field.key
  }

  const getPendingManualFieldPlaceholder = (field: ManualFormField) => {
    return helpers.getLocalizedText(field.placeholder) || ''
  }

  const pendingManualFormValidation = computed(() => {
    return validateManualForm(pendingManualForms.value, pendingManualFormData.value, t, getPendingManualFieldLabel)
  })

  const pendingManualFieldError = (itemKey: string, fieldKey: string) => {
    return pendingManualFormValidation.value.errors[`${itemKey}:${fieldKey}`] || ''
  }

  const submitPendingManualForm = async () => {
    if (!order.value?.order_no || pendingManualSubmitting.value) return
    pendingManualSubmitAttempted.value = true
    if (!pendingManualFormValidation.value.valid) {
      toast.error(pendingManualFormValidation.value.firstError || t('orderDetail.manualFormSubmitFailed'))
      return
    }
    pendingManualSubmitting.value = true
    try {
      const response = await userOrderAPI.submitManualForm(order.value.order_no, {
        submissions: buildManualFormSubmissions(pendingManualForms.value, pendingManualFormData.value),
      })
      order.value = response.data?.data || order.value
      pendingManualSubmitAttempted.value = false
      await loadPendingManualForms(order.value.order_no)
      toast.success(t('orderDetail.manualFormSubmitted'))
    } catch {
      toast.error(t('orderDetail.manualFormSubmitFailed'))
    } finally {
      pendingManualSubmitting.value = false
    }
  }

  watch(pendingManualForms, (products) => {
    pendingManualFormData.value = emptyManualFormData(products, pendingManualFormData.value)
  })

  onMounted(() => {
    if (!route.params.order_no) {
      router.push('/me/orders')
      return
    }
    loadOrder()
  })

  onUnmounted(() => {
    debouncedLoadOrder.cancel()
  })

  return {
    loading,
    order,
    debouncedLoadOrder,
    cancelOrder,
    fulfillmentDownloading,
    handleDownloadFulfillment,
    pendingManualForms,
    pendingManualFormData,
    pendingManualSubmitAttempted,
    pendingManualSubmitting,
    pendingManualSlaHours,
    getPendingManualFieldLabel,
    getPendingManualFieldPlaceholder,
    pendingManualFieldError,
    submitPendingManualForm,
    ...helpers,
  }
}
