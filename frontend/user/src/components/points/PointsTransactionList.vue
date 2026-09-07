<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ReceiptText } from 'lucide-vue-next'
import PanelHeading from '../shared/PanelHeading.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { PointsTransactionData } from '../../api/types'

defineProps<{
  loading: boolean
  transactions: PointsTransactionData[]
  currentPage: number
  totalPages: number
}>()

defineEmits<{
  (e: 'refresh'): void
  (e: 'changePage', page: number): void
}>()

const { t, te } = useI18n()

const formatDate = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

const formatPoints = (value: number) => new Intl.NumberFormat().format(value ?? 0)

// type 是后端返回的字面值（checkin / referral_register / ...），
// 没有对应文案时回落显示原值，而不是显示原始 i18n key
const typeLabel = (type: string) => {
  const key = `personalCenter.points.types.${type}`
  return te(key) ? t(key) : type
}

const signedAmount = (direction: string, amount: number) =>
  `${direction === 'in' ? '+' : '-'}${formatPoints(Math.abs(amount))}`
</script>

<template>
  <div class="rounded-2xl border bg-card p-7 shadow-sm">
    <PanelHeading :title="t('personalCenter.points.detailTitle')" :icon="ReceiptText">
      <template #actions>
        <Button type="button" variant="outline" size="sm" @click="$emit('refresh')">
          {{ t('orders.filters.refresh') }}
        </Button>
      </template>
    </PanelHeading>

    <div v-if="loading" class="space-y-3">
      <div v-for="idx in 3" :key="idx" class="h-16 animate-pulse rounded-xl border bg-muted"></div>
    </div>
    <div
      v-else-if="transactions.length === 0"
      class="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground"
    >
      {{ t('personalCenter.points.empty') }}
    </div>
    <div v-else class="overflow-x-auto rounded-xl border">
      <Table>
        <TableHeader>
          <TableRow class="bg-muted/50">
            <TableHead class="px-4">{{ t('personalCenter.points.table.createdAt') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.points.table.type') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.points.table.direction') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.points.table.amount') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.points.table.balanceAfter') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.points.table.remark') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="item in transactions" :key="item.id">
            <TableCell class="px-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell class="px-4 text-xs text-muted-foreground">{{ typeLabel(item.type) }}</TableCell>
            <TableCell class="px-4">
              <Badge size="sm" :variant="item.direction === 'in' ? 'success' : 'secondary'">
                {{ t(`personalCenter.points.directions.${item.direction}`) }}
              </Badge>
            </TableCell>
            <TableCell
              class="px-4 font-mono text-sm"
              :class="item.direction === 'in' ? 'text-success' : 'text-destructive'"
            >
              {{ signedAmount(item.direction, item.amount) }}
            </TableCell>
            <TableCell class="px-4 font-mono text-sm text-foreground">
              {{ formatPoints(item.balance_after) }}
            </TableCell>
            <TableCell class="px-4 text-xs text-muted-foreground">{{ item.remark || '-' }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <div v-if="totalPages > 1" class="mt-6 flex flex-wrap items-center justify-center gap-3">
      <Button variant="outline" size="sm" :disabled="currentPage <= 1" @click="$emit('changePage', currentPage - 1)">
        {{ t('orders.prevPage') }}
      </Button>
      <span class="rounded-full border px-4 py-2 text-sm text-muted-foreground">
        {{ t('orders.pageInfo', { page: currentPage, total: totalPages }) }}
      </span>
      <Button
        variant="outline"
        size="sm"
        :disabled="currentPage >= totalPages"
        @click="$emit('changePage', currentPage + 1)"
      >
        {{ t('orders.nextPage') }}
      </Button>
    </div>
  </div>
</template>
