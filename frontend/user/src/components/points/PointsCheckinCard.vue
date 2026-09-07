<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { CalendarCheck, Flame } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { PointsCheckinStatusData } from '../../api/types'

defineProps<{
  status: PointsCheckinStatusData | null
  loading: boolean
  canCheckin: boolean
}>()

defineEmits<{ (e: 'checkin'): void }>()

const { t } = useI18n()
</script>

<template>
  <!-- 站点未开启签到时整块不渲染，而不是渲染一个禁用按钮 -->
  <div v-if="status?.enabled" class="rounded-2xl border bg-card p-7 shadow-sm">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex items-center gap-3.5">
        <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary">
          <CalendarCheck class="h-5 w-5" />
        </span>
        <div>
          <h3 class="text-base font-black text-foreground">{{ t('personalCenter.points.checkinTitle') }}</h3>
          <p class="mt-1 text-sm text-muted-foreground">
            {{ t('personalCenter.points.checkinHint', { points: status.checkin_points }) }}
          </p>
          <p v-if="status.streak_days > 0" class="mt-1 flex items-center gap-1 text-xs text-warning">
            <Flame class="h-3.5 w-3.5" aria-hidden="true" />
            {{ t('personalCenter.points.streakDays', { days: status.streak_days }) }}
          </p>
        </div>
      </div>

      <Button
        type="button"
        :disabled="!canCheckin"
        :aria-label="t('personalCenter.points.checkin')"
        @click="$emit('checkin')"
      >
        {{ status.checked_in ? t('personalCenter.points.checkedIn') : t('personalCenter.points.checkin') }}
      </Button>
    </div>
  </div>
</template>
