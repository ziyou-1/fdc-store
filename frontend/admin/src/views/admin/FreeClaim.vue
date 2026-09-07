<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  AdminFreeClaimConfig,
  AdminFreeClaimPoolItem,
  AdminMemberLevel,
  AdminProduct,
  AdminProductSKU,
} from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { formatDate, getLocalizedText } from '@/utils/format'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'

const { t } = useI18n()

const configLoading = ref(true)
const configSaving = ref(false)
const poolLoading = ref(true)
const { refreshing, refreshList } = useListRefresh()

const config = reactive<AdminFreeClaimConfig>({
  enabled: false,
  member_only: true,
  min_member_level_id: 0,
})
const memberLevels = ref<AdminMemberLevel[]>([])
const poolItems = ref<AdminFreeClaimPoolItem[]>([])

// 后端池条目只有 product_id / sku_id，表格要显示可读名称必须自己解析。
// 商品名来自商品列表；SKU 名只能从商品详情的 skus 数组取（无独立 SKU 列表接口）。
const products = ref<AdminProduct[]>([])
const skuCache = reactive<Record<number, AdminProductSKU[]>>({})

const showModal = ref(false)
const submitting = ref(false)
const formError = ref('')
const isEditing = ref(false)
const editingId = ref<number | null>(null)

const form = reactive({
  product_id: 0,
  sku_id: 0,
  is_active: true,
  sort_order: 0,
})

const resetForm = () => {
  form.product_id = 0
  form.sku_id = 0
  form.is_active = true
  form.sort_order = 0
  formError.value = ''
}

const formatSkuSpecValues = (values?: Record<string, string>) => {
  if (!values) return ''
  return Object.entries(values)
    .map(([key, value]) => `${key}: ${value}`)
    .join(' / ')
}

const buildSkuLabel = (sku: AdminProductSKU | null | undefined) => {
  const skuCode = String(sku?.sku_code || '').trim()
  const specText = formatSkuSpecValues(sku?.spec_values)
  if (skuCode && specText) return `${skuCode} · ${specText}`
  if (skuCode) return skuCode
  if (specText) return specText
  if (sku?.id) return `#${sku.id}`
  return '-'
}

const productLabel = (productId: number) => {
  const product = products.value.find((item) => Number(item.id) === Number(productId))
  const name = product ? getLocalizedText(product.title || {}) : ''
  return name || `#${productId}`
}

const skuLabel = (productId: number, skuId: number) => {
  const list = skuCache[productId]
  const sku = list?.find((item) => Number(item.id) === Number(skuId))
  return sku ? buildSkuLabel(sku) : `#${skuId}`
}

// 只加载池内出现过的商品的 SKU，避免为整个商品库拉详情
const ensureSkusLoaded = async (productIds: number[]) => {
  const missing = [...new Set(productIds)].filter((id) => id > 0 && !skuCache[id])
  if (missing.length === 0) return
  const results = await Promise.allSettled(missing.map((id) => adminAPI.getProduct(id)))
  results.forEach((result, index) => {
    const productId = missing[index]
    if (productId === undefined) return
    if (result.status === 'fulfilled') {
      const detail = result.value.data.data as AdminProduct | undefined
      skuCache[productId] = Array.isArray(detail?.skus) ? detail.skus : []
    } else {
      // 详情拉不到就退回显示 ID，不阻断整张表
      skuCache[productId] = []
    }
  })
}

const fetchConfig = async () => {
  configLoading.value = true
  try {
    const response = await adminAPI.getFreeClaimConfig()
    const data = response.data.data || {}
    config.enabled = Boolean(data.enabled)
    config.member_only = Boolean(data.member_only)
    config.min_member_level_id = Number(data.min_member_level_id) || 0
  } catch {
    // client 已统一提示，这里不重复弹
  } finally {
    configLoading.value = false
  }
}

const fetchMemberLevels = async () => {
  try {
    const response = await adminAPI.getMemberLevels({ page: 1, page_size: 100 })
    memberLevels.value = response.data.data || []
  } catch {
    memberLevels.value = []
  }
}

const fetchProducts = async () => {
  try {
    const response = await adminAPI.getProducts({ page: 1, page_size: 200 })
    products.value = response.data.data || []
  } catch {
    products.value = []
  }
}

const fetchPool = async (options: ListFetchOptions = {}) => {
  if (!options.preserveRows) poolLoading.value = true
  try {
    const response = await adminAPI.getFreeClaimPool()
    poolItems.value = response.data.data || []
    await ensureSkusLoaded(poolItems.value.map((item) => Number(item.product_id)))
  } catch {
    if (!options.preserveRows) poolItems.value = []
  } finally {
    if (!options.preserveRows) poolLoading.value = false
  }
}

const refresh = () => {
  refreshList(() => fetchPool({ preserveRows: true }))
}

