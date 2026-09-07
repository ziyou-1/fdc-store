<template>
  <div class="space-y-6">
    <div class="rounded-2xl border bg-card p-6 shadow-sm">
      <h2 class="text-xl font-bold">{{ t('personalCenter.membership.title') }}</h2>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('personalCenter.membership.subtitle') }}</p>
      <div class="mt-4 text-sm">
        <template v-if="membershipActive">
          <p class="font-semibold">{{ t('personalCenter.membership.current') }}</p>
          <p class="mt-1 text-muted-foreground">
            {{ t(`personalCenter.membership.period.${profile?.membership_period || 'month'}`) }}
            · {{ discountLabel(profile?.membership_discount_rate) }}
          </p>
          <p class="mt-1 text-muted-foreground">{{ t('personalCenter.membership.expiresAt', { date: formatDate(profile?.membership_expires_at ?? undefined) }) }}</p>
        </template>
        <p v-else class="text-muted-foreground">{{ t('personalCenter.membership.inactive') }}</p>
      </div>
    </div>

    <Alert v-if="alert" :variant="alert.level === 'error' ? 'destructive' : 'default'">
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <div v-if="loading" class="grid gap-3 md:grid-cols-3">
      <div v-for="idx in 3" :key="idx" class="h-40 rounded-2xl bg-secondary"></div>
    </div>
    <div v-else role="radiogroup" :aria-label="t('personalCenter.membership.title')" class="grid gap-3 md:grid-cols-3">
      <button
        v-for="plan in plans"
        :key="plan.period"
        type="button"
        role="radio"
        :aria-checked="selectedPeriod === plan.period"
        class="rounded-2xl border p-5 text-left transition focus-visible:ring-2 focus-visible:ring-ring"
        :class="selectedPeriod === plan.period ? 'border-primary bg-primary/5' : 'bg-card'"
        @click="selectedPeriod = plan.period"
      >
        <p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          {{ t(`personalCenter.membership.period.${plan.period}`) }}
        </p>
        <p class="mt-2 text-lg font-bold">{{ localized(plan.name) }}</p>
        <p class="mt-1 text-2xl font-black tabular-nums">{{ formatMoney(String(plan.price)) }}</p>
        <p class="mt-2 text-sm font-semibold text-primary">
          {{ discountLabel(plan.discount_rate) }}
        </p>
        <p class="mt-2 text-xs text-muted-foreground">{{ localized(plan.description) }}</p>
      </button>
    </div>

    <div class="rounded-2xl border bg-card p-6 shadow-sm space-y-4">
      <div>
        <p class="text-sm font-medium">{{ t('personalCenter.membership.channel') }}</p>
        <p v-if="channelLoading" class="mt-2 text-sm text-muted-foreground">{{ t('common.loading') }}</p>
        <p v-else-if="channels.length === 0" class="mt-2 text-sm text-muted-foreground">{{ t('personalCenter.membership.channelEmpty') }}</p>
        <div v-else role="radiogroup" :aria-label="t('personalCenter.membership.channel')" class="mt-3 grid gap-2 sm:grid-cols-2">
          <button
            v-for="channel in channels"
            :key="channel.id"
            type="button"
            role="radio"
            :aria-checked="channelId === channel.id"
            class="rounded-xl border px-3 py-2 text-left text-sm focus-visible:ring-2 focus-visible:ring-ring"
            :class="channelId === channel.id ? 'border-primary bg-primary/5' : ''"
            @click="channelId = channel.id"
          >
            {{ channel.name || channel.channel_type }}
          </button>
        </div>
      </div>
      <Button :disabled="purchasing || !selectedPeriod || channelId <= 0" @click="purchase">
        {{ t('personalCenter.membership.buy') }}
      </Button>
    </div>

    <div v-if="qrResult?.qr_code" class="rounded-2xl border bg-card p-7 text-center shadow-sm">
      <h3 class="mb-4 text-base font-black">{{ t('personalCenter.membership.scanToPay') }}</h3>
      <img v-if="qrDataUrl" :src="qrDataUrl" class="mx-auto h-48 w-48" width="192" height="192" :alt="t('personalCenter.membership.scanToPay')" />
      <p class="mt-3 break-all text-xs text-muted-foreground">{{ qrResult.qr_code }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useMembership, isMembershipActive } from '../../composables/useMembership'
import { useUserProfileStore } from '../../stores/userProfile'
import { usePersonalCenter } from '../../composables/usePersonalCenter'

const { t, locale } = useI18n()
const discountLabel = (rate: number | string | undefined) => {
  const value = Number(rate)
  if (!Number.isFinite(value) || value <= 0) return ''
  if (String(locale.value).startsWith('en')) {
    return t('personalCenter.membership.discount', { n: value })
  }
  return t('personalCenter.membership.discount', { n: value / 10 })
}
const userProfileStore = useUserProfileStore()
const { formatMoney, formatDate } = usePersonalCenter(() => 'membership')
const {
  loading, purchasing, channelLoading, plans, channels, selectedPeriod, channelId,
  alert, qrResult, localized, loadPlans, loadChannels, purchase,
} = useMembership()

const profile = computed(() => userProfileStore.profile)
const membershipActive = computed(() => isMembershipActive(profile.value))
const qrDataUrl = ref('')

watch(selectedPeriod, () => {
  void loadChannels()
})

watch(() => qrResult.value?.qr_code, async (value) => {
  qrDataUrl.value = value ? await QRCode.toDataURL(value) : ''
})

onMounted(async () => {
  await Promise.allSettled([userProfileStore.loadProfile(), loadPlans()])
  await loadChannels()
})
</script>
