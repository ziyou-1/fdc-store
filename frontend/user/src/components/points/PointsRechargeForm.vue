<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Wallet } from 'lucide-vue-next'
import PanelHeading from '../shared/PanelHeading.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const props = defineProps<{
  amount: string
  channelId: number
  remark: string
  channels: any[]
  recharging: boolean
  channelLoading: boolean
  pointsPerUnit: number
}>()

const emit = defineEmits<{
  (e: 'update:amount', value: string): void
  (e: 'update:channelId', value: number): void
  (e: 'update:remark', value: string): void
  (e: 'amountBlur'): void
  (e: 'submit'): void
}>()

const { t } = useI18n()

// 预估到账积分：仅作展示，真实数量以后端按下单时的规则为准
const estimatedPoints = computed(() => {
  const value = Number(props.amount)
  if (!Number.isFinite(value) || value <= 0 || props.pointsPerUnit <= 0) return 0
  return Math.floor(value * props.pointsPerUnit)
})

// 与面板其余数字保持一致的千分位展示
const estimatedPointsDisplay = computed(() => new Intl.NumberFormat().format(estimatedPoints.value))

const canSubmit = computed(
  () => props.amount.trim() !== '' && props.channelId > 0 && !props.recharging,
)
</script>

<template>
  <div class="rounded-2xl border bg-card p-7 shadow-sm">
    <PanelHeading
      :title="t('personalCenter.points.rechargeTitle')"
      :description="t('personalCenter.points.rechargeSubtitle')"
      :icon="Wallet"
    />

    <div class="space-y-5">
      <div class="space-y-2">
        <Label for="points-recharge-amount">{{ t('personalCenter.points.amount') }}</Label>
        <Input
          id="points-recharge-amount"
          :model-value="amount"
          type="number"
          min="0"
          step="0.01"
          inputmode="decimal"
          :placeholder="t('personalCenter.points.amountPlaceholder')"
          @update:model-value="emit('update:amount', String($event))"
          @blur="emit('amountBlur')"
        />
        <p v-if="estimatedPoints > 0" class="text-xs text-muted-foreground">
          {{ t('personalCenter.points.estimated', { points: estimatedPointsDisplay }) }}
        </p>
      </div>

      <div class="space-y-2">
        <Label>{{ t('personalCenter.points.channel') }}</Label>
        <p v-if="channelLoading" class="text-sm text-muted-foreground">
          {{ t('common.loading') }}
        </p>
        <p v-else-if="channels.length === 0" class="text-sm text-muted-foreground">
          {{ t('personalCenter.points.channelEmpty') }}
        </p>
        <div v-else class="grid gap-2 sm:grid-cols-2">
          <button
            v-for="channel in channels"
            :key="channel.id"
            type="button"
            class="flex items-center justify-between rounded-xl border px-4 py-3 text-left text-sm transition-colors"
            :class="channelId === channel.id
              ? 'border-primary bg-primary/5 text-foreground'
              : 'border-border hover:bg-accent'"
            @click="emit('update:channelId', channel.id)"
          >
            <span class="truncate">{{ channel.name || channel.channel_type }}</span>
          </button>
        </div>
      </div>

      <div class="space-y-2">
        <Label for="points-recharge-remark">{{ t('personalCenter.points.remark') }}</Label>
        <Input
          id="points-recharge-remark"
          :model-value="remark"
          maxlength="200"
          :placeholder="t('personalCenter.points.remarkPlaceholder')"
          @update:model-value="emit('update:remark', String($event))"
        />
      </div>

      <Button type="button" :disabled="!canSubmit" @click="emit('submit')">
        {{ recharging ? t('common.loading') : t('personalCenter.points.rechargeSubmit') }}
      </Button>
    </div>
  </div>
</template>