const handleSaveConfig = async () => {
  configSaving.value = true
  try {
    await adminAPI.updateFreeClaimConfig({
      enabled: config.enabled,
      member_only: config.member_only,
      min_member_level_id: Number(config.min_member_level_id) || 0,
    })
    notifySuccess(t('admin.freeClaim.saveSuccess'))
  } catch {
    // client 已提示
  } finally {
    configSaving.value = false
  }
}

// 表单里选中的商品的可选 SKU
const formSkus = computed(() => skuCache[form.product_id] ?? [])

const handleProductChange = async (value: string | number) => {
  form.product_id = Number(value) || 0
  form.sku_id = 0
  if (form.product_id > 0) {
    await ensureSkusLoaded([form.product_id])
  }
}

const openCreateModal = () => {
  isEditing.value = false
  editingId.value = null
  resetForm()
  showModal.value = true
}

const openEditModal = async (item: AdminFreeClaimPoolItem) => {
  isEditing.value = true
  editingId.value = item.id
  formError.value = ''
  form.product_id = Number(item.product_id)
  form.sku_id = Number(item.sku_id)
  form.is_active = Boolean(item.is_active)
  form.sort_order = Number(item.sort_order) || 0
  showModal.value = true
  await ensureSkusLoaded([form.product_id])
}

const closeModal = () => {
  showModal.value = false
  isEditing.value = false
  editingId.value = null
}

