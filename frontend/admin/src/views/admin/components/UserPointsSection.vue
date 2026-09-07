<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminPointsAccount, AdminPointsTransaction } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import ListPagination from '@/components/ListPagination.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  userId: number
  canAdjust: boolean
}>()

const { t } = useI18n()

const account = ref<AdminPointsAccount | null>(null)
const transactions = ref<AdminPointsTransaction[]>([])
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const success = ref('')
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const pageSizeOptions = [10, 20, 50, 100]
const form = reactive({
  operation: 'add',
  amount: '',
  remark: '',
})

const asInt = (value: unknown) => {
  const n = Math.trunc(Number(value))
  return Number.isFinite(n) ? n : 0
}

const fetchAccount = async () => {
  const response = await adminAPI.getUserPoints(props.userId)
  account.value = response.data.data || { balance: 0, total_earned: 0, total_spent: 0 }
}

const fetchTransactions = async (page = 1) => {
  loading.value = true
  try {
    const response = await adminAPI.getUserPointsTransactions(props.userId, {
      page,
      page_size: pagination.value.page_size,
    })
    transactions.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } finally {
    loading.value = false
  }
}

const load = async (page = pagination.value.page) => {
  if (!Number.isFinite(props.userId) || props.userId <= 0) return
  error.value = ''
  try {
    await Promise.all([fetchAccount(), fetchTransactions(page)])
  } catch (err: any) {
    error.value = err?.message || t('admin.userDetail.points.errors.loadFailed')
  }
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  void fetchTransactions(page)
}

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  void fetchTransactions(1)
}

const directionClass = (direction?: string) => {
  if (direction === 'in') return 'theme-badge-success'
  if (direction === 'out') return 'theme-badge-danger'
  return 'theme-badge-warning'
}

const directionLabel = (direction?: string) => {
  if (direction === 'in') return t('admin.userDetail.points.directionIn')
  if (direction === 'out') return t('admin.userDetail.points.directionOut')
  return direction || '-'
}

const typeLabel = (type?: string) => {
  const key = `admin.userDetail.points.types.${type || ''}`
  const translated = t(key)
  if (translated === key) return type || '-'
  return translated
}

const submitAdjust = async () => {
  if (!props.canAdjust || !Number.isFinite(props.userId) || props.userId <= 0) return
  error.value = ''
  success.value = ''
  const amount = asInt(form.amount)
  if (amount <= 0) {
    error.value = t('admin.userDetail.points.errors.invalidAmount')
    return
  }
  const remark = form.remark.trim()
  if (!remark) {
    error.value = t('admin.userDetail.points.errors.remarkRequired')
    return
  }
  if (remark.length > 200) {
    error.value = t('admin.userDetail.points.errors.remarkTooLong')
    return
  }

  submitting.value = true
  try {
    const response = await adminAPI.adjustUserPoints(props.userId, {
      delta: form.operation === 'subtract' ? -amount : amount,
      remark,
    })
    account.value = response.data.data || account.value
    form.amount = ''
    form.remark = ''
    await fetchTransactions(1)
    success.value = t('admin.userDetail.points.adjustSuccess')
  } catch (err: any) {
    error.value = err?.message || t('admin.userDetail.points.errors.adjustFailed')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void load(1)
})

watch(
  () => props.userId,
  () => {
    void load(1)
  },
)
</script>

<template>
  <div class="space-y-4">
    <Card class="rounded-lg border-border bg-background shadow-none">
      <CardContent class="grid grid-cols-1 gap-4 p-4 md:grid-cols-3">
        <div>
          <div class="text-xs text-muted-foreground">{{ t('admin.userDetail.points.balanceLabel') }}</div>
          <div class="mt-1 text-xl font-bold font-mono text-foreground">{{ asInt(account?.balance) }}</div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">{{ t('admin.userDetail.points.earnedLabel') }}</div>
          <div class="mt-1 text-sm font-mono text-foreground">{{ asInt(account?.total_earned) }}</div>
        </div>
        <div>
          <div class="text-xs text-muted-foreground">{{ t('admin.userDetail.points.spentLabel') }}</div>
          <div class="mt-1 text-sm font-mono text-foreground">{{ asInt(account?.total_spent) }}</div>
        </div>
      </CardContent>
    </Card>

    <div v-if="canAdjust" class="rounded-xl border border-border bg-card p-4">
      <div class="mb-4 text-sm font-semibold text-foreground">{{ t('admin.userDetail.points.adjustTitle') }}</div>
      <form class="grid grid-cols-1 gap-3 md:grid-cols-[180px_1fr_2fr_auto]" @submit.prevent="submitAdjust">
        <div>
          <label class="mb-1.5 block text-xs text-muted-foreground">{{ t('admin.userDetail.points.operationLabel') }}</label>
          <Select v-model="form.operation">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.userDetail.points.operations.add')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="add">{{ t('admin.userDetail.points.operations.add') }}</SelectItem>
              <SelectItem value="subtract">{{ t('admin.userDetail.points.operations.subtract') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <label class="mb-1.5 block text-xs text-muted-foreground">{{ t('admin.userDetail.points.amountLabel') }}</label>
          <Input v-model="form.amount" :placeholder="t('admin.userDetail.points.amountPlaceholder')" />
        </div>
        <div>
          <label class="mb-1.5 block text-xs text-muted-foreground">
            {{ t('admin.userDetail.points.remarkLabel') }} <span class="text-destructive" aria-hidden="true">*</span>
          </label>
          <Input v-model="form.remark" required maxlength="200" :placeholder="t('admin.userDetail.points.remarkPlaceholder')" />
        </div>
        <div class="flex items-end">
          <Button type="submit" class="w-full md:w-auto" :disabled="submitting">
            {{ submitting ? t('admin.userDetail.points.adjusting') : t('admin.userDetail.points.adjustSubmit') }}
          </Button>
        </div>
      </form>
    </div>

    <div v-if="error" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
      {{ error }}
    </div>
    <div v-if="success" class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">
      {{ success }}
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[920px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.userDetail.points.table.id') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.userDetail.points.table.type') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.userDetail.points.table.direction') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.userDetail.points.table.amount') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.userDetail.points.table.balanceAfter') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[220px]">{{ t('admin.userDetail.points.table.remark') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.userDetail.points.table.createdAt') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell colspan="7" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.common.loading') }}</TableCell>
          </TableRow>
          <TableRow v-else-if="transactions.length === 0">
            <TableCell colspan="7" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.userDetail.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="item in transactions" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="item.id" />
            </TableCell>
            <TableCell class="px-6 py-4 text-xs text-foreground">{{ typeLabel(item.type) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="directionClass(item.direction)">
                {{ directionLabel(item.direction) }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs font-mono text-foreground">{{ asInt(item.amount) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs font-mono text-foreground">{{ asInt(item.balance_after) }}</TableCell>
            <TableCell class="min-w-[220px] px-6 py-4 text-xs text-muted-foreground break-words">{{ item.remark || '-' }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <ListPagination
        :page="pagination.page"
        :total-page="pagination.total_page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        :page-size-options="pageSizeOptions"
        @change-page="changePage"
        @change-page-size="changePageSize"
      />
    </div>
  </div>
</template>
