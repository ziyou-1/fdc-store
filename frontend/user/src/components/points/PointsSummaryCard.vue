<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Coins, TrendingDown, TrendingUp } from 'lucide-vue-next'
import PanelHeading from '../shared/PanelHeading.vue'
import StatCard from '../shared/StatCard.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { pageAlertToneClass, pageAlertVariant, type PageAlert } from '../../utils/alerts'
import type { PointsAccountData } from '../../api/types'

defineProps<{
  account: PointsAccountData
  alert: PageAlert | null
}>()

const { t } = useI18n()

// 积分是整数，用千分位展示；不要走 money 工具（那套是两位小数字符串）
const formatPoints = (value: number) => new Intl.NumberFormat().format(value ?? 0)
</script>

<template>
  <div class="rounded-2xl border bg-card p-7 shadow-sm">
    <PanelHeading
      :title="t('personalCenter.points.title')"
      :description="t('personalCenter.points.subtitle')"
      :icon="Coins"
    />

    <Alert
      v-if="alert"
      :variant="pageAlertVariant(alert.level)"
      :class="pageAlertToneClass(alert.level)"
      class="mb-5"
    >
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <div class="grid gap-4 sm:grid-cols-3">
      <StatCard
        :label="t('personalCenter.points.balance')"
        :value="formatPoints(account.balance)"
        :icon="Coins"
        tone="accent"
        mono
      />
      <StatCard
        :label="t('personalCenter.points.totalEarned')"
        :value="formatPoints(account.total_earned)"
        :icon="TrendingUp"
        tone="success"
        mono
      />
      <StatCard
        :label="t('personalCenter.points.totalSpent')"
        :value="formatPoints(account.total_spent)"
        :icon="TrendingDown"
        tone="neutral"
        mono
      />
    </div>
  </div>
</template>