const handleSubmit = async () => {
  formError.value = ''
  if (form.product_id <= 0) {
    formError.value = t('admin.freeClaim.form.productRequired')
    return
  }
  if (form.sku_id <= 0) {
    formError.value = t('admin.freeClaim.form.skuRequired')
    return
  }
  submitting.value = true
  try {
    const payload = {
      product_id: Number(form.product_id),
      sku_id: Number(form.sku_id),
      is_active: form.is_active,
      sort_order: Number(form.sort_order) || 0,
    }
    if (isEditing.value && editingId.value) {
      await adminAPI.updateFreeClaimPoolItem(editingId.value, payload)
    } else {
      await adminAPI.createFreeClaimPoolItem(payload)
    }
    closeModal()
    await fetchPool()
  } catch (err: any) {
    formError.value = err?.message || t('admin.freeClaim.saveFailed')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (item: AdminFreeClaimPoolItem) => {
  const confirmed = await confirmAction({
    description: t('admin.freeClaim.deleteConfirm', { name: productLabel(item.product_id) }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteFreeClaimPoolItem(item.id)
    await fetchPool()
  } catch (err: any) {
    notifyError(err?.message || t('admin.freeClaim.deleteFailed'))
  }
}

onMounted(async () => {
  await Promise.allSettled([fetchConfig(), fetchMemberLevels(), fetchProducts()])
  await fetchPool()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.freeClaim.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.freeClaim.subtitle') }}</p>
      </div>
      <Button size="sm" class="w-full gap-2 sm:w-auto" @click="openCreateModal">
        <svg class="h-4 w-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.freeClaim.addPoolItem') }}
      </Button>
    </div>

    <!-- 规则配置 -->
    <div class="rounded-xl border border-border bg-card p-6 shadow-sm">
      <h2 class="mb-4 text-base font-semibold">{{ t('admin.freeClaim.configTitle') }}</h2>

      <div v-if="configLoading" class="h-24 animate-pulse rounded-lg bg-muted"></div>
      <div v-else class="space-y-5">
        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-sm font-medium">{{ t('admin.freeClaim.form.enabled') }}</p>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.freeClaim.form.enabledHint') }}</p>
          </div>
          <Switch v-model="config.enabled" />
        </div>

        <div class="flex items-start justify-between gap-4">
          <div>
            <p class="text-sm font-medium">{{ t('admin.freeClaim.form.memberOnly') }}</p>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.freeClaim.form.memberOnlyHint') }}</p>
          </div>
          <Switch v-model="config.member_only" />
        </div>

        <div v-if="config.member_only">
          <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
            {{ t('admin.freeClaim.form.minMemberLevel') }}
          </label>
          <select
            v-model.number="config.min_member_level_id"
            class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring sm:max-w-xs"
          >
            <option :value="0">{{ t('admin.freeClaim.form.anyLevel') }}</option>
            <option v-for="level in memberLevels" :key="level.id" :value="level.id">
              {{ getLocalizedText(level.name) }}
            </option>
          </select>
          <p class="mt-1 text-[11px] leading-5 text-muted-foreground">
            {{ t('admin.freeClaim.form.minMemberLevelHint') }}
          </p>
        </div>

        <Button size="sm" :disabled="configSaving" @click="handleSaveConfig">
          {{ configSaving ? t('admin.common.submitting') : t('admin.common.save') }}
        </Button>
      </div>
    </div>

    <!-- 商品池 -->
    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-base font-semibold">{{ t('admin.freeClaim.poolTitle') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.freeClaim.poolHint') }}</p>
        </div>
        <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">
          {{ t('admin.common.refresh') }}
        </Button>
      </div>
    </div>

    <div class="overflow-x-auto rounded-xl border border-border bg-card">
      <Table class="min-w-[860px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="min-w-[80px] px-6 py-3">{{ t('admin.freeClaim.table.id') }}</TableHead>
            <TableHead class="min-w-[200px] px-6 py-3">{{ t('admin.freeClaim.table.product') }}</TableHead>
            <TableHead class="min-w-[180px] px-6 py-3">{{ t('admin.freeClaim.table.sku') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.freeClaim.table.sortOrder') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.freeClaim.table.isActive') }}</TableHead>
            <TableHead class="min-w-[120px] px-6 py-3">{{ t('admin.freeClaim.table.createdAt') }}</TableHead>
            <TableHead class="min-w-[120px] px-6 py-3 text-right">{{ t('admin.freeClaim.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="poolLoading">
            <TableCell :colspan="7" class="p-0">
              <TableSkeleton :columns="7" :rows="4" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="poolItems.length === 0">
            <TableCell :colspan="7" class="px-6 py-8 text-center text-muted-foreground">
              {{ t('admin.freeClaim.empty') }}
            </TableCell>
          </TableRow>
          <TableRow v-for="item in poolItems" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4"><IdCell :value="item.id" /></TableCell>
            <TableCell class="px-6 py-4 font-medium text-foreground">
              {{ productLabel(item.product_id) }}
            </TableCell>
            <TableCell class="px-6 py-4 font-mono text-xs text-muted-foreground">
              {{ skuLabel(item.product_id, item.sku_id) }}
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ item.sort_order }}</TableCell>
            <TableCell class="px-6 py-4">
              <span
                class="inline-flex rounded-full px-2 py-0.5 text-[11px]"
                :class="item.is_active
                  ? 'border border-emerald-200 bg-emerald-50 text-emerald-700'
                  : 'border border-border bg-muted text-muted-foreground'"
              >
                {{ item.is_active ? t('admin.common.enabled') : t('admin.common.disabled') }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell class="px-6 py-4 text-right">
              <div class="flex justify-end gap-2">
                <Button size="sm" variant="outline" @click="openEditModal(item)">
                  {{ t('admin.common.edit') }}
                </Button>
                <Button size="sm" variant="destructive" @click="handleDelete(item)">
                  {{ t('admin.common.delete') }}
                </Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <Dialog v-model:open="showModal" @update:open="(value: boolean) => { if (!value) closeModal() }">
      <DialogScrollContent
        class="w-[calc(100vw-1rem)] max-w-xl p-4 sm:p-6"
        @interact-outside="(e: Event) => e.preventDefault()"
      >
        <DialogHeader>
          <DialogTitle>
            {{ isEditing ? t('admin.freeClaim.form.editTitle') : t('admin.freeClaim.form.createTitle') }}
          </DialogTitle>
        </DialogHeader>
        <form class="space-y-4" @submit.prevent="handleSubmit">
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.freeClaim.form.product') }} *
            </label>
            <select
              :value="form.product_id"
              class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              @change="handleProductChange(($event.target as HTMLSelectElement).value)"
            >
              <option :value="0">{{ t('admin.freeClaim.form.productPlaceholder') }}</option>
              <option v-for="product in products" :key="product.id" :value="product.id">
                {{ getLocalizedText(product.title) }}
              </option>
            </select>
          </div>

          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.freeClaim.form.sku') }} *
            </label>
            <select
              v-model.number="form.sku_id"
              :disabled="form.product_id <= 0"
              class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50"
            >
              <option :value="0">
                {{ form.product_id > 0
                  ? t('admin.freeClaim.form.skuPlaceholder')
                  : t('admin.freeClaim.form.selectProductFirst') }}
              </option>
              <option v-for="sku in formSkus" :key="sku.id" :value="sku.id">
                {{ buildSkuLabel(sku) }}
              </option>
            </select>
            <p v-if="form.product_id > 0 && formSkus.length === 0" class="mt-1 text-[11px] text-muted-foreground">
              {{ t('admin.freeClaim.form.noSkus') }}
            </p>
          </div>

          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.freeClaim.form.sortOrder') }}
            </label>
            <Input v-model.number="form.sort_order" type="number" min="0" placeholder="0" />
          </div>

          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-medium">{{ t('admin.freeClaim.form.isActive') }}</p>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.freeClaim.form.isActiveHint') }}</p>
            </div>
            <Switch v-model="form.is_active" />
          </div>

          <p v-if="formError" class="text-sm text-destructive">{{ formError }}</p>

          <div class="flex justify-end gap-2 pt-2">
            <Button type="button" variant="outline" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" :disabled="submitting">
              {{ submitting ? t('admin.common.submitting') : t('admin.common.save') }}
            </Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
